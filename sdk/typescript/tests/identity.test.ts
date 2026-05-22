import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  DID,
  Identity,
  InvalidDIDFormat,
  KeyMismatchError,
  SignatureVerificationError,
  didKeyFromPublicKey,
  publicKeyFromDidKey,
} from "../src/index.js";

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
    assert.deepEqual(restored.privateSeed, identity.privateSeed);
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

    assert.throws(() => Identity.fromSeed(left.privateSeed, right.did), KeyMismatchError);
  });

  it("allows non-key DID labels", () => {
    const identity = Identity.generate();
    const labeled = Identity.fromSeed(identity.privateSeed, "did:example:alice");

    assert.equal(labeled.did, "did:example:alice");
    assert.deepEqual(labeled.publicKey, identity.publicKey);
  });

  it("rejects invalid DID values", () => {
    for (const value of ["", "alice", "did::alice", "did:example:", "did:example:alice bob"]) {
      assert.throws(() => DID.parse(value), InvalidDIDFormat);
    }
  });
});
