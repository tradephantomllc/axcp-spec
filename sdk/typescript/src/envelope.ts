import { randomUUID } from "node:crypto";
import Long from "long";
import { encodeBase64, toUint8Array, utf8Bytes } from "./bytes.js";
import {
  InvalidEnvelopeError,
  SignatureMissingError,
} from "./errors.js";
import {
  DID,
  type DIDResolver,
  type Identity,
  publicKeyFromDidKey,
  resolveEd25519PublicKey,
  verifySignature,
} from "./identity.js";
import { type AxcpEnvelope, AxcpEnvelopeType, toSafeNumber } from "./pb/schema.js";
import { nowMs, rfc3339SecondsFromMs } from "./timestamp.js";

export const DID_AUTH_TRANSCRIPT_VERSION = "AXCP-DID-AUTH-v1";
export const DEFAULT_PROTOCOL_VERSION = 1;
export const DEFAULT_PROFILE_SECURE_BASELINE = 1;

export interface NewEnvelopeOptions {
  readonly traceId?: string;
  readonly profile?: number;
}

export interface SignEnvelopeOptions {
  readonly recipientDid?: string;
  readonly sequence?: number;
  readonly timestampMs?: number;
}

export interface VerifyEnvelopeOptions {
  readonly publicKey?: Uint8Array;
  readonly resolver?: DIDResolver;
}

export function newEnvelope(options: NewEnvelopeOptions = {}): AxcpEnvelope {
  return {
    version: DEFAULT_PROTOCOL_VERSION,
    traceId: options.traceId ?? randomUUID(),
    profile: options.profile ?? DEFAULT_PROFILE_SECURE_BASELINE,
  };
}

export function encodeEnvelope(envelope: AxcpEnvelope): Uint8Array {
  const normalized = stripProto3Defaults(envelope) as AxcpEnvelope;
  const error = AxcpEnvelopeType.verify(normalized);
  if (error !== null) {
    throw new InvalidEnvelopeError(`invalid AXCP envelope: ${error}`);
  }
  return AxcpEnvelopeType.encode(AxcpEnvelopeType.create(normalized)).finish();
}

export function decodeEnvelope(data: Uint8Array): AxcpEnvelope {
  try {
    return AxcpEnvelopeType.decode(data) as unknown as AxcpEnvelope;
  } catch (error) {
    throw new InvalidEnvelopeError("failed to decode AXCP envelope");
  }
}

export function signingPayload(envelope: AxcpEnvelope): Uint8Array {
  const clone = decodeEnvelope(encodeEnvelope(envelope)) as Record<string, unknown>;
  delete clone.senderDid;
  delete clone.recipientDid;
  delete clone.timestampMs;
  delete clone.signature;
  delete clone.attestationProof;
  return encodeEnvelope(clone as AxcpEnvelope);
}

export function buildAuthTranscript(envelope: AxcpEnvelope): Uint8Array {
  const payload = signingPayload(envelope);
  const payloadB64 = encodeBase64(payload);
  const timestamp = rfc3339SecondsFromMs(toSafeNumber(envelope.timestampMs, "timestampMs"));
  return utf8Bytes(
    [
      DID_AUTH_TRANSCRIPT_VERSION,
      envelope.senderDid ?? "",
      envelope.recipientDid ?? "",
      payloadB64,
      timestamp,
    ].join("\n"),
  );
}

export function signEnvelope(
  envelope: AxcpEnvelope,
  identity: Identity,
  options: SignEnvelopeOptions = {},
): AxcpEnvelope {
  if (options.timestampMs !== undefined && options.timestampMs < 0) {
    throw new InvalidEnvelopeError("timestampMs cannot be negative");
  }
  if (options.sequence !== undefined && options.sequence < 0) {
    throw new InvalidEnvelopeError("sequence cannot be negative");
  }

  envelope.senderDid = identity.did;
  envelope.recipientDid = options.recipientDid ?? "";
  envelope.timestampMs = options.timestampMs ?? nowMs();
  if (options.sequence !== undefined) {
    envelope.sequence = options.sequence;
  }
  envelope.signature = identity.sign(buildAuthTranscript(envelope));
  return envelope;
}

export async function verifyEnvelope(envelope: AxcpEnvelope, options: VerifyEnvelopeOptions = {}): Promise<void> {
  const signature = toUint8Array(envelope.signature);
  if (signature.length === 0) {
    throw new SignatureMissingError("envelope signature is missing");
  }
  if (!envelope.senderDid) {
    throw new InvalidEnvelopeError("sender_did is required");
  }

  const resolvedPublicKey = options.publicKey ?? (await resolveEd25519PublicKey(envelope.senderDid, options.resolver));
  const parsedSender = DID.parse(envelope.senderDid);
  if (
    options.publicKey !== undefined &&
    parsedSender.method === "key" &&
    !bytesMatch(publicKeyFromDidKey(envelope.senderDid), options.publicKey)
  ) {
    throw new InvalidEnvelopeError("explicit public key does not match sender did:key");
  }

  verifySignature(resolvedPublicKey, buildAuthTranscript(envelope), signature);
}

function bytesMatch(left: Uint8Array, right: Uint8Array): boolean {
  if (left.length !== right.length) {
    return false;
  }
  let diff = 0;
  for (let index = 0; index < left.length; index += 1) {
    diff |= left[index]! ^ right[index]!;
  }
  return diff === 0;
}

function stripProto3Defaults(value: unknown): unknown {
  if (value === undefined || value === null) {
    return undefined;
  }
  if (typeof value === "string") {
    return value.length === 0 ? undefined : value;
  }
  if (typeof value === "number") {
    return value === 0 ? undefined : value;
  }
  if (typeof value === "bigint") {
    return value === 0n ? undefined : value;
  }
  if (typeof value === "boolean") {
    return value ? value : undefined;
  }
  if (Long.isLong(value)) {
    return value.isZero() ? undefined : value;
  }
  if (value instanceof Uint8Array) {
    return value.length === 0 ? undefined : value;
  }
  if (Array.isArray(value)) {
    const normalizedItems = value
      .map((item) => stripProto3Defaults(item))
      .filter((item): item is NonNullable<unknown> => item !== undefined);
    return normalizedItems.length === 0 ? undefined : normalizedItems;
  }
  if (typeof value === "object") {
    const normalized: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      const normalizedItem = stripProto3Defaults(item);
      if (normalizedItem !== undefined) {
        normalized[key] = normalizedItem;
      }
    }
    return normalized;
  }
  return value;
}
