import { randomUUID } from "node:crypto";
import Long from "long";
import { bytesEqual, encodeBase64, toUint8Array, utf8Bytes } from "./bytes.js";
import {
  AXCPError,
  InvalidEnvelopeError,
  SignatureMissingError,
} from "./errors.js";
import {
  DID,
  type DIDResolver,
  ED25519_SIGNATURE_SIZE,
  type Identity,
  isSmallOrderPublicKey,
  publicKeyFromDidKey,
  resolveEd25519PublicKey,
  verifySignature,
} from "./identity.js";
import { type AxcpEnvelope, AxcpEnvelopeType, toSafeNumber } from "./pb/schema.js";
import { assertStrictWire } from "./pb/strict.js";
import { nowMs, rfc3339SecondsFromMs } from "./timestamp.js";

export const DID_AUTH_TRANSCRIPT_VERSION = "AXCP-DID-AUTH-v1";
export const DEFAULT_PROTOCOL_VERSION = 1;
export const DEFAULT_PROFILE_SECURE_BASELINE = 1;

export interface NewEnvelopeOptions {
  readonly traceId?: string;
  readonly profile?: number;
}

export interface SignEnvelopeOptions {
  /** Recipient DID bound into the signature (required, D4). */
  readonly recipientDid: string;
  /**
   * Replay sequence (> 0). When omitted, the envelope must already carry a non-zero
   * sequence; low-level signing never signs sequence 0 (D7).
   */
  readonly sequence?: number;
  readonly timestampMs?: number;
}

export interface VerifyEnvelopeOptions {
  /** The verifier's own DID; `recipient_did` must equal it (D4). */
  readonly expectedRecipientDid: string;
  /** Explicitly pinned sender key. For did:key senders it must match the DID. */
  readonly publicKey?: Uint8Array;
  /** Resolver consulted for every sender DID, including did:key (D3). */
  readonly resolver?: DIDResolver;
}

export interface DecodeEnvelopeOptions {
  /**
   * Strict decoding for verification (D5): rejects unknown fields at any level,
   * undefined enum values, invalid UTF-8, two distinct payload members and envelopes
   * without a payload.
   */
  readonly strict?: boolean;
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
  try {
    return AxcpEnvelopeType.encode(AxcpEnvelopeType.create(normalized)).finish();
  } catch (cause) {
    if (cause instanceof AXCPError) {
      throw cause;
    }
    throw new InvalidEnvelopeError("failed to encode AXCP envelope");
  }
}

/**
 * Decode an envelope. Use `{ strict: true }` for bytes received from the network
 * before verification; non-strict decoding silently drops unknown fields.
 */
export function decodeEnvelope(data: Uint8Array, options: DecodeEnvelopeOptions = {}): AxcpEnvelope {
  if (!(data instanceof Uint8Array)) {
    throw new InvalidEnvelopeError("envelope bytes must be a Uint8Array");
  }
  if (options.strict === true) {
    assertStrictWire(AxcpEnvelopeType, data, "payload");
  }
  try {
    return AxcpEnvelopeType.decode(data) as unknown as AxcpEnvelope;
  } catch {
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
  options: SignEnvelopeOptions,
): AxcpEnvelope {
  const recipientDid = options?.recipientDid;
  if (typeof recipientDid !== "string" || recipientDid.length === 0) {
    throw new InvalidEnvelopeError("recipientDid is required to sign an envelope");
  }
  DID.parse(recipientDid);
  if (options.timestampMs !== undefined && (!Number.isSafeInteger(options.timestampMs) || options.timestampMs <= 0)) {
    throw new InvalidEnvelopeError("timestampMs must be a positive safe integer");
  }
  const sequence = options.sequence ?? toSafeNumber(envelope.sequence, "sequence");
  if (!Number.isSafeInteger(sequence) || sequence <= 0) {
    throw new InvalidEnvelopeError("sequence must be a positive safe integer");
  }

  envelope.senderDid = identity.did;
  envelope.recipientDid = recipientDid;
  envelope.timestampMs = options.timestampMs ?? nowMs();
  envelope.sequence = sequence;
  envelope.signature = identity.sign(buildAuthTranscript(envelope));
  return envelope;
}

/**
 * Structural Secure Baseline checks (D1, D4, D5) that do not need key material.
 * Throws an SDK error on the first violation.
 */
export function validateEnvelopeForVerification(envelope: AxcpEnvelope, expectedRecipientDid: string): void {
  if (typeof envelope !== "object" || envelope === null) {
    throw new InvalidEnvelopeError("envelope must be an object");
  }
  if (typeof expectedRecipientDid !== "string" || expectedRecipientDid.length === 0) {
    throw new InvalidEnvelopeError("expectedRecipientDid is required for verification");
  }
  const signature = toUint8Array(envelope.signature);
  if (signature.length === 0) {
    throw new SignatureMissingError("envelope signature is missing");
  }
  if (signature.length !== ED25519_SIGNATURE_SIZE) {
    throw new InvalidEnvelopeError("envelope signature must be 64 bytes");
  }
  if ((envelope.version ?? 0) !== DEFAULT_PROTOCOL_VERSION) {
    throw new InvalidEnvelopeError("unsupported envelope version");
  }
  if ((envelope.profile ?? 0) !== DEFAULT_PROFILE_SECURE_BASELINE) {
    throw new InvalidEnvelopeError("envelope profile must be secure-baseline-v1");
  }
  if (!envelope.senderDid) {
    throw new InvalidEnvelopeError("sender_did is required");
  }
  DID.parse(envelope.senderDid);
  if (!envelope.recipientDid) {
    throw new InvalidEnvelopeError("recipient_did is required");
  }
  if (envelope.recipientDid !== expectedRecipientDid) {
    throw new InvalidEnvelopeError("envelope recipient_did does not match the verifier");
  }
  if (toSafeNumber(envelope.sequence, "sequence") <= 0) {
    throw new InvalidEnvelopeError("sequence must be greater than zero");
  }
  toSafeNumber(envelope.timestampMs, "timestampMs");
  // Re-encode and apply the strict wire rules: exactly one payload member (the encoder
  // writes every member present on the object), defined enum values and UTF-8.
  assertStrictWire(AxcpEnvelopeType, encodeEnvelope(envelope), "payload");
}

/**
 * Verify an envelope signature and its Secure Baseline structure. Does not check
 * timestamps or replay; {@link Agent.verify} does both.
 */
export async function verifyEnvelope(envelope: AxcpEnvelope, options: VerifyEnvelopeOptions): Promise<void> {
  validateEnvelopeForVerification(envelope, options?.expectedRecipientDid);
  const senderDid = envelope.senderDid as string;

  let publicKey: Uint8Array;
  if (options.publicKey !== undefined) {
    publicKey = toUint8Array(options.publicKey);
    if (isSmallOrderPublicKey(publicKey)) {
      throw new InvalidEnvelopeError("explicit public key has small order");
    }
    if (DID.parse(senderDid).method === "key" && !bytesEqual(publicKeyFromDidKey(senderDid), publicKey)) {
      throw new InvalidEnvelopeError("explicit public key does not match sender did:key");
    }
  } else {
    publicKey = await resolveEd25519PublicKey(senderDid, options.resolver);
  }

  verifySignature(publicKey, buildAuthTranscript(envelope), toUint8Array(envelope.signature));
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
    // A repeated scalar has presence even when its value is the scalar default.
    // Only fields inside repeated messages are eligible for default omission.
    const normalizedItems = value.map((item) => {
      if (item === undefined || item === null) {
        throw new InvalidEnvelopeError("repeated fields cannot contain null or undefined");
      }
      if (typeof item === "object" && !Long.isLong(item) && !(item instanceof Uint8Array)) {
        return stripProto3Defaults(item);
      }
      return item;
    });
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
