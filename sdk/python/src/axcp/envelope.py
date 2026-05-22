"""AXCP envelope serialization, signing, and verification."""

from __future__ import annotations

import base64
import uuid

from google.protobuf.message import DecodeError

from axcp.errors import InvalidEnvelopeError, SignatureMissingError
from axcp.identity import (
    DID,
    DIDResolver,
    Identity,
    public_key_from_did_key,
    resolve_ed25519_public_key,
    verify_signature,
)
from axcp.pb import axcp_pb2
from axcp.timestamp import now_ms, rfc3339_seconds_from_ms

DID_AUTH_TRANSCRIPT_VERSION = "AXCP-DID-AUTH-v1"
DEFAULT_PROTOCOL_VERSION = 1
DEFAULT_PROFILE_SECURE_BASELINE = 1


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


def decode_envelope(data: bytes) -> axcp_pb2.AxcpEnvelope:
    env = axcp_pb2.AxcpEnvelope()
    try:
        env.ParseFromString(data)
    except DecodeError as exc:
        raise InvalidEnvelopeError("failed to decode AXCP envelope") from exc
    return env


def signing_payload(envelope: axcp_pb2.AxcpEnvelope) -> bytes:
    """Return deterministic protobuf payload covered by AXCP DID auth."""

    clone = axcp_pb2.AxcpEnvelope()
    clone.CopyFrom(envelope)
    clone.sender_did = ""
    clone.recipient_did = ""
    clone.timestamp_ms = 0
    clone.sequence = 0
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
    recipient_did: str = "",
    sequence: int | None = None,
    timestamp_ms: int | None = None,
) -> axcp_pb2.AxcpEnvelope:
    if timestamp_ms is not None and timestamp_ms < 0:
        raise InvalidEnvelopeError("timestamp_ms cannot be negative")
    if sequence is not None and sequence < 0:
        raise InvalidEnvelopeError("sequence cannot be negative")

    envelope.sender_did = identity.did
    envelope.recipient_did = recipient_did
    envelope.timestamp_ms = timestamp_ms if timestamp_ms is not None else now_ms()
    if sequence is not None:
        envelope.sequence = sequence
    transcript = build_auth_transcript(envelope)
    envelope.signature = identity.sign(transcript)
    return envelope


def verify_envelope(
    envelope: axcp_pb2.AxcpEnvelope,
    *,
    public_key: bytes | None = None,
    resolver: DIDResolver | None = None,
) -> None:
    if not envelope.signature:
        raise SignatureMissingError("envelope signature is missing")
    if not envelope.sender_did:
        raise InvalidEnvelopeError("sender_did is required")

    resolved_public_key = public_key or resolve_ed25519_public_key(envelope.sender_did, resolver)
    parsed_sender = DID.parse(envelope.sender_did)
    if (
        public_key is not None
        and parsed_sender.method == "key"
        and public_key_from_did_key(envelope.sender_did) != public_key
    ):
        raise InvalidEnvelopeError("explicit public key does not match sender did:key")
    transcript = build_auth_transcript(envelope)
    verify_signature(resolved_public_key, transcript, bytes(envelope.signature))
