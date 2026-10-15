import {
  DeltaOpType,
  type AxcpEnvelope,
  type Identity,
  buildAuthTranscript,
  newEnvelope,
} from "../src/index.js";

export function contextEnvelope(traceId = "trace-123"): AxcpEnvelope {
  const envelope = newEnvelope({ traceId, profile: 1 });
  envelope.contextPatch = {
    contextId: "ctx",
    baseVersion: 7,
    ops: [
      {
        op: DeltaOpType.ADD,
        path: "/x",
        data: new TextEncoder().encode("1"),
        ts: 1,
      },
    ],
  };
  return envelope;
}

/**
 * Sign without any of the SDK's signing-side guards, to build envelopes that only a
 * verifier-side check can reject (empty recipient, sequence 0, other profiles...).
 */
export function rawSign(
  envelope: AxcpEnvelope,
  identity: Identity,
  fields: { recipientDid?: string; sequence?: number; timestampMs?: number },
): AxcpEnvelope {
  envelope.senderDid = identity.did;
  envelope.recipientDid = fields.recipientDid ?? "";
  envelope.timestampMs = fields.timestampMs ?? Date.now();
  envelope.sequence = fields.sequence ?? 0;
  envelope.signature = identity.sign(buildAuthTranscript(envelope));
  return envelope;
}

export function varint(value: number | bigint): number[] {
  let remaining = BigInt(value);
  const out: number[] = [];
  do {
    let byte = Number(remaining & 0x7fn);
    remaining >>= 7n;
    if (remaining > 0n) {
      byte |= 0x80;
    }
    out.push(byte);
  } while (remaining > 0n);
  return out;
}

export function lengthDelimited(fieldNumber: number, body: Uint8Array): Uint8Array {
  return Uint8Array.from([...varint((fieldNumber << 3) | 2), ...varint(body.length), ...body]);
}

export function varintField(fieldNumber: number, value: number | bigint): Uint8Array {
  return Uint8Array.from([...varint(fieldNumber << 3), ...varint(value)]);
}

export function concatBytes(...parts: Uint8Array[]): Uint8Array {
  return new Uint8Array(Buffer.concat(parts.map((part) => Buffer.from(part))));
}
