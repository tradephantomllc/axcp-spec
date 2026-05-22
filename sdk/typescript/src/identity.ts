import { createPrivateKey, createPublicKey, randomBytes, sign as nodeSign, verify as nodeVerify } from "node:crypto";
import type { KeyObject } from "node:crypto";
import { base58Decode, base58Encode } from "./base58.js";
import { bytesEqual, decodeBase64Strict, encodeBase64, toUint8Array } from "./bytes.js";
import {
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

const DID_RE = /^did:([^:\s]+):(.+)$/;
const DID_INVALID_CHAR_RE = /[\s\x00-\x1f\x7f]/u;
const ED25519_PKCS8_PREFIX = Buffer.from("302e020100300506032b657004220420", "hex");
const ED25519_SPKI_PREFIX = Buffer.from("302a300506032b6570032100", "hex");

export interface ParsedDID {
  readonly value: string;
  readonly method: string;
  readonly identifier: string;
}

export class DID {
  static parse(value: string): ParsedDID {
    if (!value) {
      throw new InvalidDIDFormat("DID cannot be empty");
    }
    if (DID_INVALID_CHAR_RE.test(value)) {
      throw new InvalidDIDFormat("DID cannot contain whitespace or control characters");
    }
    const match = DID_RE.exec(value);
    if (match === null) {
      throw new InvalidDIDFormat("expected DID format did:<method>:<id>");
    }
    const method = match[1];
    const identifier = match[2];
    if (method === undefined || method.length === 0) {
      throw new InvalidDIDFormat("DID method cannot be empty");
    }
    if (identifier === undefined || identifier.length === 0) {
      throw new InvalidDIDFormat("DID identifier cannot be empty");
    }
    return { value, method, identifier };
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
      throw new DIDResolutionError(`DID not found: ${did}`);
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
  readonly privateSeed: Uint8Array;
  private readonly privateKey: KeyObject;

  private constructor(seed: Uint8Array, did: string) {
    this.privateSeed = new Uint8Array(seed);
    this.privateKey = privateKeyFromSeed(seed);
    this.publicKey = publicKeyFromPrivateKey(this.privateKey);
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

  sign(payload: Uint8Array): Uint8Array {
    return new Uint8Array(nodeSign(null, Buffer.from(payload), this.privateKey));
  }

  verify(payload: Uint8Array, signature: Uint8Array): void {
    verifySignature(this.publicKey, payload, signature);
  }

  toJson(): Required<IdentityJson> {
    return {
      did: this.did,
      public_key_b64: encodeBase64(this.publicKey),
      private_seed_b64: encodeBase64(this.privateSeed),
    };
  }
}

export function didKeyFromPublicKey(publicKeyInput: Uint8Array): string {
  const publicKey = toUint8Array(publicKeyInput);
  if (publicKey.length !== ED25519_PUBLIC_KEY_SIZE) {
    throw new InvalidDIDFormat("Ed25519 public key must be 32 bytes");
  }
  const multicodec = Uint8Array.from([...DID_KEY_ED25519_MULTICODEC_PREFIX, ...publicKey]);
  return `did:key:z${base58Encode(multicodec)}`;
}

export function publicKeyFromDidKey(did: string): Uint8Array {
  const parsed = DID.parse(did);
  if (parsed.method !== "key") {
    throw new InvalidDIDFormat("only did:key can be decoded without a resolver");
  }
  if (!parsed.identifier.startsWith("z")) {
    throw new InvalidDIDFormat("did:key identifier must use base58btc multibase");
  }
  const decoded = base58Decode(parsed.identifier.slice(1));
  const prefix = decoded.slice(0, DID_KEY_ED25519_MULTICODEC_PREFIX.length);
  if (!bytesEqual(prefix, DID_KEY_ED25519_MULTICODEC_PREFIX)) {
    throw new InvalidDIDFormat("did:key is not an Ed25519 verification key");
  }
  const publicKey = decoded.slice(DID_KEY_ED25519_MULTICODEC_PREFIX.length);
  if (publicKey.length !== ED25519_PUBLIC_KEY_SIZE) {
    throw new InvalidDIDFormat("did:key Ed25519 public key must be 32 bytes");
  }
  return publicKey;
}

export async function resolveEd25519PublicKey(did: string, resolver?: DIDResolver): Promise<Uint8Array> {
  const parsed = DID.parse(did);
  if (parsed.method === "key") {
    return publicKeyFromDidKey(did);
  }
  if (resolver === undefined) {
    throw new DIDResolutionError("resolver is required for non did:key identifiers");
  }

  const document = normalizeDIDDocument(await resolver.resolve(did));
  if (document.id !== did) {
    throw new DIDResolutionError("resolved DID document ID does not match request");
  }

  for (const record of document.publicKeys) {
    const allowedType =
      record.type === "Ed25519VerificationKey2020" || record.type === "Ed25519VerificationKey2018";
    if (allowedType && record.publicKeyBytes.length === ED25519_PUBLIC_KEY_SIZE) {
      return new Uint8Array(record.publicKeyBytes);
    }
  }
  throw new DIDResolutionError("no acceptable Ed25519 public key found");
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

  const ok = nodeVerify(null, Buffer.from(payload), publicKeyObjectFromRaw(publicKey), Buffer.from(signature));
  if (!ok) {
    throw new SignatureVerificationError("signature verification failed");
  }
}

function normalizeDIDDocument(document: DIDDocument): DIDDocument {
  return {
    id: document.id,
    publicKeys: document.publicKeys.map((record) => ({
      id: record.id,
      type: record.type,
      publicKeyBytes: toUint8Array(record.publicKeyBytes),
    })),
  };
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
