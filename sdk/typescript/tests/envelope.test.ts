import { describe, it } from "node:test";
import assert from "node:assert/strict";
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

  it("matches the Python SDK canonical signature vector", () => {
    const identity = Identity.fromSeed(Uint8Array.from([...Array(32).keys()]));
    const envelope = contextEnvelope();
    envelope.traceId = "trace-vector";

    signEnvelope(envelope, identity, {
      recipientDid: "did:example:bob",
      sequence: 42,
      timestampMs: 1_700_000_000_123,
    });

    assert.equal(identity.did, "did:key:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd");
    assert.deepEqual(
      Buffer.from(identity.publicKey),
      Buffer.from("A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=", "base64"),
    );
    assert.deepEqual(
      Buffer.from(signingPayload(envelope)),
      Buffer.from("CAESDHRyYWNlLXZlY3RvchgBIhIKA2N0eBAHGgkSAi94GgExIAE=", "base64"),
    );
    assert.deepEqual(
      Buffer.from(buildAuthTranscript(envelope)),
      Buffer.from(
        "QVhDUC1ESUQtQVVUSC12MQpkaWQ6a2V5Ono2TWtlaFJnZjd5SmJnYUdmWXNkb0FzS2RCUEUzZGoyQ1lob3dRZGNqcVNKZ3ZWZApkaWQ6ZXhhbXBsZTpib2IKQ0FFU0RIUnlZV05sTFhabFkzUnZjaGdCSWhJS0EyTjBlQkFIR2drU0FpOTRHZ0V4SUFFPQoyMDIzLTExLTE0VDIyOjEzOjIwWg==",
        "base64",
      ),
    );
    assert.deepEqual(
      Buffer.from(envelope.signature ?? new Uint8Array()),
      Buffer.from(
        "PJQax44TKAkSXgLmHviGE3ntoFqQ9h00Lk+NfGvq35uKjPVc9wV9BLRg6ByvhIHDYbaddAsSkcTtjcpm9xlsAg==",
        "base64",
      ),
    );
    assert.deepEqual(
      Buffer.from(encodeEnvelope(envelope)),
      Buffer.from(
        "CAESDHRyYWNlLXZlY3RvchgBIhIKA2N0eBAHGgkSAi94GgExIAGiAThkaWQ6a2V5Ono2TWtlaFJnZjd5SmJnYUdmWXNkb0FzS2RCUEUzZGoyQ1lob3dRZGNqcVNKZ3ZWZKgB+9CV/7wxsAEqugEPZGlkOmV4YW1wbGU6Ym9iogZAPJQax44TKAkSXgLmHviGE3ntoFqQ9h00Lk+NfGvq35uKjPVc9wV9BLRg6ByvhIHDYbaddAsSkcTtjcpm9xlsAg==",
        "base64",
      ),
    );
  });
});
