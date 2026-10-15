import base64
import json
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from axcp import Agent, Identity, InMemoryDIDResolver
from axcp.envelope import (
    DID_AUTH_TRANSCRIPT_VERSION,
    build_auth_transcript,
    decode_envelope,
    encode_envelope,
    new_envelope,
    sign_envelope,
    signing_payload,
    verify_envelope,
)
from axcp.errors import (
    AXCPError,
    InvalidDIDFormat,
    InvalidEnvelopeError,
    ReplayDetectedError,
    SignatureMissingError,
    SignatureVerificationError,
    TimestampExpiredError,
    TimestampInvalidError,
)
from axcp.pb import axcp_pb2
from axcp.timestamp import rfc3339_seconds_from_ms

TESTDATA = Path(__file__).resolve().parents[3] / "testdata" / "sdk"
VECTOR_PATH = TESTDATA / "secure_baseline_vector.json"
REPEATED_VECTOR_PATH = TESTDATA / "repeated_defaults_vector.json"


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


def _raw_sign(env, identity: Identity, *, recipient_did: str = "", sequence: int = 0, timestamp_ms=None):
    """Sign without any signing-side guard, so only verifier checks can reject."""

    env.sender_did = identity.did
    env.recipient_did = recipient_did
    env.timestamp_ms = timestamp_ms if timestamp_ms is not None else int(time.time() * 1000)
    env.sequence = sequence
    env.signature = identity.sign(build_auth_transcript(env))
    return env


def _vector_envelope(vector: dict) -> axcp_pb2.AxcpEnvelope:
    env = new_envelope(trace_id=vector["trace_id"], profile=vector["profile"])
    patch = vector["context_patch"]
    env.context_patch.context_id = patch["context_id"]
    env.context_patch.base_version = patch["base_version"]
    for op in patch["ops"]:
        env.context_patch.ops.append(
            axcp_pb2.DeltaOp(
                op=getattr(axcp_pb2.DeltaOp, op["op"]),
                path=op["path"],
                data=base64.b64decode(op["data_b64"]),
                ts=op["ts"],
            )
        )
    return env


def test_signing_payload_excludes_detached_auth_fields() -> None:
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
    assert decoded.sequence == 42
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
    right.sequence = 1
    right.signature = b"right"

    assert signing_payload(left) == signing_payload(right)


def test_signing_payload_changes_when_sequence_changes() -> None:
    left = _context_envelope()
    left.sequence = 1

    right = _context_envelope()
    right.sequence = 2

    assert signing_payload(left) != signing_payload(right)


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


def test_agent_verify_bytes_roundtrip() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = alice.sign_message(_context_envelope(), recipient_did=bob.identity.did)

    verified = bob.verify_bytes(encode_envelope(env))
    assert verified.context_patch.context_id == "ctx"


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


def test_sequence_tamper_rejects_signature() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.identity.did)
    env.sequence += 1

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
    bob = Identity.generate()
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.did)
    verify_envelope(env, expected_recipient_did=bob.did, public_key=alice.identity.public_key)


def test_verify_envelope_rejects_explicit_key_mismatch_for_did_key() -> None:
    alice = Agent(Identity.generate())
    bob = Identity.generate()
    other = Identity.generate()
    env = _context_envelope()

    alice.sign_message(env, recipient_did=bob.did)

    with pytest.raises(InvalidEnvelopeError):
        verify_envelope(env, expected_recipient_did=bob.did, public_key=other.public_key)


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


def test_shared_secure_baseline_vector_matches_python_sdk() -> None:
    vector = json.loads(VECTOR_PATH.read_text(encoding="utf-8"))
    identity = Identity.from_seed(bytes.fromhex(vector["identity"]["private_seed_hex"]))
    env = _vector_envelope(vector)

    sign_envelope(
        env,
        identity,
        recipient_did=vector["recipient_did"],
        sequence=vector["sequence"],
        timestamp_ms=vector["timestamp_ms"],
    )

    expected = vector["expected"]
    assert identity.did == vector["identity"]["did"]
    assert base64.b64encode(identity.public_key).decode("ascii") == vector["identity"]["public_key_b64"]
    assert base64.b64encode(signing_payload(env)).decode("ascii") == expected["signing_payload_b64"]
    assert base64.b64encode(build_auth_transcript(env)).decode("ascii") == expected["auth_transcript_b64"]
    assert base64.b64encode(env.signature).decode("ascii") == expected["signature_b64"]
    assert base64.b64encode(encode_envelope(env)).decode("ascii") == expected["envelope_b64"]


def test_shared_secure_baseline_go_envelope_bytes_verify_strictly() -> None:
    vector = json.loads(VECTOR_PATH.read_text(encoding="utf-8"))
    decoded = decode_envelope(base64.b64decode(vector["expected"]["envelope_b64"]), strict=True)
    verify_envelope(decoded, expected_recipient_did=vector["recipient_did"])
    assert decoded.sender_did == vector["identity"]["did"]
    assert decoded.sequence == vector["sequence"]
    with pytest.raises(InvalidEnvelopeError):
        verify_envelope(decoded, expected_recipient_did="did:example:carol")


def test_repeated_defaults_vector_matches_python_sdk() -> None:
    vector = json.loads(REPEATED_VECTOR_PATH.read_text(encoding="utf-8"))
    identity = Identity.from_seed(bytes.fromhex(vector["identity"]["private_seed_hex"]))
    assert identity.did == vector["identity"]["did"]
    env = new_envelope(trace_id="repeated-defaults", profile=1)
    env.capability_msg.request.ids.extend(vector["ids"])
    sign_envelope(
        env,
        identity,
        recipient_did=vector["recipient_did"],
        sequence=vector["sequence"],
        timestamp_ms=vector["timestamp_ms"],
    )

    expected = vector["expected"]
    assert list(decode_envelope(encode_envelope(env)).capability_msg.request.ids) == ["", "read", ""]
    assert base64.b64encode(encode_envelope(env)).decode("ascii") == expected["envelope_b64"]
    assert base64.b64encode(signing_payload(env)).decode("ascii") == expected["signing_payload_b64"]
    assert base64.b64encode(env.signature).decode("ascii") == expected["signature_b64"]

    go_bytes = base64.b64decode(expected["envelope_b64"])
    decoded = decode_envelope(go_bytes, strict=True)
    verify_envelope(decoded, expected_recipient_did=vector["recipient_did"])
    assert list(decoded.capability_msg.request.ids) == ["", "read", ""]


# --- D4: recipient binding -------------------------------------------------


def test_signing_requires_recipient() -> None:
    alice = Agent(Identity.generate())
    with pytest.raises(InvalidEnvelopeError):
        alice.sign_message(_context_envelope(), recipient_did="")
    with pytest.raises(TypeError):
        alice.sign_message(_context_envelope())  # type: ignore[call-arg]
    with pytest.raises(InvalidEnvelopeError):
        sign_envelope(_context_envelope(), alice.identity, recipient_did="", sequence=1)
    with pytest.raises(InvalidDIDFormat):
        sign_envelope(_context_envelope(), alice.identity, recipient_did="not-a-did", sequence=1)


def test_empty_recipient_is_rejected_even_with_valid_signature() -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = _raw_sign(_context_envelope(), alice, sequence=5)
    alice.verify(build_auth_transcript(env), env.signature)  # the signature itself is valid

    with pytest.raises(InvalidEnvelopeError, match="recipient_did is required"):
        bob.verify(env)


def test_envelope_for_another_recipient_is_rejected() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    carol = Agent(Identity.generate())
    env = alice.sign_message(_context_envelope(), recipient_did=bob.identity.did)

    bob.verify(env)
    with pytest.raises(InvalidEnvelopeError, match="does not match the verifier"):
        carol.verify(env)


def test_low_level_verify_requires_expected_recipient() -> None:
    alice = Agent(Identity.generate())
    bob = Identity.generate()
    env = alice.sign_message(_context_envelope(), recipient_did=bob.did)
    with pytest.raises(InvalidEnvelopeError):
        verify_envelope(env, expected_recipient_did="")


# --- D5: envelope rules ----------------------------------------------------


def test_version_other_than_one_is_rejected() -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = _context_envelope()
    env.version = 2
    sign_envelope(env, alice, recipient_did=bob.identity.did, sequence=1)
    with pytest.raises(InvalidEnvelopeError, match="unsupported envelope version"):
        bob.verify(env)


@pytest.mark.parametrize("profile", [0, 2, 3])
def test_non_secure_baseline_profiles_are_rejected(profile: int) -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = new_envelope(trace_id="profile", profile=profile)
    env.context_patch.context_id = "ctx"
    sign_envelope(env, alice, recipient_did=bob.identity.did, sequence=1)
    with pytest.raises(InvalidEnvelopeError, match="secure-baseline-v1"):
        bob.verify(env)


def test_envelope_without_payload_is_rejected() -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = new_envelope(trace_id="empty")
    sign_envelope(env, alice, recipient_did=bob.identity.did, sequence=1)
    with pytest.raises(InvalidEnvelopeError, match="exactly one payload"):
        bob.verify(env)
    with pytest.raises(InvalidEnvelopeError, match="exactly one payload"):
        bob.verify_bytes(encode_envelope(env))


def test_sequence_zero_is_rejected() -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = _raw_sign(_context_envelope(), alice, recipient_did=bob.identity.did, sequence=0)
    with pytest.raises(InvalidEnvelopeError, match="sequence must be greater than zero"):
        bob.verify(env)


def test_signature_must_be_64_bytes() -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = sign_envelope(_context_envelope(), alice, recipient_did=bob.identity.did, sequence=1)
    env.signature = env.signature[:63]
    with pytest.raises(InvalidEnvelopeError, match="64 bytes"):
        bob.verify(env)
    env.signature = b""
    with pytest.raises(SignatureMissingError):
        bob.verify(env)


def test_unknown_enum_value_is_rejected_although_the_signature_is_valid() -> None:
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = _context_envelope()
    env.context_patch.ops[0].op = 7  # proto3 open enum: Python keeps the undefined value
    sign_envelope(env, alice, recipient_did=bob.identity.did, sequence=1)
    alice.verify(build_auth_transcript(env), env.signature)
    with pytest.raises(InvalidEnvelopeError, match="undefined enum value"):
        bob.verify(env)


def test_unknown_field_on_decoded_object_is_rejected() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = alice.sign_message(_context_envelope(), recipient_did=bob.identity.did)
    tampered = decode_envelope(encode_envelope(env) + b"\x90\x03\x01")  # field 50 varint
    with pytest.raises(InvalidEnvelopeError, match="unknown field 50"):
        bob.verify(tampered)


def test_invalid_sender_did_is_rejected_before_resolution() -> None:
    bob = Agent(Identity.generate())
    env = _context_envelope()
    env.sender_did = "did:key:z6Mk/../path"
    env.recipient_did = bob.identity.did
    env.sequence = 1
    env.timestamp_ms = int(time.time() * 1000)
    env.signature = b"\x00" * 64
    with pytest.raises(InvalidDIDFormat):
        bob.verify(env)


def test_ten_megabyte_sender_did_is_rejected_quickly() -> None:
    bob = Agent(Identity.generate())
    env = _context_envelope()
    env.sender_did = "did:key:z" + "6" * (10 * 1024 * 1024)
    env.recipient_did = bob.identity.did
    env.sequence = 1
    env.timestamp_ms = int(time.time() * 1000)
    env.signature = b"\x00" * 64
    started = time.perf_counter()
    with pytest.raises(InvalidDIDFormat, match="exceeds 512 bytes"):
        bob.verify(env)
    assert time.perf_counter() - started < 0.05


# --- D8/D10: error hygiene -------------------------------------------------


@pytest.mark.parametrize("timestamp_ms", [253_402_300_800_000, (1 << 63), (1 << 64) - 1])
def test_out_of_range_timestamps_raise_sdk_errors(timestamp_ms: int) -> None:
    with pytest.raises(TimestampInvalidError):
        rfc3339_seconds_from_ms(timestamp_ms)
    alice = Identity.generate()
    bob = Agent(Identity.generate())
    env = _context_envelope()
    env.sender_did = alice.did
    env.recipient_did = bob.identity.did
    env.sequence = 1
    env.timestamp_ms = timestamp_ms
    env.signature = b"\x00" * 64
    with pytest.raises(TimestampInvalidError):
        bob.verify_bytes(encode_envelope(env))
    with pytest.raises(TimestampInvalidError):
        bob.timestamps.validate(timestamp_ms)


def test_garbage_bytes_raise_sdk_errors() -> None:
    bob = Agent(Identity.generate())
    for data in (b"\xff", b"\x0a\x05abc", b"\x22\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80"):
        with pytest.raises(AXCPError):
            bob.verify_bytes(data)
        with pytest.raises(InvalidEnvelopeError):
            decode_envelope(data, strict=True)
