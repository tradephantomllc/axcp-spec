"""Strict wire decoding (D5): each rejected case is accepted by lenient decoding."""

import pytest

from axcp import Agent, Identity
from axcp.envelope import decode_envelope, encode_envelope, new_envelope
from axcp.errors import InvalidEnvelopeError
from axcp.pb import axcp_pb2


def _varint(value: int) -> bytes:
    out = bytearray()
    while True:
        byte = value & 0x7F
        value >>= 7
        if value:
            out.append(byte | 0x80)
        else:
            out.append(byte)
            return bytes(out)


def _ld(field: int, body: bytes) -> bytes:
    return _varint((field << 3) | 2) + _varint(len(body)) + body


def _vi(field: int, value: int) -> bytes:
    return _varint(field << 3) + _varint(value)


HEADER = _vi(1, 1) + _vi(3, 1)


def _signed():
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())
    env = new_envelope(trace_id="strict")
    env.context_patch.CopyFrom(
        axcp_pb2.ContextPatch(
            context_id="ctx",
            base_version=7,
            ops=[axcp_pb2.DeltaOp(op=axcp_pb2.DeltaOp.REPLACE, path="/x", data=b"1", ts=1)],
        )
    )
    alice.sign_message(env, recipient_did=bob.identity.did)
    return alice, bob, env


def _without_context_patch(env) -> bytes:
    copy = axcp_pb2.AxcpEnvelope()
    copy.CopyFrom(env)
    copy.ClearField("context_patch")
    return encode_envelope(copy)


def test_canonical_envelope_is_accepted() -> None:
    _, bob, env = _signed()
    assert bob.verify_bytes(encode_envelope(env)).context_patch.context_id == "ctx"


def test_unknown_nested_field_is_rejected() -> None:
    _, bob, env = _signed()
    nested = env.context_patch.SerializeToString() + _vi(9, 7)
    tampered = _without_context_patch(env) + _ld(4, nested)
    with pytest.raises(InvalidEnvelopeError, match="unknown field 9 in ContextPatch"):
        decode_envelope(tampered, strict=True)
    with pytest.raises(InvalidEnvelopeError):
        bob.verify_bytes(tampered)
    # The lenient object still carries the unknown field, so object verification also rejects it.
    with pytest.raises(InvalidEnvelopeError, match="unknown field 9"):
        Agent(bob.identity).verify(decode_envelope(tampered))


def test_reserved_capability_descriptor_field_is_rejected() -> None:
    descriptor = _ld(1, b"tool") + _vi(8, 1)
    data = HEADER + _ld(5, _ld(1, _ld(1, descriptor)))
    with pytest.raises(InvalidEnvelopeError, match="unknown field 8 in CapabilityDescriptor"):
        decode_envelope(data, strict=True)


def test_two_distinct_payload_members_are_rejected() -> None:
    _, bob, env = _signed()
    # profile_ack appended BEFORE the signed context_patch: lenient parsing keeps only the
    # last member, so the object looks valid and its signature verifies.
    tampered = _without_context_patch(env) + _ld(9, b"") + _ld(4, env.context_patch.SerializeToString())
    lenient = decode_envelope(tampered)
    assert lenient.WhichOneof("payload") == "context_patch"
    Agent(bob.identity).verify(lenient)
    with pytest.raises(InvalidEnvelopeError, match="multiple members of oneof payload"):
        decode_envelope(tampered, strict=True)
    with pytest.raises(InvalidEnvelopeError):
        bob.verify_bytes(tampered)


def test_two_distinct_nested_oneof_members_are_rejected() -> None:
    data = HEADER + _ld(5, _ld(2, b"") + _ld(3, b""))
    with pytest.raises(InvalidEnvelopeError, match="multiple members of oneof kind"):
        decode_envelope(data, strict=True)


def test_repeated_same_payload_member_is_merged() -> None:
    _, bob, env = _signed()
    first = axcp_pb2.ContextPatch(context_id="ctx").SerializeToString()
    rest = axcp_pb2.ContextPatch()
    rest.CopyFrom(env.context_patch)
    rest.context_id = ""
    split = _without_context_patch(env) + _ld(4, first) + _ld(4, rest.SerializeToString())
    verified = bob.verify_bytes(split)
    assert verified.context_patch.context_id == "ctx"
    assert len(verified.context_patch.ops) == 1


def test_missing_payload_is_rejected() -> None:
    with pytest.raises(InvalidEnvelopeError, match="exactly one payload member"):
        decode_envelope(HEADER, strict=True)
    decode_envelope(HEADER)


@pytest.mark.parametrize(
    ("value", "accepted"),
    [(3, True), ((1 << 32) + 1, True), (7, False), ((1 << 64) - 1, False)],
)
def test_enum_values_are_checked_after_int32_truncation(value: int, accepted: bool) -> None:
    data = HEADER + _ld(4, _ld(3, _vi(1, value)))
    if accepted:
        decode_envelope(data, strict=True)
    else:
        with pytest.raises(InvalidEnvelopeError, match="undefined enum value"):
            decode_envelope(data, strict=True)
        decode_envelope(data)


def test_invalid_utf8_is_rejected() -> None:
    for bad in (b"\xc3\x28", b"\xed\xa0\x80", b"\xc0\xaf"):
        data = HEADER + _ld(4, _ld(1, bad))
        with pytest.raises(InvalidEnvelopeError, match="invalid UTF-8"):
            decode_envelope(data, strict=True)


def test_wire_type_mismatch_and_malformed_varints_are_rejected() -> None:
    payload = _ld(4, b"")
    with pytest.raises(InvalidEnvelopeError, match="wire type mismatch"):
        decode_envelope(_ld(1, b"\x01") + payload, strict=True)
    with pytest.raises(InvalidEnvelopeError, match="overflows"):
        decode_envelope(b"\x08" + b"\xff" * 10 + b"\x01" + payload, strict=True)
    with pytest.raises(InvalidEnvelopeError, match="truncated varint"):
        decode_envelope(b"\x08\xff", strict=True)
    with pytest.raises(InvalidEnvelopeError, match="overflows"):
        decode_envelope(b"\x08" + b"\xff" * 9 + b"\x02" + payload, strict=True)
    with pytest.raises(InvalidEnvelopeError, match="field number"):
        decode_envelope(b"\x00\x01" + payload, strict=True)
    with pytest.raises(InvalidEnvelopeError, match="exceeds message bounds"):
        decode_envelope(b"\x22\x0a\x01\x02", strict=True)
    with pytest.raises(InvalidEnvelopeError, match="field number"):
        decode_envelope(_varint(((1 << 29) << 3) | 0) + b"\x01" + payload, strict=True)
