"""Strict protobuf wire validation for AXCP envelopes (D5).

``google.protobuf`` (upb backend) silently keeps unknown fields, keeps only the last of
two different oneof members and stores undefined enum values, and ``UnknownFields()`` is
not available on every backend. This module therefore scans the raw wire bytes against
the message descriptors and rejects, at every nesting level:

* unknown field numbers and known fields encoded with an unexpected wire type;
* malformed or overflowing varints, truncated fields and invalid tags;
* enum values not defined in the schema (after int32 truncation, as Go does);
* strings that are not valid UTF-8;
* two or more DISTINCT members of the same oneof (repeated occurrences of the same
  member are merged by normal protobuf rules and are accepted).

For the envelope, exactly one ``payload`` member must be present.
"""

from __future__ import annotations

from google.protobuf.descriptor import Descriptor, FieldDescriptor

from axcp.errors import InvalidEnvelopeError
from axcp.pb import axcp_pb2

_MAX_DEPTH = 32
_MAX_FIELD_NUMBER = (1 << 29) - 1

_WIRE_VARINT = 0
_WIRE_FIXED64 = 1
_WIRE_LENGTH_DELIMITED = 2
_WIRE_FIXED32 = 5

_VARINT_TYPES = frozenset(
    {
        FieldDescriptor.TYPE_INT32,
        FieldDescriptor.TYPE_INT64,
        FieldDescriptor.TYPE_UINT32,
        FieldDescriptor.TYPE_UINT64,
        FieldDescriptor.TYPE_SINT32,
        FieldDescriptor.TYPE_SINT64,
        FieldDescriptor.TYPE_BOOL,
        FieldDescriptor.TYPE_ENUM,
    }
)
_FIXED64_TYPES = frozenset(
    {FieldDescriptor.TYPE_FIXED64, FieldDescriptor.TYPE_SFIXED64, FieldDescriptor.TYPE_DOUBLE}
)
_FIXED32_TYPES = frozenset(
    {FieldDescriptor.TYPE_FIXED32, FieldDescriptor.TYPE_SFIXED32, FieldDescriptor.TYPE_FLOAT}
)


def _fail(message: str) -> None:
    raise InvalidEnvelopeError(message)


def _read_varint(data: memoryview, pos: int, end: int) -> tuple[int, int]:
    result = 0
    shift = 0
    for index in range(10):
        if pos >= end:
            _fail("truncated varint")
        byte = data[pos]
        pos += 1
        if index == 9 and byte > 1:
            _fail("varint overflows 64 bits")
        result |= (byte & 0x7F) << shift
        if byte < 0x80:
            return result, pos
        shift += 7
    _fail("varint too long")
    raise AssertionError("unreachable")


def _is_repeated(field: FieldDescriptor) -> bool:
    is_repeated = getattr(field, "is_repeated", None)
    if is_repeated is not None:
        return bool(is_repeated)
    return field.label == FieldDescriptor.LABEL_REPEATED  # pragma: no cover - protobuf < 5


def _expected_wire_type(field: FieldDescriptor) -> int:
    if field.type in _VARINT_TYPES:
        return _WIRE_VARINT
    if field.type in _FIXED64_TYPES:
        return _WIRE_FIXED64
    if field.type in _FIXED32_TYPES:
        return _WIRE_FIXED32
    if field.type in (FieldDescriptor.TYPE_STRING, FieldDescriptor.TYPE_BYTES, FieldDescriptor.TYPE_MESSAGE):
        return _WIRE_LENGTH_DELIMITED
    _fail("unsupported field type in schema")
    raise AssertionError("unreachable")


def _check_enum(field: FieldDescriptor, value: int) -> None:
    if field.type != FieldDescriptor.TYPE_ENUM:
        return
    number = value & 0xFFFFFFFF
    if number >= 0x80000000:
        number -= 0x100000000
    if number not in field.enum_type.values_by_number:
        _fail("undefined enum value")


def _skip_scalar(data: memoryview, pos: int, end: int, wire_type: int, field: FieldDescriptor) -> int:
    if wire_type == _WIRE_VARINT:
        value, pos = _read_varint(data, pos, end)
        _check_enum(field, value)
        return pos
    if wire_type == _WIRE_FIXED64:
        if end - pos < 8:
            _fail("truncated fixed64 field")
        return pos + 8
    if wire_type == _WIRE_FIXED32:
        if end - pos < 4:
            _fail("truncated fixed32 field")
        return pos + 4
    _fail("unexpected wire type")
    raise AssertionError("unreachable")


def _scan_message(
    descriptor: Descriptor, data: memoryview, start: int, end: int, depth: int
) -> dict[str, set[int]]:
    if depth > _MAX_DEPTH:
        _fail("message nesting too deep")
    oneofs: dict[str, set[int]] = {}
    fields = descriptor.fields_by_number
    pos = start
    while pos < end:
        tag, pos = _read_varint(data, pos, end)
        number = tag >> 3
        wire_type = tag & 7
        if number == 0 or number > _MAX_FIELD_NUMBER:
            _fail("invalid field number")
        field = fields.get(number)
        if field is None:
            _fail(f"unknown field {number} in {descriptor.name}")
        expected = _expected_wire_type(field)

        if wire_type == _WIRE_LENGTH_DELIMITED and expected != _WIRE_LENGTH_DELIMITED and _is_repeated(field):
            length, pos = _read_varint(data, pos, end)
            if length > end - pos:
                _fail("length-delimited field exceeds message bounds")
            packed_end = pos + length
            while pos < packed_end:
                pos = _skip_scalar(data, pos, packed_end, expected, field)
        elif wire_type != expected:
            _fail(f"wire type mismatch for field {number} in {descriptor.name}")
        elif wire_type == _WIRE_LENGTH_DELIMITED:
            length, pos = _read_varint(data, pos, end)
            if length > end - pos:
                _fail("length-delimited field exceeds message bounds")
            field_end = pos + length
            if field.type == FieldDescriptor.TYPE_MESSAGE:
                _scan_message(field.message_type, data, pos, field_end, depth + 1)
            elif field.type == FieldDescriptor.TYPE_STRING:
                try:
                    str(data[pos:field_end], "utf-8")
                except UnicodeDecodeError:
                    _fail(f"invalid UTF-8 in field {number} of {descriptor.name}")
            pos = field_end
        else:
            pos = _skip_scalar(data, pos, end, wire_type, field)

        oneof = field.containing_oneof
        if oneof is not None:
            members = oneofs.setdefault(oneof.name, set())
            members.add(number)
            if len(members) > 1:
                _fail(f"multiple members of oneof {oneof.name} in {descriptor.name}")
    return oneofs


def check_strict_wire(descriptor: Descriptor, data: bytes, required_oneof: str | None = None) -> None:
    """Validate ``data`` as a strictly-encoded message of type ``descriptor``.

    When ``required_oneof`` is given, exactly one member of that top-level oneof must be
    present on the wire.
    """

    if not isinstance(data, (bytes, bytearray, memoryview)):
        raise InvalidEnvelopeError("envelope bytes must be bytes-like")
    view = memoryview(data).cast("B") if isinstance(data, memoryview) else memoryview(data)
    oneofs = _scan_message(descriptor, view, 0, len(view), 0)
    if required_oneof is not None and len(oneofs.get(required_oneof, ())) != 1:
        raise InvalidEnvelopeError(f"exactly one {required_oneof} member is required")


def check_envelope_wire(data: bytes) -> None:
    """Strict AXCP envelope wire validation (D5); raises :class:`InvalidEnvelopeError`."""

    check_strict_wire(axcp_pb2.AxcpEnvelope.DESCRIPTOR, data, "payload")
