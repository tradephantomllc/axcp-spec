"""AXCP envelope serialization, signing, and verification."""

from __future__ import annotations

import base64
import uuid

from google.protobuf.message import DecodeError

from axcp.errors import InvalidEnvelopeError, SignatureMissingError
from axcp.identity import (
    DID,
    ED25519_SIGNATURE_SIZE,
    DIDResolver,
    Identity,
    is_small_order_public_key,
    public_key_from_did_key,
    resolve_ed25519_public_key,
    verify_signature,
)
from axcp.pb import axcp_pb2
from axcp.timestamp import now_ms, rfc3339_seconds_from_ms
from axcp.wire import check_envelope_wire

DID_AUTH_TRANSCRIPT_VERSION = "AXCP-DID-AUTH-v1"
DEFAULT_PROTOCOL_VERSION = 1
DEFAULT_PROFILE_SECURE_BASELINE = 1
MAX_UINT64 = (1 << 64) - 1


def new_envelope(
    trace_id: str | None = None,
    profile: int = DEFAULT_PROFILE_SECURE_BASELINE,
) -> axcp_pb2.AxcpEnvelope:
    env = axcp_pb2.AxcpEnvelope()
    env.version = DEFAULT_PROTOCOL_VERSION
    env.trace_id = trace_id or str(uuid.uuid4())
    env.profile = profile
    return env


def encode_envelope(envelope: axcp_pb2.AxcpEnvelope) -> bytes:
    return envelope.SerializeToString(deterministic=True)


def decode_envelope(data: bytes, *, strict: bool = False) -> axcp_pb2.AxcpEnvelope:
    """Decode envelope bytes.

    Use ``strict=True`` for bytes received from the network before verification: it
    rejects unknown fields at any level, undefined enum values, invalid UTF-8, two
    distinct payload members and envelopes without a payload (D5). Lenient decoding keeps
    only the last of two different payload members, so that condition can only be
    detected here.
    """

    if not isinstance(data, (bytes, bytearray, memoryview)):
        raise InvalidEnvelopeError("envelope bytes must be bytes-like")
    if strict:
        check_envelope_wire(data)
    env = axcp_pb2.AxcpEnvelope()
    try:
        env.ParseFromString(bytes(data))
    except (DecodeError, ValueError, TypeError, RuntimeError) as exc:
        raise InvalidEnvelopeError("failed to decode AXCP envelope") from exc
    return env


def signing_payload(envelope: axcp_pb2.AxcpEnvelope) -> bytes:
    """Return deterministic protobuf payload covered by AXCP DID auth.

    Sender DID, recipient DID, and timestamp are transcript bindings. Sequence
    remains inside the signed payload because replay protection consumes it and
    sequence tampering must invalidate the detached signature before replay
    state is touched.
    """

    clone = axcp_pb2.AxcpEnvelope()
    clone.CopyFrom(envelope)
    clone.sender_did = ""
    clone.recipient_did = ""
    clone.timestamp_ms = 0
    clone.ClearField("signature")
    clone.ClearField("attestation_proof")
    return clone.SerializeToString(deterministic=True)


def build_auth_transcript(envelope: axcp_pb2.AxcpEnvelope) -> bytes:
    payload = signing_payload(envelope)
    payload_b64 = base64.b64encode(payload).decode("ascii")
    timestamp = rfc3339_seconds_from_ms(envelope.timestamp_ms)
    return "\n".join(
        [
            DID_AUTH_TRANSCRIPT_VERSION,
            envelope.sender_did,
            envelope.recipient_did,
            payload_b64,
            timestamp,
        ]
    ).encode("utf-8")


def sign_envelope(
    envelope: axcp_pb2.AxcpEnvelope,
    identity: Identity,
    *,
    recipient_did: str,
    sequence: int | None = None,
    timestamp_ms: int | None = None,
) -> axcp_pb2.AxcpEnvelope:
    """Sign ``envelope`` for ``recipient_did`` (required, D4).

    When ``sequence`` is omitted the envelope must already carry a non-zero sequence:
    low-level signing never signs sequence 0 (D7).
    """

    if not isinstance(recipient_did, str) or not recipient_did:
        raise InvalidEnvelopeError("recipient_did is required to sign an envelope")
    DID.parse(recipient_did)
    if timestamp_ms is not None and (
        isinstance(timestamp_ms, bool) or not isinstance(timestamp_ms, int) or timestamp_ms <= 0
    ):
        raise InvalidEnvelopeError("timestamp_ms must be a positive integer")
    effective_sequence = envelope.sequence if sequence is None else sequence
    if (
        isinstance(effective_sequence, bool)
        or not isinstance(effective_sequence, int)
        or not 0 < effective_sequence <= MAX_UINT64
    ):
        raise InvalidEnvelopeError("sequence must be an integer between 1 and 2^64-1")

    envelope.sender_did = identity.did
    envelope.recipient_did = recipient_did
    envelope.timestamp_ms = timestamp_ms if timestamp_ms is not None else now_ms()
    envelope.sequence = effective_sequence
    transcript = build_auth_transcript(envelope)
    envelope.signature = identity.sign(transcript)
    return envelope


def validate_envelope_for_verification(
    envelope: axcp_pb2.AxcpEnvelope, expected_recipient_did: str
) -> None:
    """Structural Secure Baseline checks that need no key material (D1, D4, D5)."""

    if not isinstance(envelope, axcp_pb2.AxcpEnvelope):
        raise InvalidEnvelopeError("envelope must be an AxcpEnvelope")
    if not isinstance(expected_recipient_did, str) or not expected_recipient_did:
        raise InvalidEnvelopeError("expected_recipient_did is required for verification")
    if not envelope.signature:
        raise SignatureMissingError("envelope signature is missing")
    if len(envelope.signature) != ED25519_SIGNATURE_SIZE:
        raise InvalidEnvelopeError("envelope signature must be 64 bytes")
    if envelope.version != DEFAULT_PROTOCOL_VERSION:
        raise InvalidEnvelopeError("unsupported envelope version")
    if envelope.profile != DEFAULT_PROFILE_SECURE_BASELINE:
        raise InvalidEnvelopeError("envelope profile must be secure-baseline-v1")
    if not envelope.sender_did:
        raise InvalidEnvelopeError("sender_did is required")
    DID.parse(envelope.sender_did)
    if not envelope.recipient_did:
        raise InvalidEnvelopeError("recipient_did is required")
    if envelope.recipient_did != expected_recipient_did:
        raise InvalidEnvelopeError("envelope recipient_did does not match the verifier")
    if envelope.sequence <= 0:
        raise InvalidEnvelopeError("sequence must be greater than zero")
    # Unknown fields survive parsing in Python objects, so re-serializing exposes them,
    # together with undefined enum values and a missing payload, to the strict wire rules.
    check_envelope_wire(envelope.SerializeToString())


def verify_envelope(
    envelope: axcp_pb2.AxcpEnvelope,
    *,
    expected_recipient_did: str,
    public_key: bytes | None = None,
    resolver: DIDResolver | None = None,
) -> None:
    """Verify the signature and Secure Baseline structure of ``envelope``.

    ``expected_recipient_did`` is the verifier's own DID (D4). Timestamps and replay are
    not checked here; :meth:`axcp.Agent.verify` does both.
    """

    validate_envelope_for_verification(envelope, expected_recipient_did)
    sender_did = envelope.sender_did
    if public_key is not None:
        resolved_public_key = bytes(public_key)
        if is_small_order_public_key(resolved_public_key):
            raise InvalidEnvelopeError("explicit public key has small order")
        if DID.parse(sender_did).method == "key" and public_key_from_did_key(sender_did) != resolved_public_key:
            raise InvalidEnvelopeError("explicit public key does not match sender did:key")
    else:
        resolved_public_key = resolve_ed25519_public_key(sender_did, resolver)
    transcript = build_auth_transcript(envelope)
    verify_signature(resolved_public_key, transcript, bytes(envelope.signature))
