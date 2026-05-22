import base64
from datetime import datetime, timedelta, timezone

import pytest

from axcp import Agent, Identity, InMemoryDIDResolver
from axcp.envelope import (
    DID_AUTH_TRANSCRIPT_VERSION,
    build_auth_transcript,
    decode_envelope,
    encode_envelope,
    new_envelope,
    signing_payload,
    verify_envelope,
)
from axcp.errors import (
    InvalidEnvelopeError,
    ReplayDetectedError,
    SignatureVerificationError,
    TimestampExpiredError,
)
from axcp.pb import axcp_pb2


def _context_envelope():
    env = new_envelope(trace_id="trace-123", profile=1)
    env.context_patch.CopyFrom(
        axcp_pb2.ContextPatch(
            context_id="ctx",
            base_version=7,
            ops=[axcp_pb2.DeltaOp(op=axcp_pb2.DeltaOp.ADD, path="/x", data=b"1", ts=1)],
        )
    )
    return env


def test_signing_payload_excludes_auth_fields() -> None:
    env = _context_envelope()
    env.sender_did = "did:key:sender"
    env.recipient_did = "did:key:recipient"
    env.timestamp_ms = 1_700_000_000_000
    env.sequence = 42
    env.signature = b"signature"
    env.attestation_proof = b"attestation"

    payload = signing_payload(env)
    decoded = axcp_pb2.AxcpEnvelope()
    decoded.ParseFromString(payload)

    assert decoded.sender_did == ""
    assert decoded.recipient_did == ""
    assert decoded.timestamp_ms == 0
    assert decoded.sequence == 0
    assert decoded.signature == b""
    assert decoded.attestation_proof == b""
    assert decoded.context_patch.context_id == "ctx"


def test_signing_payload_stable_when_auth_fields_change() -> None:
    left = _context_envelope()
    left.sender_did = "did:key:left"
    left.recipient_did = "did:key:recipient"
    left.timestamp_ms = 1_700_000_000_000
    left.sequence = 1
    left.signature = b"left"

    right = _context_envelope()
    right.sender_did = "did:key:right"
    right.recipient_did = "did:key:other"
    right.timestamp_ms = 1_700_000_001_000
    right.sequence = 2
    right.signature = b"right"

    assert signing_payload(left) == signing_payload(right)


def test_auth_transcript_matches_go_format_shape() -> None:
    env = _context_envelope()
    env.sender_did = "did:key:sender"
    env.recipient_did = "did:key:recipient"
    env.timestamp_ms = 1_700_000_000_123

    transcript = build_auth_transcript(env).decode("utf-8")
    lines = transcript.split("\n")

    assert lines[0] == DID_AUTH_TRANSCRIPT_VERSION
    assert lines[1] == env.sender_did
    assert lines[2] == env.recipient_did
    assert base64.b64decode(lines[3]) == signing_payload(env)
    assert lines[4] == "2023-11-14T22:13:20Z"


def test_agent_sign_verify_encode_decode_roundtrip() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.identity.did)
    raw = encode_envelope(env)
    decoded = decode_envelope(raw)

    bob.verify(decoded)
    assert decoded.sender_did == alice.identity.did
    assert decoded.recipient_did == bob.identity.did


def test_replay_is_rejected_after_valid_signature() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.identity.did)
    bob.verify(env)
    with pytest.raises(ReplayDetectedError):
        bob.verify(env)


def test_payload_tamper_rejects_signature() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.identity.did)
    env.context_patch.context_id = "tampered"

    with pytest.raises(SignatureVerificationError):
        bob.verify(env)


def test_timestamp_expiry_is_enforced_after_signature_verification() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    old_ms = int((datetime.now(tz=timezone.utc) - timedelta(minutes=10)).timestamp() * 1000)
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.identity.did, timestamp_ms=old_ms)

    with pytest.raises(TimestampExpiredError):
        bob.verify(env)


def test_verify_envelope_allows_explicit_public_key() -> None:
    alice = Agent(Identity.generate())
    env = _context_envelope()

    alice.sign_message(env)
    verify_envelope(env, public_key=alice.identity.public_key)


def test_verify_envelope_rejects_explicit_key_mismatch_for_did_key() -> None:
    alice = Agent(Identity.generate())
    other = Identity.generate()
    env = _context_envelope()

    alice.sign_message(env)

    with pytest.raises(InvalidEnvelopeError):
        verify_envelope(env, public_key=other.public_key)


def test_agent_rejects_wrong_recipient() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    carol = Agent(Identity.generate())
    env = _context_envelope()

    alice.sign_message(env, recipient_did=carol.identity.did)

    with pytest.raises(InvalidEnvelopeError):
        bob.verify(env)


def test_agent_verifies_non_key_sender_with_resolver() -> None:
    seed_identity = Identity.generate()
    alice_identity = Identity.from_seed(seed_identity.private_seed, did="did:example:alice")
    resolver = InMemoryDIDResolver()
    resolver.register(alice_identity)

    alice = Agent(alice_identity)
    bob = Agent(Identity.generate(), resolver=resolver)
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.identity.did)
    bob.verify(env)
