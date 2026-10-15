import { createPrivateKey, createPublicKey, randomBytes, sign as nodeSign, verify as nodeVerify } from "node:crypto";
import type { KeyObject } from "node:crypto";
import { inspect } from "node:util";
import { base58Decode, base58Encode } from "./base58.js";
import { bytesEqual, decodeBase64Strict, encodeBase64, toUint8Array } from "./bytes.js";
import {
  AXCPError,
  DIDResolutionError,
  InvalidDIDFormat,
  InvalidPrivateKeyError,
  KeyMismatchError,
  SignatureVerificationError,
} from "./errors.js";

export const ED25519_PUBLIC_KEY_SIZE = 32;
export const ED25519_PRIVATE_SEED_SIZE = 32;
export const ED25519_SIGNATURE_SIZE = 64;
export const DID_KEY_ED25519_MULTICODEC_PREFIX = Uint8Array.from([0xed, 0x01]);

/** Maximum DID length in bytes (D1). Checked before any other parsing. */
export const MAX_DID_LENGTH = 512;
/** Maximum did:key method-specific identifier length, checked before base58 decoding (D2). */
export const MAX_DID_KEY_ID_LENGTH = 128;
/** Verification key types accepted from resolver documents (D3). */
export const ALLOWED_VERIFICATION_KEY_TYPES: readonly string[] = Object.freeze([
  "Ed25519VerificationKey2020",
  "Ed25519VerificationKey2018",
]);

const DID_KEY_DECODED_LENGTH = DID_KEY_ED25519_MULTICODEC_PREFIX.length + ED25519_PUBLIC_KEY_SIZE;
const ED25519_PKCS8_PREFIX = Buffer.from("302e020100300506032b657004220420", "hex");
const ED25519_SPKI_PREFIX = Buffer.from("302a300506032b6570032100", "hex");

// Small-order Ed25519 points (little-endian encodings with the sign bit cleared),
// identical to the libsodium blocklist (D9).
const SMALL_ORDER_PUBLIC_KEYS: readonly Uint8Array[] = [
  "0000000000000000000000000000000000000000000000000000000000000000",
  "0100000000000000000000000000000000000000000000000000000000000000",
  "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
  "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
  "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
  "edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
  "eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
].map((hex) => Uint8Array.from(Buffer.from(hex, "hex")));

export interface ParsedDID {
  readonly value: string;
  readonly method: string;
  readonly identifier: string;
}

function isLowerAlphaOrDigit(code: number): boolean {
  return (code >= 0x61 && code <= 0x7a) || (code >= 0x30 && code <= 0x39);
}

function isHexDigit(code: number): boolean {
  return (code >= 0x30 && code <= 0x39) || (code >= 0x41 && code <= 0x46) || (code >= 0x61 && code <= 0x66);
}

function isIdChar(code: number): boolean {
  return (
    (code >= 0x41 && code <= 0x5a) ||
    (code >= 0x61 && code <= 0x7a) ||
    (code >= 0x30 && code <= 0x39) ||
    code === 0x2e ||
    code === 0x2d ||
    code === 0x5f
  );
}

function describeInvalidDIDChar(code: number): string {
  if (code === 0x2f || code === 0x3f || code === 0x23) {
    return "DID cannot contain a path, query or fragment";
  }
  if (code <= 0x20 || code === 0x7f) {
    return "DID cannot contain whitespace or control characters";
  }
  if (code > 0x7f) {
    return "DID must be ASCII";
  }
  return "DID contains an invalid character";
}

export class DID {
  /**
   * Parse a DID using the W3C DID Core ABNF (D1): `did:<method>:<method-specific-id>`,
   * lowercase/digit method names, ASCII-only identifiers, no path/query/fragment and at
   * most {@link MAX_DID_LENGTH} bytes. The length is checked before anything else.
   */
  static parse(value: string): ParsedDID {
    if (typeof value !== "string") {
      throw new InvalidDIDFormat("DID must be a string");
    }
    if (value.length === 0) {
      throw new InvalidDIDFormat("DID cannot be empty");
    }
    // Every UTF-16 code unit encodes to at least one UTF-8 byte, and non-ASCII input is
    // rejected below, so a code-unit count above the limit always exceeds the byte limit.
    if (value.length > MAX_DID_LENGTH) {
      throw new InvalidDIDFormat(`DID exceeds ${MAX_DID_LENGTH} bytes`);
    }
    if (!value.startsWith("did:")) {
      throw new InvalidDIDFormat("expected DID format did:<method>:<id>");
    }

    let index = 4;
    const methodStart = index;
    while (index < value.length && isLowerAlphaOrDigit(value.charCodeAt(index))) {
      index += 1;
    }
    if (index === methodStart) {
      throw new InvalidDIDFormat("DID method must be non-empty lowercase letters or digits");
    }
    if (index >= value.length || value.charCodeAt(index) !== 0x3a) {
      throw new InvalidDIDFormat("DID method must be lowercase letters or digits followed by ':'");
    }
    const method = value.slice(methodStart, index);
    index += 1;

    const identifierStart = index;
    if (identifierStart === value.length) {
      throw new InvalidDIDFormat("DID identifier cannot be empty");
    }
    let segmentLength = 0;
    while (index < value.length) {
      const code = value.charCodeAt(index);
      if (code === 0x3a) {
        segmentLength = 0;
        index += 1;
        continue;
      }
      if (code === 0x25) {
        if (
          index + 2 >= value.length ||
          !isHexDigit(value.charCodeAt(index + 1)) ||
          !isHexDigit(value.charCodeAt(index + 2))
        ) {
          throw new InvalidDIDFormat("DID contains an invalid percent-encoding");
        }
        index += 3;
        segmentLength += 1;
        continue;
      }
      if (!isIdChar(code)) {
        throw new InvalidDIDFormat(describeInvalidDIDChar(code));
      }
      index += 1;
      segmentLength += 1;
    }
    if (segmentLength === 0) {
      throw new InvalidDIDFormat("DID identifier cannot end with ':'");
    }
    return { value, method, identifier: value.slice(identifierStart) };
  }
}

export interface PublicKeyRecord {
  readonly id: string;
  readonly type: string;
  readonly publicKeyBytes: Uint8Array;
}

export interface DIDDocument {
  readonly id: string;
  readonly publicKeys: readonly PublicKeyRecord[];
}

/**
 * Resolves DIDs to verification keys. When a resolver is configured on a verifier it is
 * consulted for every sender, including did:key (it acts as an allowlist, D3).
 */
export interface DIDResolver {
  resolve(did: string): DIDDocument | Promise<DIDDocument>;
}

export class InMemoryDIDResolver implements DIDResolver {
  private readonly documents = new Map<string, DIDDocument>();

  constructor(documents: Iterable<readonly [string, DIDDocument]> = []) {
    for (const [did, document] of documents) {
      this.documents.set(did, normalizeDIDDocument(document));
    }
  }

  register(identity: Identity, keyId = "key-1"): void {
    this.documents.set(identity.did, {
      id: identity.did,
      publicKeys: [
        {
          id: keyId,
          type: "Ed25519VerificationKey2020",
          publicKeyBytes: identity.publicKey,
        },
      ],
    });
  }

  resolve(did: string): DIDDocument {
    const document = this.documents.get(did);
    if (document === undefined) {
      throw new DIDResolutionError("DID not found");
    }
    return normalizeDIDDocument(document);
  }
}

export interface IdentityJson {
  readonly did?: string;
  readonly public_key_b64?: string;
  readonly private_seed_b64?: string;
}

export class Identity {
  readonly did: string;
  readonly publicKey: Uint8Array;
  readonly #seed: Uint8Array;
  readonly #privateKey: KeyObject;

  private constructor(seed: Uint8Array, did: string) {
    this.#seed = new Uint8Array(seed);
    this.#privateKey = privateKeyFromSeed(seed);
    this.publicKey = publicKeyFromPrivateKey(this.#privateKey);
    this.did = did;
  }

  static generate(): Identity {
    return Identity.fromSeed(randomBytes(ED25519_PRIVATE_SEED_SIZE));
  }

  static fromSeed(seedInput: Uint8Array, did?: string): Identity {
    const seed = toUint8Array(seedInput);
    if (seed.length !== ED25519_PRIVATE_SEED_SIZE) {
      throw new InvalidPrivateKeyError("Ed25519 private seed must be 32 bytes");
    }

    const privateKey = privateKeyFromSeed(seed);
    const publicKey = publicKeyFromPrivateKey(privateKey);
    const resolvedDid = did === undefined ? didKeyFromPublicKey(publicKey) : DID.parse(did).value;
    if (DID.parse(resolvedDid).method === "key" && !bytesEqual(publicKeyFromDidKey(resolvedDid), publicKey)) {
      throw new KeyMismatchError("DID public key does not match private key");
    }
    return new Identity(seed, resolvedDid);
  }

  static fromJson(data: IdentityJson): Identity {
    if (!data.private_seed_b64) {
      throw new InvalidPrivateKeyError("private_seed_b64 is required");
    }
    const seed = decodeBase64Strict(data.private_seed_b64, "private_seed_b64");
    const identity = Identity.fromSeed(seed, data.did);
    if (data.public_key_b64 !== undefined) {
      const declaredPublicKey = decodeBase64Strict(data.public_key_b64, "public_key_b64");
      if (!bytesEqual(declaredPublicKey, identity.publicKey)) {
        throw new KeyMismatchError("public_key_b64 does not match private_seed_b64");
      }
    }
    return identity;
  }

  /**
   * Explicitly export a copy of the 32-byte Ed25519 private seed. Handle with care:
   * the seed is never included in `JSON.stringify`, `util.inspect` or `console.log`.
   */
  exportPrivateSeed(): Uint8Array {
    return new Uint8Array(this.#seed);
  }

  /** @deprecated Use {@link Identity.exportPrivateSeed}. Not enumerable and never printed. */
  get privateSeed(): Uint8Array {
    return this.exportPrivateSeed();
  }

  sign(payload: Uint8Array): Uint8Array {
    return new Uint8Array(nodeSign(null, Buffer.from(payload), this.#privateKey));
  }

  verify(payload: Uint8Array, signature: Uint8Array): void {
    verifySignature(this.publicKey, payload, signature);
  }

  /**
   * Explicit persistence format, INCLUDING the private seed. Use {@link Identity.toJSON}
   * (what `JSON.stringify` calls) for a public-only view.
   */
  toJson(): Required<IdentityJson> {
    return {
      did: this.did,
      public_key_b64: encodeBase64(this.publicKey),
      private_seed_b64: encodeBase64(this.#seed),
    };
  }

  /** Public-only JSON view used by `JSON.stringify`; never contains private key material. */
  toJSON(): { did: string; public_key_b64: string } {
    return { did: this.did, public_key_b64: encodeBase64(this.publicKey) };
  }

  [inspect.custom](): string {
    return `Identity { did: ${JSON.stringify(this.did)}, publicKey: ${JSON.stringify(encodeBase64(this.publicKey))} }`;
  }
}

/** True when the key is one of the small-order Ed25519 points (sign bit ignored, D9). */
export function isSmallOrderPublicKey(publicKeyInput: Uint8Array): boolean {
  if (publicKeyInput.length !== ED25519_PUBLIC_KEY_SIZE) {
    return false;
  }
  const normalized = Uint8Array.from(publicKeyInput);
  normalized[31] = normalized[31]! & 0x7f;
  return SMALL_ORDER_PUBLIC_KEYS.some((candidate) => bytesEqual(candidate, normalized));
}

export function didKeyFromPublicKey(publicKeyInput: Uint8Array): string {
  const publicKey = toUint8Array(publicKeyInput);
  if (publicKey.length !== ED25519_PUBLIC_KEY_SIZE) {
    throw new InvalidDIDFormat("Ed25519 public key must be 32 bytes");
  }
  const multicodec = Uint8Array.from([...DID_KEY_ED25519_MULTICODEC_PREFIX, ...publicKey]);
  return `did:key:z${base58Encode(multicodec)}`;
}

/** Decode an Ed25519 did:key (D1, D2, D9). */
export function publicKeyFromDidKey(did: string): Uint8Array {
  const parsed = DID.parse(did);
  if (parsed.method !== "key") {
    throw new InvalidDIDFormat("only did:key can be decoded without a resolver");
  }
  if (!parsed.identifier.startsWith("z")) {
    throw new InvalidDIDFormat("did:key identifier must use base58btc multibase");
  }
  if (parsed.identifier.length > MAX_DID_KEY_ID_LENGTH) {
    throw new InvalidDIDFormat(`did:key identifier exceeds ${MAX_DID_KEY_ID_LENGTH} characters`);
  }
  const decoded = base58Decode(parsed.identifier.slice(1), MAX_DID_KEY_ID_LENGTH);
  if (decoded.length !== DID_KEY_DECODED_LENGTH) {
    throw new InvalidDIDFormat("did:key must decode to the Ed25519 multicodec prefix and a 32-byte key");
  }
  const prefix = decoded.subarray(0, DID_KEY_ED25519_MULTICODEC_PREFIX.length);
  if (!bytesEqual(prefix, DID_KEY_ED25519_MULTICODEC_PREFIX)) {
    throw new InvalidDIDFormat("did:key is not an Ed25519 verification key");
  }
  const publicKey = decoded.slice(DID_KEY_ED25519_MULTICODEC_PREFIX.length);
  if (isSmallOrderPublicKey(publicKey)) {
    throw new InvalidDIDFormat("did:key encodes a small-order Ed25519 public key");
  }
  return publicKey;
}

/**
 * Resolve the Ed25519 verification key for a DID (D3).
 *
 * - With a resolver: the resolver is consulted for every DID, including did:key. For
 *   did:key the resolved key must equal the key encoded in the DID.
 * - Without a resolver: did:key is decoded directly (open mode); other methods are rejected.
 */
export async function resolveEd25519PublicKey(did: string, resolver?: DIDResolver): Promise<Uint8Array> {
  const parsed = DID.parse(did);
  const didKeyPublicKey = parsed.method === "key" ? publicKeyFromDidKey(did) : undefined;
  if (resolver === undefined) {
    if (didKeyPublicKey !== undefined) {
      return didKeyPublicKey;
    }
    throw new DIDResolutionError("resolver is required for non did:key identifiers");
  }

  let rawDocument: DIDDocument;
  try {
    rawDocument = await resolver.resolve(did);
  } catch (error) {
    if (error instanceof AXCPError) {
      throw error;
    }
    throw new DIDResolutionError("DID resolution failed");
  }
  const document = normalizeDIDDocument(rawDocument);
  if (document.id !== did) {
    throw new DIDResolutionError("resolved DID document ID does not match request");
  }

  const record = document.publicKeys.find(
    (candidate) =>
      ALLOWED_VERIFICATION_KEY_TYPES.includes(candidate.type) &&
      candidate.publicKeyBytes.length === ED25519_PUBLIC_KEY_SIZE,
  );
  if (record === undefined) {
    throw new DIDResolutionError("no acceptable Ed25519 public key found");
  }
  const publicKey = new Uint8Array(record.publicKeyBytes);
  if (isSmallOrderPublicKey(publicKey)) {
    throw new DIDResolutionError("resolved Ed25519 public key has small order");
  }
  if (didKeyPublicKey !== undefined && !bytesEqual(publicKey, didKeyPublicKey)) {
    throw new KeyMismatchError("resolved key does not match the key encoded in the did:key");
  }
  return publicKey;
}

export function verifySignature(publicKeyInput: Uint8Array, payload: Uint8Array, signatureInput: Uint8Array): void {
  const publicKey = toUint8Array(publicKeyInput);
  const signature = toUint8Array(signatureInput);
  if (publicKey.length !== ED25519_PUBLIC_KEY_SIZE) {
    throw new SignatureVerificationError("Ed25519 public key must be 32 bytes");
  }
  if (signature.length !== ED25519_SIGNATURE_SIZE) {
    throw new SignatureVerificationError("Ed25519 signature must be 64 bytes");
  }
  if (isSmallOrderPublicKey(publicKey)) {
    throw new SignatureVerificationError("Ed25519 public key has small order");
  }

  let ok: boolean;
  try {
    ok = nodeVerify(null, Buffer.from(payload), publicKeyObjectFromRaw(publicKey), Buffer.from(signature));
  } catch {
    throw new SignatureVerificationError("signature verification failed");
  }
  if (!ok) {
    throw new SignatureVerificationError("signature verification failed");
  }
}

function normalizeDIDDocument(document: DIDDocument): DIDDocument {
  try {
    if (typeof document !== "object" || document === null || typeof document.id !== "string") {
      throw new DIDResolutionError("resolver returned an invalid DID document");
    }
    if (!Array.isArray(document.publicKeys)) {
      throw new DIDResolutionError("resolver returned an invalid DID document");
    }
    return {
      id: document.id,
      publicKeys: document.publicKeys.map((record: PublicKeyRecord) => ({
        id: String(record.id),
        type: String(record.type),
        publicKeyBytes: toUint8Array(record.publicKeyBytes),
      })),
    };
  } catch (error) {
    if (error instanceof AXCPError) {
      throw error;
    }
    throw new DIDResolutionError("resolver returned an invalid DID document");
  }
}

function privateKeyFromSeed(seed: Uint8Array): KeyObject {
  if (seed.length !== ED25519_PRIVATE_SEED_SIZE) {
    throw new InvalidPrivateKeyError("Ed25519 private seed must be 32 bytes");
  }
  return createPrivateKey({
    key: Buffer.concat([ED25519_PKCS8_PREFIX, Buffer.from(seed)]),
    format: "der",
    type: "pkcs8",
  });
}

function publicKeyObjectFromRaw(publicKey: Uint8Array): KeyObject {
  if (publicKey.length !== ED25519_PUBLIC_KEY_SIZE) {
    throw new SignatureVerificationError("Ed25519 public key must be 32 bytes");
  }
  return createPublicKey({
    key: Buffer.concat([ED25519_SPKI_PREFIX, Buffer.from(publicKey)]),
    format: "der",
    type: "spki",
  });
}

function publicKeyFromPrivateKey(privateKey: KeyObject): Uint8Array {
  const publicKeyDer = createPublicKey(privateKey).export({ format: "der", type: "spki" });
  const prefix = publicKeyDer.subarray(0, ED25519_SPKI_PREFIX.length);
  if (!prefix.equals(ED25519_SPKI_PREFIX) || publicKeyDer.length !== ED25519_SPKI_PREFIX.length + 32) {
    throw new InvalidPrivateKeyError("failed to derive Ed25519 public key from private seed");
  }
  return new Uint8Array(publicKeyDer.subarray(ED25519_SPKI_PREFIX.length));
}
