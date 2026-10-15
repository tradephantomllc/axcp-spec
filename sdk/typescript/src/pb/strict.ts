import protobuf from "protobufjs";
import { InvalidEnvelopeError } from "../errors.js";

/**
 * Strict protobuf wire validation against the reflected schema (D5).
 *
 * Rejects, at every nesting level:
 * - unknown field numbers, and known fields encoded with an unexpected wire type;
 * - malformed or overflowing varints, truncated fields, invalid tags;
 * - enum values that are not defined in the schema (after int32 truncation, as Go does);
 * - strings that are not valid UTF-8;
 * - two or more DISTINCT members of the same oneof (repeated occurrences of the same
 *   member are merged by normal protobuf rules and are accepted).
 *
 * The caller decides how many members of a top-level oneof are required.
 */

const MAX_DEPTH = 32;
const MAX_FIELD_NUMBER = 0x1fffffff;
const utf8Decoder = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true });

const WIRE_VARINT = 0;
const WIRE_FIXED64 = 1;
const WIRE_LENGTH_DELIMITED = 2;
const WIRE_FIXED32 = 5;

const VARINT_TYPES = new Set(["int32", "uint32", "sint32", "int64", "uint64", "sint64", "bool"]);
const FIXED64_TYPES = new Set(["fixed64", "sfixed64", "double"]);
const FIXED32_TYPES = new Set(["fixed32", "sfixed32", "float"]);

interface Cursor {
  pos: number;
}

// Oneof membership is derived from the oneof member *names*: the hand-written schema
// declares each OneOf before its fields, so protobufjs leaves `Field.partOf` unset.
const oneofMembershipCache = new WeakMap<protobuf.Type, Map<string, string>>();

function oneofMembership(type: protobuf.Type): Map<string, string> {
  let membership = oneofMembershipCache.get(type);
  if (membership === undefined) {
    membership = new Map<string, string>();
    for (const oneof of type.oneofsArray) {
      for (const fieldName of oneof.oneof) {
        membership.set(fieldName, oneof.name);
      }
    }
    oneofMembershipCache.set(type, membership);
  }
  return membership;
}

function fail(message: string): never {
  throw new InvalidEnvelopeError(message);
}

/** Returns the low 32 bits of the varint as a signed int32 (Go/protobuf enum semantics). */
function readVarint(data: Uint8Array, cursor: Cursor, end: number): number {
  let low = 0;
  for (let index = 0; index < 10; index += 1) {
    if (cursor.pos >= end) {
      fail("truncated varint");
    }
    const byte = data[cursor.pos]!;
    cursor.pos += 1;
    if (index === 9 && byte > 1) {
      fail("varint overflows 64 bits");
    }
    if (index < 5) {
      low |= (byte & 0x7f) << (7 * index);
    }
    if (byte < 0x80) {
      return low | 0;
    }
  }
  return fail("varint too long");
}

/** Reads a varint as a JavaScript number (exact below 2^53, saturating approximation above). */
function readVarintNumber(data: Uint8Array, cursor: Cursor, end: number): number {
  let value = 0;
  let multiplier = 1;
  for (let index = 0; index < 10; index += 1) {
    if (cursor.pos >= end) {
      fail("truncated varint");
    }
    const byte = data[cursor.pos]!;
    cursor.pos += 1;
    if (index === 9 && byte > 1) {
      fail("varint overflows 64 bits");
    }
    value += (byte & 0x7f) * multiplier;
    multiplier *= 128;
    if (byte < 0x80) {
      return value;
    }
  }
  return fail("varint too long");
}

function readLength(data: Uint8Array, cursor: Cursor, end: number): number {
  const length = readVarintNumber(data, cursor, end);
  if (length > end - cursor.pos) {
    fail("length-delimited field exceeds message bounds");
  }
  return length;
}

function expectedWireType(field: protobuf.Field): number {
  const resolved = field.resolvedType;
  if (resolved instanceof protobuf.Type) {
    return WIRE_LENGTH_DELIMITED;
  }
  if (resolved instanceof protobuf.Enum) {
    return WIRE_VARINT;
  }
  if (field.type === "string" || field.type === "bytes") {
    return WIRE_LENGTH_DELIMITED;
  }
  if (VARINT_TYPES.has(field.type)) {
    return WIRE_VARINT;
  }
  if (FIXED64_TYPES.has(field.type)) {
    return WIRE_FIXED64;
  }
  if (FIXED32_TYPES.has(field.type)) {
    return WIRE_FIXED32;
  }
  return fail("unsupported field type in schema");
}

function isPackable(field: protobuf.Field): boolean {
  return field.repeated && expectedWireType(field) !== WIRE_LENGTH_DELIMITED;
}

function checkEnum(field: protobuf.Field, value: number): void {
  const resolved = field.resolvedType;
  if (resolved instanceof protobuf.Enum && resolved.valuesById[value] === undefined) {
    fail("undefined enum value");
  }
}

function skipScalar(data: Uint8Array, cursor: Cursor, end: number, wireType: number, field: protobuf.Field): void {
  switch (wireType) {
    case WIRE_VARINT:
      checkEnum(field, readVarint(data, cursor, end));
      return;
    case WIRE_FIXED64:
      if (end - cursor.pos < 8) {
        fail("truncated fixed64 field");
      }
      cursor.pos += 8;
      return;
    case WIRE_FIXED32:
      if (end - cursor.pos < 4) {
        fail("truncated fixed32 field");
      }
      cursor.pos += 4;
      return;
    default:
      fail("unexpected wire type");
  }
}

/**
 * Validate one message body. Returns, per oneof name, the set of distinct member field
 * numbers present on the wire.
 */
function scanMessage(
  type: protobuf.Type,
  data: Uint8Array,
  start: number,
  end: number,
  depth: number,
): Map<string, Set<number>> {
  if (depth > MAX_DEPTH) {
    fail("message nesting too deep");
  }
  const oneofs = new Map<string, Set<number>>();
  const membership = oneofMembership(type);
  const cursor: Cursor = { pos: start };
  while (cursor.pos < end) {
    const tag = readVarintNumber(data, cursor, end);
    const fieldNumber = Math.floor(tag / 8);
    const wireType = tag % 8;
    if (fieldNumber === 0 || fieldNumber > MAX_FIELD_NUMBER) {
      fail("invalid field number");
    }
    const field = type.fieldsById[fieldNumber];
    if (field === undefined) {
      fail(`unknown field ${fieldNumber} in ${type.name}`);
    }
    const expected = expectedWireType(field);

    if (wireType === WIRE_LENGTH_DELIMITED && expected !== WIRE_LENGTH_DELIMITED && isPackable(field)) {
      const length = readLength(data, cursor, end);
      const packedEnd = cursor.pos + length;
      while (cursor.pos < packedEnd) {
        skipScalar(data, cursor, packedEnd, expected, field);
      }
    } else if (wireType !== expected) {
      fail(`wire type mismatch for field ${fieldNumber} in ${type.name}`);
    } else if (wireType === WIRE_LENGTH_DELIMITED) {
      const length = readLength(data, cursor, end);
      const fieldStart = cursor.pos;
      const fieldEnd = fieldStart + length;
      const resolved = field.resolvedType;
      if (resolved instanceof protobuf.Type) {
        scanMessage(resolved, data, fieldStart, fieldEnd, depth + 1);
      } else if (field.type === "string") {
        try {
          utf8Decoder.decode(data.subarray(fieldStart, fieldEnd));
        } catch {
          fail(`invalid UTF-8 in field ${fieldNumber} of ${type.name}`);
        }
      }
      cursor.pos = fieldEnd;
    } else {
      skipScalar(data, cursor, end, wireType, field);
    }

    const oneofName = membership.get(field.name) ?? field.partOf?.name;
    if (oneofName !== undefined) {
      let members = oneofs.get(oneofName);
      if (members === undefined) {
        members = new Set<number>();
        oneofs.set(oneofName, members);
      }
      members.add(fieldNumber);
      if (members.size > 1) {
        fail(`multiple members of oneof ${oneofName} in ${type.name}`);
      }
    }
  }
  return oneofs;
}

/**
 * Validate `data` as a strictly-encoded `type` message. When `requiredOneof` is given,
 * exactly one member of that top-level oneof must be present on the wire.
 */
export function assertStrictWire(type: protobuf.Type, data: Uint8Array, requiredOneof?: string): void {
  const oneofs = scanMessage(type, data, 0, data.length, 0);
  if (requiredOneof !== undefined && (oneofs.get(requiredOneof)?.size ?? 0) !== 1) {
    fail(`exactly one ${requiredOneof} member is required`);
  }
}
