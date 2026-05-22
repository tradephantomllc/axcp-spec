import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  Agent,
  DID_AUTH_TRANSCRIPT_VERSION,
  DeltaOpType,
  Identity,
  InMemoryDIDResolver,
  InvalidEnvelopeError,
  ReplayDetectedError,
  SignatureVerificationError,
  TimestampExpiredError,
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

const VECTOR_URL = new URL("../../../../testdata/sdk/secure_baseline_vector.json", import.meta.url);

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

function contextEnvelope(): AxcpEnvelope {
  const envelope = newEnvelope({ traceId: "trace-123", profile: 1 });
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
  it("excludes auth fields from signing payload", () => {
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
    assert.equal(toSafeNumber(decoded.sequence, "sequence"), 0);
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
    right.sequence = 2;
    right.signature = new TextEncoder().encode("right");

    assert.deepEqual(signingPayload(left), signingPayload(right));
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
    const envelope = contextEnvelope();

    alice.signMessage(envelope);
    await verifyEnvelope(envelope, { publicKey: alice.identity.publicKey });
  });

  it("rejects explicit key mismatch for did:key", async () => {
    const alice = new Agent(Identity.generate());
    const other = Identity.generate();
    const envelope = contextEnvelope();

    alice.signMessage(envelope);
    await assert.rejects(() => verifyEnvelope(envelope, { publicKey: other.publicKey }), InvalidEnvelopeError);
  });

  it("rejects wrong recipients", async () => {
    const alice = new Agent(Identity.generate());
    const bob = new Agent(Identity.generate());
    const carol = new Agent(Identity.generate());
    const envelope = contextEnvelope();

    alice.signMessage(envelope, { recipientDid: carol.identity.did });

    await assert.rejects(() => bob.verify(envelope), InvalidEnvelopeError);
  });

  it("verifies non-key senders through a resolver", async () => {
    const seedIdentity = Identity.generate();
    const aliceIdentity = Identity.fromSeed(seedIdentity.privateSeed, "did:example:alice");
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
});
