import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  DeprecatedProfileError,
  NoProfileOverlapError,
  Profile,
  ReplayDetectedError,
  ReplayProtector,
  SequenceTooOldError,
  negotiate,
} from "../src/index.js";

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

describe("replay protection", () => {
  it("enforces sequence windows", () => {
    const replay = new ReplayProtector({ ttlMs: 60_000, window: 5 });
    const now = new Date("2026-01-01T00:00:00Z");

    replay.checkAndMark("peer", 100, now);
    replay.checkAndMark("peer", 96, now);

    assert.throws(() => replay.checkAndMark("peer", 96, now), ReplayDetectedError);
    assert.throws(() => replay.checkAndMark("peer", 95, now), SequenceTooOldError);
  });

  it("cleans nonce entries after ttl", () => {
    const replay = new ReplayProtector({ ttlMs: 1_000, window: 5 });
    const now = new Date("2026-01-01T00:00:00Z");

    replay.checkAndMarkNonce("peer", "nonce", now);
    assert.throws(() => replay.checkAndMarkNonce("peer", "nonce", now), ReplayDetectedError);
    replay.checkAndMarkNonce("peer", "nonce", new Date(now.getTime() + 2_000));
  });
});
