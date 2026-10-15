import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { verify as nodeVerify, createPublicKey } from "node:crypto";
import { performance } from "node:perf_hooks";
import { inspect } from "node:util";
import {
  Agent,
  DID,
  DIDResolutionError,
  Identity,
  InMemoryDIDResolver,
  InvalidDIDFormat,
  KeyMismatchError,
  MAX_DID_LENGTH,
  SignatureVerificationError,
  didKeyFromPublicKey,
  encodeEnvelope,
  isSmallOrderPublicKey,
  publicKeyFromDidKey,
  resolveEd25519PublicKey,
  verifySignature,
  type DIDDocument,
  type DIDResolver,
} from "../src/index.js";
import { contextEnvelope } from "./helpers.js";

const SMALL_ORDER_HEX = [
  "0000000000000000000000000000000000000000000000000000000000000000",
  "0100000000000000000000000000000000000000000000000000000000000000",
  "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
  "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
  "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
  "edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
  "eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
];

describe("identity", () => {
  it("generates a did:key identity", () => {
    const identity = Identity.generate();

    const parsed = DID.parse(identity.did);
    assert.equal(parsed.method, "key");
    assert.deepEqual(publicKeyFromDidKey(identity.did), identity.publicKey);
    assert.equal(didKeyFromPublicKey(identity.publicKey), identity.did);
  });

  it("roundtrips seed JSON", () => {
    const identity = Identity.generate();
    const restored = Identity.fromJson(identity.toJson());

    assert.equal(restored.did, identity.did);
    assert.deepEqual(restored.publicKey, identity.publicKey);
    assert.deepEqual(restored.exportPrivateSeed(), identity.exportPrivateSeed());
  });

  it("signs and verifies payloads", () => {
    const identity = Identity.generate();
    const payload = new TextEncoder().encode("AXCP test payload");
    const signature = identity.sign(payload);

    identity.verify(payload, signature);
    assert.throws(() => identity.verify(new TextEncoder().encode("tampered"), signature), SignatureVerificationError);
  });

  it("rejects did:key private key mismatches", () => {
    const left = Identity.generate();
    const right = Identity.generate();

    assert.throws(() => Identity.fromSeed(left.exportPrivateSeed(), right.did), KeyMismatchError);
  });

  it("allows non-key DID labels", () => {
    const identity = Identity.generate();
    const labeled = Identity.fromSeed(identity.exportPrivateSeed(), "did:example:alice");

    assert.equal(labeled.did, "did:example:alice");
    assert.deepEqual(labeled.publicKey, identity.publicKey);
  });

  it("never prints or serializes the private seed (D12)", () => {
    const identity = Identity.generate();
    const seed = Buffer.from(identity.exportPrivateSeed());
    const forbidden = [seed.toString("hex"), seed.toString("base64"), [...seed].join(", ")];
    const rendered = [inspect(identity, { showHidden: true, depth: 5 }), JSON.stringify(identity), String(Object.keys(identity))];
    for (const text of rendered) {
      for (const secret of forbidden) {
        assert.ok(!text.includes(secret), `private seed leaked in ${text}`);
      }
    }
    assert.ok(!Object.keys(identity).includes("privateSeed"));
    assert.deepEqual(Object.keys(JSON.parse(JSON.stringify(identity))).sort(), ["did", "public_key_b64"]);
    assert.ok(inspect(identity).includes(identity.did));
    // The explicit persistence format still carries the seed.
    assert.equal(identity.toJson().private_seed_b64, seed.toString("base64"));
  });
});

describe("DID syntax (D1)", () => {
  it("accepts valid DIDs", () => {
    for (const value of [
      "did:example:alice",
      "did:web:example.com:user:alice",
      "did:example:a%20b",
      "did:example::alice",
      "did:example:A-b_c.d",
      "did:3:x",
    ]) {
      assert.equal(DID.parse(value).value, value);
    }
    assert.equal(DID.parse("did:web:example.com:user").identifier, "example.com:user");
  });

  it("rejects invalid DIDs", () => {
    for (const value of [
      "",
      "alice",
      "did::alice",
      "did:example:",
      "did:example:alice bob",
      "did:Example:alice",
      "did:ex-ample:alice",
      "did:example:alice/path",
      "did:example:alice?query=1",
      "did:example:alice#key-1",
      "did:example:alice:",
      "did:example:a%2",
      "did:example:a%zz",
      "did:example:alé",
      "did:example:a​b",
      "did:example:a\nb",
      "DID:example:alice",
    ]) {
      assert.throws(() => DID.parse(value), InvalidDIDFormat, JSON.stringify(value));
    }
  });

  it("enforces the 512-byte limit", () => {
    const prefix = "did:example:";
    const atLimit = prefix + "a".repeat(MAX_DID_LENGTH - prefix.length);
    assert.equal(DID.parse(atLimit).value.length, MAX_DID_LENGTH);
    assert.throws(() => DID.parse(`${atLimit}a`), /exceeds 512 bytes/);
  });

  it("rejects a 10 MB DID in under 50 ms", async () => {
    const huge = `did:key:z${"6".repeat(10 * 1024 * 1024)}`;
    const started = performance.now();
    assert.throws(() => DID.parse(huge), /exceeds 512 bytes/);
    assert.throws(() => publicKeyFromDidKey(huge), /exceeds 512 bytes/);
    await assert.rejects(() => resolveEd25519PublicKey(huge), /exceeds 512 bytes/);
    assert.ok(performance.now() - started < 50);
  });
});

describe("did:key limits (D2)", () => {
  it("requires the z multibase prefix", () => {
    const identity = Identity.generate();
    assert.throws(() => publicKeyFromDidKey(identity.did.replace(":z", ":Z")), /base58btc multibase/);
  });

  it("rejects identifiers longer than 128 characters before decoding", () => {
    const id = `z${"1".repeat(128)}`;
    assert.throws(() => publicKeyFromDidKey(`did:key:${id}`), /exceeds 128 characters/);
  });

  it("requires exactly 0xed01 + 32 bytes", () => {
    const key = Identity.generate().publicKey;
    const tooShort = Uint8Array.from([0xed, 0x01, ...key.subarray(0, 31)]);
    const tooLong = Uint8Array.from([0xed, 0x01, ...key, 0x00]);
    const x25519 = Uint8Array.from([0xec, 0x01, ...key]);
    for (const bytes of [tooShort, tooLong]) {
      const encoded = `did:key:z${encodeRaw(bytes)}`;
      assert.throws(() => publicKeyFromDidKey(encoded), /multicodec prefix and a 32-byte key/);
    }
    assert.throws(() => publicKeyFromDidKey(`did:key:z${encodeRaw(x25519)}`), /not an Ed25519/);
    assert.deepEqual(publicKeyFromDidKey(`did:key:z${encodeRaw(Uint8Array.from([0xed, 0x01, ...key]))}`), key);
  });

  it("rejects small-order keys encoded in did:key", () => {
    for (const hex of SMALL_ORDER_HEX) {
      const did = didKeyFromPublicKey(Buffer.from(hex, "hex"));
      assert.throws(() => publicKeyFromDidKey(did), /small-order/);
    }
  });
});

function encodeRaw(bytes: Uint8Array): string {
  // Independent base58btc encoder (didKeyFromPublicKey only accepts 32-byte keys).
  const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
  let value = 0n;
  for (const byte of bytes) {
    value = (value << 8n) + BigInt(byte);
  }
  let out = "";
  while (value > 0n) {
    out = alphabet[Number(value % 58n)] + out;
    value /= 58n;
  }
  for (const byte of bytes) {
    if (byte !== 0) {
      break;
    }
    out = `1${out}`;
  }
  return out;
}

describe("small-order public keys (D9)", () => {
  it("flags every blocklisted key, with and without the sign bit", () => {
    for (const hex of SMALL_ORDER_HEX) {
      const key = Uint8Array.from(Buffer.from(hex, "hex"));
      assert.ok(isSmallOrderPublicKey(key), hex);
      const signed = Uint8Array.from(key);
      signed[31] = signed[31]! | 0x80;
      assert.ok(isSmallOrderPublicKey(signed), `${hex} with sign bit`);
      assert.throws(() => verifySignature(key, new Uint8Array(1), new Uint8Array(64)), /small order/);
    }
    assert.ok(!isSmallOrderPublicKey(Identity.generate().publicKey));
  });

  it("blocks a universal forgery that the raw Ed25519 verifier accepts", () => {
    const identityPoint = Uint8Array.from(Buffer.from(SMALL_ORDER_HEX[1]!, "hex"));
    const forged = new Uint8Array(64);
    forged[0] = 1; // R = identity point, S = 0
    const message = new TextEncoder().encode("any message");
    const spki = Buffer.concat([Buffer.from("302a300506032b6570032100", "hex"), Buffer.from(identityPoint)]);
    const rawAccepts = nodeVerify(null, message, createPublicKey({ key: spki, format: "der", type: "spki" }), forged);
    assert.equal(rawAccepts, true, "OpenSSL accepts the forgery, so the explicit blocklist is required");
    assert.throws(() => verifySignature(identityPoint, message, forged), SignatureVerificationError);
  });
});

class StaticResolver implements DIDResolver {
  constructor(private readonly document: DIDDocument | Error) {}
  resolve(): DIDDocument {
    if (this.document instanceof Error) {
      throw this.document;
    }
    return this.document;
  }
}

describe("resolver precedence (D3)", () => {
  it("consults the resolver for did:key senders (allowlist)", async () => {
    const alice = new Agent(Identity.generate());
    const resolver = new InMemoryDIDResolver();
    const bob = new Agent(Identity.generate(), { resolver });
    const envelope = alice.signMessage(contextEnvelope(), { recipientDid: bob.identity.did });

    await assert.rejects(() => bob.verifyBytes(encodeEnvelope(envelope)), DIDResolutionError);
    resolver.register(alice.identity);
    await bob.verifyBytes(encodeEnvelope(envelope));
  });

  it("requires the resolved key to equal the did:key key", async () => {
    const alice = Identity.generate();
    const other = Identity.generate();
    const resolver = new StaticResolver({
      id: alice.did,
      publicKeys: [{ id: "k", type: "Ed25519VerificationKey2020", publicKeyBytes: other.publicKey }],
    });
    await assert.rejects(() => resolveEd25519PublicKey(alice.did, resolver), KeyMismatchError);
  });

  it("validates resolver documents", async () => {
    const did = "did:example:alice";
    const key = Identity.generate().publicKey;
    const cases: Array<[DIDDocument | Error, RegExp]> = [
      [{ id: "did:example:mallory", publicKeys: [{ id: "k", type: "Ed25519VerificationKey2020", publicKeyBytes: key }] }, /does not match/],
      [{ id: did, publicKeys: [{ id: "k", type: "X25519KeyAgreementKey2019", publicKeyBytes: key }] }, /no acceptable/],
      [{ id: did, publicKeys: [{ id: "k", type: "Ed25519VerificationKey2018", publicKeyBytes: key.subarray(0, 31) }] }, /no acceptable/],
      [
        { id: did, publicKeys: [{ id: "k", type: "Ed25519VerificationKey2020", publicKeyBytes: new Uint8Array(32) }] },
        /small order/,
      ],
      [new TypeError("network down"), /DID resolution failed/],
      [{ id: did } as unknown as DIDDocument, /invalid DID document/],
    ];
    for (const [document, pattern] of cases) {
      await assert.rejects(() => resolveEd25519PublicKey(did, new StaticResolver(document)), (error: unknown) => {
        assert.ok(error instanceof DIDResolutionError, String(error));
        assert.match((error as Error).message, pattern);
        return true;
      });
    }
    const ok = new StaticResolver({
      id: did,
      publicKeys: [
        { id: "x", type: "X25519KeyAgreementKey2019", publicKeyBytes: new Uint8Array(32) },
        { id: "k", type: "Ed25519VerificationKey2018", publicKeyBytes: key },
      ],
    });
    assert.deepEqual(await resolveEd25519PublicKey(did, ok), key);
  });

  it("decodes did:key directly without a resolver and rejects other methods", async () => {
    const alice = Identity.generate();
    assert.deepEqual(await resolveEd25519PublicKey(alice.did), alice.publicKey);
    await assert.rejects(() => resolveEd25519PublicKey("did:example:alice"), DIDResolutionError);
  });
});
