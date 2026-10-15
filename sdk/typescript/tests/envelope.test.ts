import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { performance } from "node:perf_hooks";
import Long from "long";
import {
  AXCPError,
  Agent,
  DID_AUTH_TRANSCRIPT_VERSION,
  DeltaOpType,
  Identity,
  InMemoryDIDResolver,
  InvalidDIDFormat,
  InvalidEnvelopeError,
  ReplayDetectedError,
  SignatureMissingError,
  SignatureVerificationError,
  TimestampExpiredError,
  TimestampInvalidError,
  buildAuthTranscript,
  decodeEnvelope,
  encodeEnvelope,
  newEnvelope,
  signEnvelope,
  signingPayload,
  toSafeNumber,
  verifyEnvelope,
  type AxcpEnvelope,
} from "../src/index.js";
import { contextEnvelope, rawSign } from "./helpers.js";

const VECTOR_URL = new URL("../../../../testdata/sdk/secure_baseline_vector.json", import.meta.url);
const REPEATED_VECTOR_URL = new URL("../../../../testdata/sdk/repeated_defaults_vector.json", import.meta.url);

interface SecureBaselineVector {
  readonly trace_id: string;
  readonly profile: number;
  readonly recipient_did: string;
  readonly timestamp_ms: number;
  readonly sequence: number;
  readonly identity: {
    readonly private_seed_hex: string;
    readonly did: string;
    readonly public_key_b64: string;
  };
  readonly context_patch: {
    readonly context_id: string;
    readonly base_version: number;
    readonly ops: Array<{
      readonly op: keyof typeof DeltaOpType;
      readonly path: string;
      readonly data_b64: string;
      readonly ts: number;
    }>;
  };
  readonly expected: {
    readonly signing_payload_b64: string;
    readonly auth_transcript_b64: string;
    readonly signature_b64: string;
    readonly envelope_b64: string;
  };
}

function vectorEnvelope(vector: SecureBaselineVector): AxcpEnvelope {
  const envelope = newEnvelope({ traceId: vector.trace_id, profile: vector.profile });
  envelope.contextPatch = {
    contextId: vector.context_patch.context_id,
    baseVersion: vector.context_patch.base_version,
    ops: vector.context_patch.ops.map((op) => ({
      op: DeltaOpType[op.op],
      path: op.path,
      data: Buffer.from(op.data_b64, "base64"),
      ts: op.ts,
    })),
  };
  return envelope;
}

function loadVector(): SecureBaselineVector {
  return JSON.parse(readFileSync(VECTOR_URL, "utf8")) as SecureBaselineVector;
}

describe("envelope", () => {
  it("excludes detached auth fields from signing payload", () => {
    const envelope = contextEnvelope();
    envelope.senderDid = "did:key:sender";
    envelope.recipientDid = "did:key:recipient";
    envelope.timestampMs = 1_700_000_000_000;
    envelope.sequence = 42;
    envelope.signature = new TextEncoder().encode("signature");
    envelope.attestationProof = new TextEncoder().encode("attestation");

    const payload = signingPayload(envelope);
    const decoded = decodeEnvelope(payload);

    assert.equal(decoded.senderDid, "");
    assert.equal(decoded.recipientDid, "");
    assert.equal(toSafeNumber(decoded.timestampMs, "timestampMs"), 0);
    assert.equal(toSafeNumber(decoded.sequence, "sequence"), 42);
    assert.equal((decoded.signature ?? new Uint8Array()).length, 0);
    assert.equal((decoded.attestationProof ?? new Uint8Array()).length, 0);
    assert.equal(decoded.contextPatch?.contextId, "ctx");
  });

  it("keeps signing payload stable when auth fields change", () => {
    const left = contextEnvelope();
    left.senderDid = "did:key:left";
    left.recipientDid = "did:key:recipient";
    left.timestampMs = 1_700_000_000_000;
    left.sequence = 1;
    left.signature = new TextEncoder().encode("left");

    const right = contextEnvelope();
    right.senderDid = "did:key:right";
    right.recipientDid = "did:key:other";
    right.timestampMs = 1_700_000_001_000;
    right.sequence = 1;
    right.signature = new TextEncoder().encode("right");

    assert.deepEqual(signingPayload(left), signingPayload(right));
  });

  it("changes signing payload when sequence changes", () => {
    const left = contextEnvelope();
    left.sequence = 1;

    const right = contextEnvelope();
    right.sequence = 2;

    assert.notDeepEqual(signingPayload(left), signingPayload(right));
  });

  it("builds the Go/Python DID auth transcript shape", () => {
    const envelope = contextEnvelope();
    envelope.senderDid = "did:key:sender";
    envelope.recipientDid = "did:key:recipient";
    envelope.timestampMs = 1_700_000_000_123;

    const transcript = new TextDecoder().decode(buildAuthTranscript(envelope));
    const lines = transcript.split("\n");

    assert.equal(lines[0], DID_AUTH_TRANSCRIPT_VERSION);
    assert.equal(lines[1], envelope.senderDid);
    assert.equal(lines[2], envelope.recipientDid);
    assert.deepEqual(Buffer.from(lines[3] ?? "", "base64"), Buffer.from(signingPayload(envelope)));
    assert.equal(lines[4], "2023-11-14T22:13:20Z");
  });

  it("signs, verifies, encodes, and decodes an envelope", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did });
    const decoded = decodeEnvelope(encodeEnvelope(envelope));

    await bob.verify(decoded);
    assert.equal(decoded.senderDid, alice.identity.did);
    assert.equal(decoded.recipientDid, bob.identity.did);
  });

  it("rejects replay after a valid signature", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did });
    await bob.verify(envelope);
    await assert.rejects(() => bob.verify(envelope), ReplayDetectedError);
  });

  it("rejects tampered payloads", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did });
    envelope.contextPatch!.contextId = "tampered";

    await assert.rejects(() => bob.verify(envelope), SignatureVerificationError);
  });

  it("rejects tampered sequences", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did });
    envelope.sequence = toSafeNumber(envelope.sequence, "sequence") + 1;

    await assert.rejects(() => bob.verify(envelope), SignatureVerificationError);
  });

  it("enforces timestamp expiry after signature verification", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const oldMs = Date.now() - 10 * 60 * 1000;
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did, timestampMs: oldMs });

    await assert.rejects(() => bob.verify(envelope), TimestampExpiredError);
  });

  it("allows explicit public key verification", async () => {
    const alice = new Agent(Identity.generate());
    const bob = Identity.generate();
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.did });
    await verifyEnvelope(envelope, { expectedRecipientDid: bob.did, publicKey: alice.identity.publicKey });
  });

  it("rejects explicit key mismatch for did:key", async () => {
    const alice = new Agent(Identity.generate());
    const bob = Identity.generate();
    const other = Identity.generate();
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.did });
    await assert.rejects(
      () => verifyEnvelope(envelope, { expectedRecipientDid: bob.did, publicKey: other.publicKey }),
      InvalidEnvelopeError,
    );
  });

  it("verifies non-key senders through a resolver", async () => {
    const seedIdentity = Identity.generate();
    const aliceIdentity = Identity.fromSeed(seedIdentity.exportPrivateSeed(), "did:example:alice");
    const resolver = new InMemoryDIDResolver();
    resolver.register(aliceIdentity);

    const alice = new Agent(aliceIdentity);
    const bob = new Agent(Identity.generate(), { resolver });
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did });
    await bob.verify(envelope);
  });

  it("matches the shared Secure Baseline canonical signature vector", () => {
    const vector = loadVector();
    const identity = Identity.fromSeed(Buffer.from(vector.identity.private_seed_hex, "hex"));
    const envelope = vectorEnvelope(vector);

    signEnvelope(envelope, identity, {
      recipientDid: vector.recipient_did,
      sequence: vector.sequence,
      timestampMs: vector.timestamp_ms,
    });

    assert.equal(identity.did, vector.identity.did);
    assert.deepEqual(
      Buffer.from(identity.publicKey),
      Buffer.from(vector.identity.public_key_b64, "base64"),
    );
    assert.deepEqual(
      Buffer.from(signingPayload(envelope)),
      Buffer.from(vector.expected.signing_payload_b64, "base64"),
    );
    assert.deepEqual(
      Buffer.from(buildAuthTranscript(envelope)),
      Buffer.from(vector.expected.auth_transcript_b64, "base64"),
    );
    assert.deepEqual(
      Buffer.from(envelope.signature ?? new Uint8Array()),
      Buffer.from(vector.expected.signature_b64, "base64"),
    );
    assert.deepEqual(
      Buffer.from(encodeEnvelope(envelope)),
      Buffer.from(vector.expected.envelope_b64, "base64"),
    );
  });

  it("strictly decodes and verifies the Go-produced Secure Baseline envelope bytes", async () => {
    const vector = loadVector();
    const decoded = decodeEnvelope(Buffer.from(vector.expected.envelope_b64, "base64"), { strict: true });
    await verifyEnvelope(decoded, { expectedRecipientDid: vector.recipient_did });
    assert.equal(decoded.senderDid, vector.identity.did);
    await assert.rejects(
      () => verifyEnvelope(decoded, { expectedRecipientDid: "did:example:carol" }),
      InvalidEnvelopeError,
    );
  });
});

describe("repeated scalar presence", () => {
  it("preserves empty strings and matches the Python/Go wire vector", async () => {
    const vector = JSON.parse(readFileSync(REPEATED_VECTOR_URL, "utf8"));
    const env = newEnvelope({ traceId: "repeated-defaults", profile: 1 });
    env.capabilityMsg = { request: { ids: vector.ids } };
    signEnvelope(env, Identity.fromSeed(Buffer.from(vector.identity.private_seed_hex, "hex")), {
      recipientDid: vector.recipient_did, sequence: vector.sequence, timestampMs: vector.timestamp_ms,
    });
    assert.deepEqual(decodeEnvelope(encodeEnvelope(env)).capabilityMsg?.request?.ids, ["", "read", ""]);
    assert.equal(Buffer.from(encodeEnvelope(env)).toString("base64"), vector.expected.envelope_b64);
    assert.equal(Buffer.from(signingPayload(env)).toString("base64"), vector.expected.signing_payload_b64);
    assert.equal(Buffer.from(env.signature!).toString("base64"), vector.expected.signature_b64);
    const goBytes = Buffer.from(vector.expected.envelope_b64, "base64");
    await verifyEnvelope(decodeEnvelope(goBytes, { strict: true }), { expectedRecipientDid: vector.recipient_did });
  });
  it("rejects null repeated values rather than removing them", () => {
    const env = newEnvelope();
    env.capabilityMsg = { request: { ids: [null as unknown as string] } };
    assert.throws(() => encodeEnvelope(env), InvalidEnvelopeError);
  });
});

describe("recipient binding (D4)", () => {
  it("requires a recipient when signing", () => {
    const alice = new Agent(Identity.generate());
    assert.throws(() => alice.signMessage(contextEnvelope(), { recipientDid: "" }), InvalidEnvelopeError);
    assert.throws(
      () => alice.signMessage(contextEnvelope(), undefined as unknown as { recipientDid: string }),
      InvalidEnvelopeError,
    );
    assert.throws(
      () => signEnvelope(contextEnvelope(), alice.identity, {} as unknown as { recipientDid: string }),
      InvalidEnvelopeError,
    );
    assert.throws(
      () => signEnvelope(contextEnvelope(), alice.identity, { recipientDid: "not-a-did", sequence: 1 }),
      InvalidDIDFormat,
    );
  });

  it("rejects an otherwise valid envelope with an empty recipient", async () => {
    const alice = Identity.generate();
    const bob = new Agent(Identity.generate());
    const envelope = rawSign(contextEnvelope(), alice, { sequence: 5 });
    // The signature itself is valid for the empty recipient...
    alice.verify(buildAuthTranscript(envelope), envelope.signature!);
    // ...but the verifier requires recipient_did == its own DID.
    await assert.rejects(() => bob.verify(envelope), /recipient_did is required/);
  });

  it("rejects envelopes addressed to another agent (no cross-recipient replay)", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const carol = new Agent(Identity.generate());
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: bob.identity.did });
    await bob.verify(envelope);
    await assert.rejects(() => carol.verify(envelope), /does not match the verifier/);
  });

  it("requires the verifier DID in the low-level API", async () => {
    const alice = new Agent(Identity.generate());
    const bob = Identity.generate();
    const envelope = alice.signMessage(contextEnvelope(), { recipientDid: bob.did });
    await assert.rejects(
      () => verifyEnvelope(envelope, {} as unknown as { expectedRecipientDid: string }),
      InvalidEnvelopeError,
    );
  });
});

describe("envelope verification rules (D5)", () => {
  async function expectRejected(envelope: AxcpEnvelope, recipient: Identity, pattern: RegExp): Promise<void> {
    const verifier = new Agent(recipient);
    await assert.rejects(() => verifier.verify(envelope), (error: unknown) => {
      assert.ok(error instanceof AXCPError, `expected SDK error, got ${String(error)}`);
      assert.match((error as Error).message, pattern);
      return true;
    });
  }

  it("rejects versions other than 1", async () => {
    const alice = Identity.generate();
    const bob = Identity.generate();
    const envelope = contextEnvelope();
    envelope.version = 2;
    signEnvelope(envelope, alice, { recipientDid: bob.did, sequence: 1 });
    await expectRejected(envelope, bob, /unsupported envelope version/);
  });

  for (const profile of [0, 2, 3]) {
    it(`rejects profile ${profile}`, async () => {
      const alice = Identity.generate();
      const bob = Identity.generate();
      const envelope = newEnvelope({ traceId: "profile", profile });
      envelope.contextPatch = { contextId: "ctx" };
      signEnvelope(envelope, alice, { recipientDid: bob.did, sequence: 1 });
      await expectRejected(envelope, bob, /profile must be secure-baseline-v1/);
    });
  }

  it("rejects envelopes without a payload", async () => {
    const alice = Identity.generate();
    const bob = Identity.generate();
    const envelope = newEnvelope({ traceId: "empty" });
    signEnvelope(envelope, alice, { recipientDid: bob.did, sequence: 1 });
    await expectRejected(envelope, bob, /exactly one payload/);
  });

  it("rejects sequence 0", async () => {
    const alice = Identity.generate();
    const bob = Identity.generate();
    const envelope = rawSign(contextEnvelope(), alice, { recipientDid: bob.did, sequence: 0 });
    await expectRejected(envelope, bob, /sequence must be greater than zero/);
  });

  it("rejects signatures that are not 64 bytes", async () => {
    const alice = Identity.generate();
    const bob = Identity.generate();
    const envelope = signEnvelope(contextEnvelope(), alice, { recipientDid: bob.did, sequence: 1 });
    envelope.signature = envelope.signature!.subarray(0, 63);
    await assert.rejects(() => new Agent(bob).verify(envelope), InvalidEnvelopeError);
    envelope.signature = new Uint8Array();
    await assert.rejects(() => new Agent(bob).verify(envelope), SignatureMissingError);
  });

  it("rejects an invalid sender DID before any key resolution", async () => {
    const bob = Identity.generate();
    const envelope = contextEnvelope();
    envelope.senderDid = "did:key:z6Mk/../path";
    envelope.recipientDid = bob.did;
    envelope.sequence = 1;
    envelope.timestampMs = Date.now();
    envelope.signature = new Uint8Array(64);
    await assert.rejects(() => new Agent(bob).verify(envelope), InvalidDIDFormat);
  });

  it("rejects a 10 MB sender DID in well under 50 ms", async () => {
    const bob = Identity.generate();
    const envelope = contextEnvelope();
    envelope.senderDid = `did:key:z${"6".repeat(10 * 1024 * 1024)}`;
    envelope.recipientDid = bob.did;
    envelope.sequence = 1;
    envelope.timestampMs = Date.now();
    envelope.signature = new Uint8Array(64);
    const agent = new Agent(bob);
    const started = performance.now();
    await assert.rejects(() => agent.verify(envelope), InvalidDIDFormat);
    assert.ok(performance.now() - started < 50, "long DID must be rejected before decoding");
  });
});

describe("uint64 and timestamp error hygiene (D8/D10)", () => {
  it("rejects uint64 values above 2^53 with an SDK error", () => {
    const huge = Long.fromString("18446744073709551615", true);
    assert.throws(() => toSafeNumber(huge, "sequence"), InvalidEnvelopeError);
    assert.throws(() => toSafeNumber(2 ** 53, "sequence"), InvalidEnvelopeError);
    assert.throws(() => toSafeNumber("9007199254740993", "sequence"), InvalidEnvelopeError);
    assert.equal(toSafeNumber(Long.fromNumber(42, true), "sequence"), 42);
  });

  it("rejects huge wire sequences and timestamps with SDK errors", async () => {
    const alice = Identity.generate();
    const bob = new Agent(Identity.generate());
    const base = contextEnvelope();
    base.senderDid = alice.did;
    base.recipientDid = bob.identity.did;
    base.signature = new Uint8Array(64);

    base.sequence = Long.fromString("18446744073709551615", true);
    base.timestampMs = Date.now();
    await assert.rejects(() => bob.verifyBytes(encodeEnvelope(base)), InvalidEnvelopeError);

    base.sequence = 1;
    base.timestampMs = Long.fromString("9223372036854775808", true);
    await assert.rejects(() => bob.verifyBytes(encodeEnvelope(base)), InvalidEnvelopeError);

    base.timestampMs = 253_402_300_800_000;
    await assert.rejects(() => bob.verifyBytes(encodeEnvelope(base)), TimestampInvalidError);
  });
});
