import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  Agent,
  DEFAULT_PEER_TTL_MS,
  DeprecatedProfileError,
  Identity,
  InvalidEnvelopeError,
  InvalidReplayConfigError,
  NoProfileOverlapError,
  Profile,
  ReplayCapacityError,
  ReplayDetectedError,
  ReplayProtector,
  SequenceTooOldError,
  TimestampValidator,
  encodeEnvelope,
  negotiate,
  nowMicros,
  signEnvelope,
  toSafeNumber,
} from "../src/index.js";
import { contextEnvelope } from "./helpers.js";

const MINUTE = 60_000;

describe("profile negotiation", () => {
  it("negotiates secure baseline", () => {
    const result = negotiate();

    assert.equal(result.version, "1");
    assert.equal(result.profile, Profile.SECURE_BASELINE);
    assert.deepEqual(result.capabilities.sigAlgos, ["ed25519"]);
  });

  it("rejects transport-only by default", () => {
    assert.throws(
      () =>
        negotiate(
          {
            version: "1",
            profiles: [Profile.TRANSPORT_ONLY],
            capabilities: { requireAuth: false, requireReplay: false, sigAlgos: ["ed25519"] },
          },
          { serverProfiles: [Profile.TRANSPORT_ONLY] },
        ),
      DeprecatedProfileError,
    );
  });

  it("rejects missing profile overlap", () => {
    assert.throws(() => negotiate(undefined, { serverProfiles: [] }), NoProfileOverlapError);
  });
});

describe("replay protection (D6)", () => {
  const t0 = new Date("2026-01-01T00:00:00Z");
  const at = (ms: number): Date => new Date(t0.getTime() + ms);

  it("enforces the sliding window", () => {
    const replay = new ReplayProtector({ window: 5 });

    replay.checkAndMark("peer", 100, t0);
    replay.checkAndMark("peer", 96, t0);
    replay.checkAndMark("peer", 99, t0);

    assert.throws(() => replay.checkAndMark("peer", 96, t0), ReplayDetectedError);
    assert.throws(() => replay.checkAndMark("peer", 100, t0), ReplayDetectedError);
    assert.throws(() => replay.checkAndMark("peer", 95, t0), SequenceTooOldError);
    replay.checkAndMark("peer", 97, t0);
  });

  it("accepts any first sequence and advances across large gaps", () => {
    const replay = new ReplayProtector({ window: 64 });
    replay.checkAndMark("peer", 1_700_000_000_000_000, t0);
    replay.checkAndMark("peer", 1_700_000_000_000_100, t0);
    assert.throws(() => replay.checkAndMark("peer", 1_700_000_000_000_000, t0), SequenceTooOldError);
    replay.checkAndMark("peer", 1_700_000_000_000_099, t0);
    assert.throws(() => replay.checkAndMark("peer", 1_700_000_000_000_099, t0), ReplayDetectedError);
    // Bits for sequences skipped by an advance must be clear.
    replay.checkAndMark("peer", 1_700_000_000_000_098, t0);
  });

  it("keeps rejecting replays while the sender stays active beyond the old TTL", () => {
    const replay = new ReplayProtector({ window: 1000 });
    replay.checkAndMark("peer", 10, t0);
    // The peer keeps sending; seen sequences must never expire while it is active.
    for (let step = 1; step <= 6; step += 1) {
      replay.checkAndMark("peer", 10 + step, at(step * 8 * MINUTE));
    }
    assert.throws(() => replay.checkAndMark("peer", 10, at(48 * MINUTE)), ReplayDetectedError);
  });

  it("defaults the peer TTL to 10 minutes and evicts idle senders", () => {
    const replay = new ReplayProtector();
    assert.equal(replay.peerTtlMs, DEFAULT_PEER_TTL_MS);
    replay.checkAndMark("peer", 50, t0);
    assert.throws(() => replay.checkAndMark("peer", 50, at(10 * MINUTE)), ReplayDetectedError);
    // After more than the TTL of inactivity the state is gone (the timestamp check is
    // what rejects such old envelopes in a real verifier).
    replay.checkAndMark("peer", 50, at(20 * MINUTE + 1));
  });

  it("raises the peer TTL to maxAge + clockSkew (closes the +skew replay gap)", () => {
    const replay = new ReplayProtector({
      peerTtlMs: MINUTE,
      timestampPolicy: { maxAgeMs: 5 * MINUTE, clockSkewMs: 30_000 },
    });
    assert.equal(replay.peerTtlMs, 5 * MINUTE + 30_000);
    replay.checkAndMark("peer", 1, t0);
    // A message dated +25s is still timestamp-valid at T+5m10s: it must still be a replay.
    assert.throws(() => replay.checkAndMark("peer", 1, at(5 * MINUTE + 10_000)), ReplayDetectedError);
  });

  it("Agent raises the TTL of a caller-supplied protector", () => {
    const replay = new ReplayProtector({ peerTtlMs: 1_000 });
    const agent = new Agent(Identity.generate(), {
      replay,
      timestamps: new TimestampValidator({ maxAgeMs: 2 * MINUTE, clockSkewMs: 10_000 }),
    });
    assert.equal(agent.replay.peerTtlMs, 2 * MINUTE + 10_000);
  });

  it("fails closed when the sender capacity is exhausted", () => {
    const replay = new ReplayProtector({ maxPeers: 2, peerTtlMs: MINUTE });
    replay.checkAndMark("a", 1, t0);
    replay.checkAndMark("b", 1, t0);
    assert.throws(() => replay.checkAndMark("c", 1, at(1_000)), ReplayCapacityError);
    // Known senders keep working while full.
    replay.checkAndMark("a", 2, at(1_000));
    // Once "b" is idle beyond the TTL it can be evicted to make room.
    replay.checkAndMark("c", 1, at(MINUTE + 500));
    assert.equal(replay.trackedPeers, 2);
    assert.throws(() => replay.checkAndMark("d", 1, at(MINUTE + 600)), ReplayCapacityError);
  });

  it("evicts idle senders incrementally without a full scan", () => {
    const replay = new ReplayProtector({ peerTtlMs: MINUTE });
    for (let index = 0; index < 100; index += 1) {
      replay.checkAndMark(`peer-${index}`, 1, t0);
    }
    replay.checkAndMark("fresh", 1, at(2 * MINUTE));
    const afterOne = replay.trackedPeers;
    assert.ok(afterOne < 101 && afterOne > 1, `bounded batch eviction, got ${afterOne}`);
    for (let index = 0; index < 20; index += 1) {
      replay.checkAndMark("fresh", 2 + index, at(2 * MINUTE));
    }
    assert.equal(replay.trackedPeers, 1);
  });

  it("validates configuration and inputs", () => {
    assert.throws(() => new ReplayProtector({ window: 0 }), InvalidReplayConfigError);
    assert.throws(() => new ReplayProtector({ peerTtlMs: 0 }), InvalidReplayConfigError);
    assert.throws(() => new ReplayProtector({ maxPeers: 0 }), InvalidReplayConfigError);
    const replay = new ReplayProtector();
    assert.throws(() => replay.checkAndMark("", 1), InvalidReplayConfigError);
    assert.throws(() => replay.checkAndMark("peer", 0), InvalidReplayConfigError);
    assert.throws(() => replay.checkAndMark("peer", 2 ** 53), InvalidReplayConfigError);
  });

  it("cleans nonce entries after the TTL and bounds them", () => {
    const replay = new ReplayProtector({ peerTtlMs: 1_000, window: 5, maxNonces: 2 });

    replay.checkAndMarkNonce("peer", "nonce", t0);
    assert.throws(() => replay.checkAndMarkNonce("peer", "nonce", t0), ReplayDetectedError);
    replay.checkAndMarkNonce("peer", "nonce-2", t0);
    assert.throws(() => replay.checkAndMarkNonce("peer", "nonce-3", t0), ReplayCapacityError);
    replay.checkAndMarkNonce("peer", "nonce", at(2_000));
  });
});

describe("sequence allocation (D7)", () => {
  it("starts at the current Unix time in microseconds", () => {
    const before = nowMicros();
    const alice = new Agent(Identity.generate());
    const bob = Identity.generate();
    const first = toSafeNumber(alice.signMessage(contextEnvelope(), { recipientDid: bob.did }).sequence, "sequence");
    const after = nowMicros();
    assert.ok(first >= before && first <= after, `${before} <= ${first} <= ${after}`);
    const second = toSafeNumber(alice.signMessage(contextEnvelope(), { recipientDid: bob.did }).sequence, "sequence");
    assert.equal(second, first + 1);
  });

  it("honours an explicit initial sequence", () => {
    const alice = new Agent(Identity.generate(), { initialSequence: 42 });
    const envelope = alice.signMessage(contextEnvelope(), { recipientDid: Identity.generate().did });
    assert.equal(toSafeNumber(envelope.sequence, "sequence"), 42);
    assert.throws(() => new Agent(Identity.generate(), { initialSequence: 0 }), InvalidEnvelopeError);
  });

  it("never signs sequence 0 from the low-level helper", () => {
    const alice = Identity.generate();
    const bob = Identity.generate();
    assert.throws(() => signEnvelope(contextEnvelope(), alice, { recipientDid: bob.did }), /sequence/);
    assert.throws(() => signEnvelope(contextEnvelope(), alice, { recipientDid: bob.did, sequence: 0 }), /sequence/);
    const preset = contextEnvelope();
    preset.sequence = 9;
    assert.equal(toSafeNumber(signEnvelope(preset, alice, { recipientDid: bob.did }).sequence, "sequence"), 9);
  });

  it("keeps working for a verifier after the sender restarts", async () => {
    const identity = Identity.generate();
    const bob = new Agent(Identity.generate());
    const firstRun = new Agent(identity);
    for (let index = 0; index < 3; index += 1) {
      await bob.verifyBytes(encodeEnvelope(firstRun.signMessage(contextEnvelope(), { recipientDid: bob.identity.did })));
    }
    await new Promise((resolve) => setTimeout(resolve, 2));
    const restarted = new Agent(identity);
    await bob.verifyBytes(encodeEnvelope(restarted.signMessage(contextEnvelope(), { recipientDid: bob.identity.did })));

    // A counter that restarts at 1 (the old behaviour) is locked out.
    const legacy = new Agent(identity, { initialSequence: 1 });
    await assert.rejects(
      () => bob.verifyBytes(encodeEnvelope(legacy.signMessage(contextEnvelope(), { recipientDid: bob.identity.did }))),
      SequenceTooOldError,
    );
  });
});
