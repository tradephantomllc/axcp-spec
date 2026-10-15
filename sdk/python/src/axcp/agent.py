"""High-level AXCP agent facade for signed envelopes."""

from __future__ import annotations

import threading
from dataclasses import dataclass, field

from axcp.envelope import MAX_UINT64, decode_envelope, new_envelope, sign_envelope, verify_envelope
from axcp.errors import InvalidEnvelopeError
from axcp.identity import DIDResolver, Identity
from axcp.pb import axcp_pb2
from axcp.replay import ReplayProtector
from axcp.timestamp import TimestampValidator, now_us


@dataclass
class Agent:
    """AXCP Core agent facade: identity, envelope signing and verification.

    * ``resolver`` is consulted for every sender DID, including did:key (allowlist, D3).
    * ``replay``'s peer TTL is raised to at least ``timestamps.max_age + clock_skew``.
    * ``initial_sequence`` defaults to the current Unix time in microseconds so
      sequences stay monotonic across restarts (D7).
    """

    identity: Identity
    resolver: DIDResolver | None = None
    replay: ReplayProtector = field(default_factory=ReplayProtector)
    timestamps: TimestampValidator = field(default_factory=TimestampValidator)
    initial_sequence: int | None = None
    _next_sequence: int = field(default=0, init=False, repr=False)
    _lock: threading.Lock = field(default_factory=threading.Lock, init=False, repr=False)

    def __post_init__(self) -> None:
        start = now_us() if self.initial_sequence is None else self.initial_sequence
        if isinstance(start, bool) or not isinstance(start, int) or not 0 < start <= MAX_UINT64:
            raise InvalidEnvelopeError("initial_sequence must be an integer between 1 and 2^64-1")
        self._next_sequence = start
        self.replay.apply_timestamp_policy(self.timestamps)

    def new_envelope(self, trace_id: str | None = None, profile: int = 1) -> axcp_pb2.AxcpEnvelope:
        return new_envelope(trace_id=trace_id, profile=profile)

    def sign_message(
        self,
        envelope: axcp_pb2.AxcpEnvelope,
        *,
        recipient_did: str,
        timestamp_ms: int | None = None,
    ) -> axcp_pb2.AxcpEnvelope:
        """Sign with the next sequence number. ``recipient_did`` is required (D4)."""

        with self._lock:
            sequence = self._next_sequence
            if sequence > MAX_UINT64:
                raise InvalidEnvelopeError("sequence space exhausted; create a new agent")
            signed = sign_envelope(
                envelope,
                self.identity,
                recipient_did=recipient_did,
                sequence=sequence,
                timestamp_ms=timestamp_ms,
            )
            self._next_sequence = sequence + 1
        return signed

    def verify(self, envelope: axcp_pb2.AxcpEnvelope) -> None:
        """Verify structure (D5), recipient == this agent (D4), signature, timestamp, replay.

        Replay state is only touched after the signature verified, so unauthenticated
        traffic cannot burn sequence numbers. Prefer :meth:`verify_bytes` for network
        input so duplicate payload members are detected on the wire.
        """

        verify_envelope(envelope, expected_recipient_did=self.identity.did, resolver=self.resolver)
        self.timestamps.validate(envelope.timestamp_ms)
        self.replay.check_and_mark(envelope.sender_did, envelope.sequence)

    def verify_bytes(self, data: bytes) -> axcp_pb2.AxcpEnvelope:
        """Strictly decode and verify wire bytes; returns the verified envelope."""

        envelope = decode_envelope(data, strict=True)
        self.verify(envelope)
        return envelope
