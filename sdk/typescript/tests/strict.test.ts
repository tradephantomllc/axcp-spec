import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  Agent,
  ContextPatchType,
  Identity,
  InvalidEnvelopeError,
  decodeEnvelope,
  encodeEnvelope,
  type AxcpEnvelope,
} from "../src/index.js";
import { concatBytes, contextEnvelope, lengthDelimited, varint, varintField } from "./helpers.js";

interface SignedFixture {
  readonly alice: Agent;
  readonly bob: Agent;
  readonly envelope: AxcpEnvelope;
  readonly bytes: Uint8Array;
}

function signedFixture(): SignedFixture {
  const alice = new Agent(Identity.generate());
  const bob = new Agent(Identity.generate());
  const envelope = alice.signMessage(contextEnvelope(), { recipientDid: bob.identity.did });
  return { alice, bob, envelope, bytes: encodeEnvelope(envelope) };
}

/** Envelope bytes without field 4 (context_patch) so a custom field 4 can be appended. */
function withoutContextPatch(envelope: AxcpEnvelope): Uint8Array {
  const copy = { ...envelope };
  delete copy.contextPatch;
  return encodeEnvelope(copy);
}

describe("strict envelope decoding (D5)", () => {
  it("merges repeated singular sub-messages before checking nested oneofs", () => {
    const base = withoutContextPatch(contextEnvelope());
    // CapabilityMessage.kind: ack (3) in the first occurrence, offer (1) in the second.
    const ack = lengthDelimited(3, new Uint8Array());
    const offer = lengthDelimited(1, new Uint8Array());
    const sameKind = concatBytes(base, lengthDelimited(5, ack), lengthDelimited(5, ack));
    assert.doesNotThrow(() => decodeEnvelope(sameKind, { strict: true }));
    const mixedKinds = concatBytes(base, lengthDelimited(5, ack), lengthDelimited(5, offer));
    assert.throws(() => decodeEnvelope(mixedKinds, { strict: true }), /multiple members of oneof kind/);
  });


  it("accepts canonical envelopes", async () => {
    const { bob, bytes } = signedFixture();
    const verified = await bob.verifyBytes(bytes);
    assert.equal(verified.contextPatch?.contextId, "ctx");
  });

  it("rejects an unknown top-level field that the signature does not cover", async () => {
    const { bob, bytes } = signedFixture();
    const tampered = concatBytes(bytes, varintField(50, 1));
    // Lenient decoding drops the field and the signature still verifies...
    await new Agent(bob.identity).verify(decodeEnvelope(tampered));
    // ...the strict path rejects it.
    assert.throws(() => decodeEnvelope(tampered, { strict: true }), /unknown field 50/);
    await assert.rejects(() => bob.verifyBytes(tampered), InvalidEnvelopeError);
  });

  it("rejects an unknown field nested inside a known sub-message", async () => {
    const { bob, envelope } = signedFixture();
    const patch = ContextPatchType.encode(ContextPatchType.create(envelope.contextPatch!)).finish();
    const nested = concatBytes(patch, varintField(9, 7));
    const tampered = concatBytes(withoutContextPatch(envelope), lengthDelimited(4, nested));
    await new Agent(bob.identity).verify(decodeEnvelope(tampered));
    assert.throws(() => decodeEnvelope(tampered, { strict: true }), /unknown field 9 in ContextPatch/);
    await assert.rejects(() => bob.verifyBytes(tampered), InvalidEnvelopeError);
  });

  it("rejects the reserved CapabilityDescriptor field 8", () => {
    const descriptor = concatBytes(lengthDelimited(1, new TextEncoder().encode("tool")), varintField(8, 1));
    const offer = lengthDelimited(1, descriptor);
    const capability = lengthDelimited(1, offer);
    const bytes = concatBytes(varintField(1, 1), varintField(3, 1), lengthDelimited(5, capability));
    assert.throws(() => decodeEnvelope(bytes, { strict: true }), /unknown field 8 in CapabilityDescriptor/);
  });

  it("rejects two distinct payload members", async () => {
    const { bob, bytes } = signedFixture();
    const tampered = concatBytes(bytes, lengthDelimited(9, new Uint8Array()));
    assert.throws(() => decodeEnvelope(tampered, { strict: true }), /multiple members of oneof payload/);
    await assert.rejects(() => bob.verifyBytes(tampered), InvalidEnvelopeError);
  });

  it("rejects two distinct members of a nested oneof", () => {
    const capability = concatBytes(lengthDelimited(2, new Uint8Array()), lengthDelimited(3, new Uint8Array()));
    const bytes = concatBytes(varintField(1, 1), varintField(3, 1), lengthDelimited(5, capability));
    assert.throws(() => decodeEnvelope(bytes, { strict: true }), /multiple members of oneof kind/);
  });

  it("merges repeated occurrences of the same payload member like Go/Python", async () => {
    const { bob, envelope } = signedFixture();
    // Split the patch into two occurrences of field 4: context_id first, the rest second.
    const { contextId, ...rest } = envelope.contextPatch!;
    const first = ContextPatchType.encode(ContextPatchType.create({ contextId })).finish();
    const second = ContextPatchType.encode(ContextPatchType.create(rest)).finish();
    const split = concatBytes(withoutContextPatch(envelope), lengthDelimited(4, first), lengthDelimited(4, second));
    const verified = await bob.verifyBytes(split);
    assert.equal(verified.contextPatch?.contextId, "ctx");
    assert.equal(verified.contextPatch?.ops?.length, 1);
  });

  it("rejects envelopes without a payload member", () => {
    const bytes = concatBytes(varintField(1, 1), varintField(3, 1));
    assert.throws(() => decodeEnvelope(bytes, { strict: true }), /exactly one payload member/);
    decodeEnvelope(bytes);
  });

  it("rejects undefined enum values (after int32 truncation)", () => {
    const op = (value: number | bigint): Uint8Array => {
      const deltaOp = varintField(1, value);
      const patch = lengthDelimited(3, deltaOp);
      return concatBytes(varintField(1, 1), varintField(3, 1), lengthDelimited(4, patch));
    };
    decodeEnvelope(op(3), { strict: true });
    // 2^32 + 1 truncates to 1 (REPLACE) exactly like Go's int32 conversion.
    decodeEnvelope(op(2n ** 32n + 1n), { strict: true });
    assert.throws(() => decodeEnvelope(op(7), { strict: true }), /undefined enum value/);
    assert.throws(() => decodeEnvelope(op(2n ** 64n - 1n), { strict: true }), /undefined enum value/);
    // Lenient protobufjs decoding silently rewrites the unknown value to ADD (0), which is
    // why it would sign/verify different bytes than Go or Python.
    assert.equal(decodeEnvelope(op(7)).contextPatch?.ops?.[0]?.op, 0);
  });

  it("rejects invalid UTF-8 in string fields", () => {
    const badString = Uint8Array.from([0xc3, 0x28]);
    const patch = lengthDelimited(1, badString);
    const bytes = concatBytes(varintField(1, 1), varintField(3, 1), lengthDelimited(4, patch));
    assert.throws(() => decodeEnvelope(bytes, { strict: true }), /invalid UTF-8/);
    const surrogate = concatBytes(
      lengthDelimited(2, Uint8Array.from([0xed, 0xa0, 0x80])),
      varintField(1, 1),
      lengthDelimited(4, new Uint8Array()),
    );
    assert.throws(() => decodeEnvelope(surrogate, { strict: true }), /invalid UTF-8/);
    decodeEnvelope(bytes);
  });

  it("rejects wire-type mismatches and malformed varints", () => {
    const payload = lengthDelimited(4, new Uint8Array());
    // version (field 1) sent as length-delimited.
    assert.throws(
      () => decodeEnvelope(concatBytes(lengthDelimited(1, Uint8Array.of(1)), payload), { strict: true }),
      /wire type mismatch/,
    );
    // 11-byte varint.
    const tooLong = Uint8Array.from([0x08, ...new Array<number>(10).fill(0xff), 0x01]);
    assert.throws(() => decodeEnvelope(concatBytes(tooLong, payload), { strict: true }), InvalidEnvelopeError);
    // 10th byte > 1 overflows uint64.
    const overflow = Uint8Array.from([0x08, ...new Array<number>(9).fill(0xff), 0x02]);
    assert.throws(() => decodeEnvelope(concatBytes(overflow, payload), { strict: true }), /overflows/);
    // Field number 0.
    assert.throws(() => decodeEnvelope(concatBytes(Uint8Array.of(0x00, 0x01), payload), { strict: true }), /field number/);
    // Truncated length.
    assert.throws(
      () => decodeEnvelope(Uint8Array.from([0x22, ...varint(10), 1, 2]), { strict: true }),
      /exceeds message bounds/,
    );
  });
});
