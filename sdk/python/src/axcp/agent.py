"""High-level AXCP agent facade for signed envelopes."""

from __future__ import annotations

import threading
from dataclasses import dataclass, field

from axcp.envelope import new_envelope, sign_envelope, verify_envelope
from axcp.errors import InvalidEnvelopeError
from axcp.identity import DIDResolver, Identity
from axcp.pb import axcp_pb2
from axcp.replay import ReplayProtector
from axcp.timestamp import TimestampValidator


@dataclass
class Agent:
    """AXCP Core agent facade.

    The Phase 1 SDK intentionally covers identity, protobuf envelopes,
    signatures, timestamp validation, and replay protection only. QUIC
    transport is introduced in the next phase.
    """

    identity: Identity
    resolver: DIDResolver | None = None
    replay: ReplayProtector = field(default_factory=ReplayProtector)
    timestamps: TimestampValidator = field(default_factory=TimestampValidator)
    _sequence: int = 0
    _lock: threading.Lock = field(default_factory=threading.Lock, init=False, repr=False)

    def new_envelope(self, trace_id: str | None = None, profile: int = 1) -> axcp_pb2.AxcpEnvelope:
        return new_envelope(trace_id=trace_id, profile=profile)

    def sign_message(
        self,
        envelope: axcp_pb2.AxcpEnvelope,
        *,
        recipient_did: str = "",
        timestamp_ms: int | None = None,
    ) -> axcp_pb2.AxcpEnvelope:
        with self._lock:
            self._sequence += 1
            sequence = self._sequence
        return sign_envelope(
            envelope,
            self.identity,
            recipient_did=recipient_did,
            sequence=sequence,
            timestamp_ms=timestamp_ms,
        )

    def verify(self, envelope: axcp_pb2.AxcpEnvelope) -> None:
        if envelope.recipient_did and envelope.recipient_did != self.identity.did:
            raise InvalidEnvelopeError("envelope recipient_did does not match this agent")
        # Verify signature before marking replay state so unauthenticated traffic
        # cannot burn valid sequence numbers for a peer.
        verify_envelope(envelope, resolver=self.resolver)
        self.timestamps.validate(envelope.timestamp_ms)
        self.replay.check_and_mark(envelope.sender_did, envelope.sequence)
