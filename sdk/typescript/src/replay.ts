import {
  InvalidReplayConfigError,
  ReplayCapacityError,
  ReplayDetectedError,
  SequenceTooOldError,
} from "./errors.js";

/** Default sliding-window size (sequences remembered below the highest one). */
export const DEFAULT_REPLAY_WINDOW = 1000;
/** Default idle-sender eviction TTL (D6). */
export const DEFAULT_PEER_TTL_MS = 10 * 60 * 1000;
/** Default maximum number of tracked senders (D6). */
export const DEFAULT_MAX_PEERS = 100_000;
/** Default maximum number of tracked (peer, nonce) pairs. */
export const DEFAULT_MAX_NONCES = 1_000_000;

const MAX_WINDOW = 1 << 16;
const CLEANUP_BATCH = 16;

interface PeerState {
  highest: number;
  lastSeenMs: number;
  readonly bits: Uint32Array;
}

/** Timestamp policy used to enforce `peerTtl >= maxAge + clockSkew` (D6). */
export interface ReplayTimestampPolicy {
  readonly maxAgeMs: number;
  readonly clockSkewMs: number;
}

export interface ReplayProtectorOptions {
  /** Sliding window size W (default 1000). */
  readonly window?: number;
  /** Idle sender TTL in milliseconds (default 10 minutes). */
  readonly peerTtlMs?: number;
  /** @deprecated Alias of `peerTtlMs`. */
  readonly ttlMs?: number;
  /** Maximum number of tracked senders (default 100000); new senders fail closed when full. */
  readonly maxPeers?: number;
  /** Maximum number of tracked nonces (default 1000000); new nonces fail closed when full. */
  readonly maxNonces?: number;
  /** When set, the peer TTL is raised to at least `maxAgeMs + clockSkewMs`. */
  readonly timestampPolicy?: ReplayTimestampPolicy;
}

/**
 * Per-sender replay protection with a bitmap sliding window (D6).
 *
 * - first message from a sender is accepted and becomes the highest sequence;
 * - a higher sequence advances the window;
 * - a sequence inside the window is rejected if already seen;
 * - a sequence at or below `highest - window` is rejected as too old.
 *
 * Seen sequences never expire while a sender is active. Idle senders are evicted after
 * the peer TTL (amortized, oldest-first). When `maxPeers` senders are tracked and none is
 * idle, new senders are rejected with {@link ReplayCapacityError}.
 *
 * Only call {@link ReplayProtector.checkAndMark} after the envelope signature has been
 * verified, so unauthenticated traffic cannot consume sequence numbers.
 */
export class ReplayProtector {
  readonly window: number;
  readonly maxPeers: number;
  readonly maxNonces: number;
  private peerTtlMsValue: number;
  // Insertion order == last-activity order: touched peers are re-inserted at the end.
  private readonly peers = new Map<string, PeerState>();
  // Insertion order == first-seen order (entries are never refreshed).
  private readonly nonces = new Map<string, number>();
  private readonly words: number;

  constructor(options: ReplayProtectorOptions = {}) {
    this.window = options.window ?? DEFAULT_REPLAY_WINDOW;
    this.peerTtlMsValue = options.peerTtlMs ?? options.ttlMs ?? DEFAULT_PEER_TTL_MS;
    this.maxPeers = options.maxPeers ?? DEFAULT_MAX_PEERS;
    this.maxNonces = options.maxNonces ?? DEFAULT_MAX_NONCES;
    if (!Number.isSafeInteger(this.window) || this.window < 1 || this.window > MAX_WINDOW) {
      throw new InvalidReplayConfigError(`window must be an integer between 1 and ${MAX_WINDOW}`);
    }
    if (!Number.isSafeInteger(this.peerTtlMsValue) || this.peerTtlMsValue <= 0) {
      throw new InvalidReplayConfigError("peerTtlMs must be a positive safe integer");
    }
    if (!Number.isSafeInteger(this.maxPeers) || this.maxPeers < 1) {
      throw new InvalidReplayConfigError("maxPeers must be a positive safe integer");
    }
    if (!Number.isSafeInteger(this.maxNonces) || this.maxNonces < 1) {
      throw new InvalidReplayConfigError("maxNonces must be a positive safe integer");
    }
    this.words = Math.ceil(this.window / 32);
    if (options.timestampPolicy !== undefined) {
      this.applyTimestampPolicy(options.timestampPolicy);
    }
  }

  /** Effective idle-sender TTL in milliseconds. */
  get peerTtlMs(): number {
    return this.peerTtlMsValue;
  }

  /** Number of senders currently tracked. */
  get trackedPeers(): number {
    return this.peers.size;
  }

  /** Raise the peer TTL to at least `maxAgeMs + clockSkewMs` (never lowers it). */
  applyTimestampPolicy(policy: ReplayTimestampPolicy): void {
    const minimum = policy.maxAgeMs + policy.clockSkewMs;
    if (!Number.isSafeInteger(minimum) || minimum <= 0) {
      throw new InvalidReplayConfigError("timestamp policy must have a positive max age");
    }
    if (this.peerTtlMsValue < minimum) {
      this.peerTtlMsValue = minimum;
    }
  }

  checkAndMark(peerId: string, sequence: number, now: Date = new Date()): void {
    if (typeof peerId !== "string" || peerId.length === 0) {
      throw new InvalidReplayConfigError("peerId cannot be empty");
    }
    if (!Number.isSafeInteger(sequence) || sequence < 1) {
      throw new InvalidReplayConfigError("sequence must be a positive safe integer");
    }

    const currentMs = now.getTime();
    this.evictIdlePeers(currentMs, CLEANUP_BATCH);

    let state = this.peers.get(peerId);
    if (state !== undefined && currentMs - state.lastSeenMs > this.peerTtlMsValue) {
      this.peers.delete(peerId);
      state = undefined;
    }

    if (state === undefined) {
      if (this.peers.size >= this.maxPeers) {
        this.evictIdlePeers(currentMs, Number.POSITIVE_INFINITY);
        if (this.peers.size >= this.maxPeers) {
          throw new ReplayCapacityError("replay capacity exceeded");
        }
      }
      const created: PeerState = { highest: sequence, lastSeenMs: currentMs, bits: new Uint32Array(this.words) };
      this.setBit(created, sequence);
      this.peers.set(peerId, created);
      return;
    }

    if (sequence > state.highest) {
      const shift = sequence - state.highest;
      if (shift >= this.window) {
        state.bits.fill(0);
      } else {
        for (let cleared = state.highest + 1; cleared < sequence; cleared += 1) {
          this.clearBit(state, cleared);
        }
      }
      state.highest = sequence;
      this.setBit(state, sequence);
    } else if (sequence > state.highest - this.window) {
      if (this.testBit(state, sequence)) {
        throw new ReplayDetectedError("sequence already seen");
      }
      this.setBit(state, sequence);
    } else {
      throw new SequenceTooOldError("sequence outside replay window");
    }

    state.lastSeenMs = Math.max(state.lastSeenMs, currentMs);
    this.peers.delete(peerId);
    this.peers.set(peerId, state);
  }

  checkAndMarkNonce(peerId: string, nonce: string, now: Date = new Date()): void {
    if (typeof peerId !== "string" || peerId.length === 0) {
      throw new InvalidReplayConfigError("peerId cannot be empty");
    }
    if (typeof nonce !== "string" || nonce.length === 0) {
      throw new InvalidReplayConfigError("nonce cannot be empty");
    }

    const currentMs = now.getTime();
    this.evictExpiredNonces(currentMs, CLEANUP_BATCH);
    const key = `${peerId.length}:${peerId}:${nonce}`;
    const seenAt = this.nonces.get(key);
    if (seenAt !== undefined) {
      if (currentMs - seenAt <= this.peerTtlMsValue) {
        throw new ReplayDetectedError("nonce already seen");
      }
      this.nonces.delete(key);
    }
    if (this.nonces.size >= this.maxNonces) {
      this.evictExpiredNonces(currentMs, Number.POSITIVE_INFINITY);
      if (this.nonces.size >= this.maxNonces) {
        throw new ReplayCapacityError("nonce replay capacity exceeded");
      }
    }
    this.nonces.set(key, currentMs);
  }

  private evictIdlePeers(currentMs: number, limit: number): void {
    let evicted = 0;
    for (const [peerId, state] of this.peers) {
      if (evicted >= limit || currentMs - state.lastSeenMs <= this.peerTtlMsValue) {
        return;
      }
      this.peers.delete(peerId);
      evicted += 1;
    }
  }

  private evictExpiredNonces(currentMs: number, limit: number): void {
    let evicted = 0;
    for (const [key, seenAt] of this.nonces) {
      if (evicted >= limit || currentMs - seenAt <= this.peerTtlMsValue) {
        return;
      }
      this.nonces.delete(key);
      evicted += 1;
    }
  }

  private position(sequence: number): number {
    return sequence % this.window;
  }

  private setBit(state: PeerState, sequence: number): void {
    const position = this.position(sequence);
    state.bits[position >>> 5] = state.bits[position >>> 5]! | (1 << (position & 31));
  }

  private clearBit(state: PeerState, sequence: number): void {
    const position = this.position(sequence);
    state.bits[position >>> 5] = state.bits[position >>> 5]! & ~(1 << (position & 31));
  }

  private testBit(state: PeerState, sequence: number): boolean {
    const position = this.position(sequence);
    return (state.bits[position >>> 5]! & (1 << (position & 31))) !== 0;
  }
}
