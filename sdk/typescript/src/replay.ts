import {
  InvalidReplayConfigError,
  ReplayDetectedError,
  SequenceTooOldError,
} from "./errors.js";

interface PeerWindow {
  highest: number;
  seen: Map<number, number>;
}

export interface ReplayProtectorOptions {
  readonly ttlMs?: number;
  readonly window?: number;
}

export class ReplayProtector {
  private readonly ttlMs: number;
  private readonly window: number;
  private readonly peers = new Map<string, PeerWindow>();
  private readonly nonces = new Map<string, Map<string, number>>();

  constructor(options: ReplayProtectorOptions = {}) {
    this.ttlMs = options.ttlMs ?? 5 * 60 * 1000;
    this.window = options.window ?? 64;
    if (this.ttlMs <= 0) {
      throw new InvalidReplayConfigError("ttlMs must be positive");
    }
    if (this.window < 1) {
      throw new InvalidReplayConfigError("window must be at least 1");
    }
  }

  checkAndMark(peerId: string, sequence: number, now: Date = new Date()): void {
    if (!peerId) {
      throw new InvalidReplayConfigError("peerId cannot be empty");
    }
    if (!Number.isSafeInteger(sequence) || sequence < 0) {
      throw new InvalidReplayConfigError("sequence must be a non-negative safe integer");
    }

    const currentMs = now.getTime();
    const window = this.getPeerWindow(peerId);
    this.cleanupSequences(window, currentMs);

    if (window.seen.has(sequence)) {
      throw new ReplayDetectedError("sequence already seen");
    }

    if (window.highest > 0) {
      const minAcceptable = window.highest >= this.window ? window.highest - this.window + 1 : 0;
      if (sequence < minAcceptable) {
        throw new SequenceTooOldError("sequence outside replay window");
      }
    }

    window.seen.set(sequence, currentMs);
    if (sequence > window.highest) {
      window.highest = sequence;
    }
  }

  checkAndMarkNonce(peerId: string, nonce: string, now: Date = new Date()): void {
    if (!peerId) {
      throw new InvalidReplayConfigError("peerId cannot be empty");
    }
    if (!nonce) {
      throw new InvalidReplayConfigError("nonce cannot be empty");
    }

    const currentMs = now.getTime();
    const peerNonces = this.getPeerNonces(peerId);
    this.cleanupNonces(peerNonces, currentMs);
    if (peerNonces.has(nonce)) {
      throw new ReplayDetectedError("nonce already seen");
    }
    peerNonces.set(nonce, currentMs);
  }

  private getPeerWindow(peerId: string): PeerWindow {
    const existing = this.peers.get(peerId);
    if (existing !== undefined) {
      return existing;
    }
    const created = { highest: 0, seen: new Map<number, number>() };
    this.peers.set(peerId, created);
    return created;
  }

  private getPeerNonces(peerId: string): Map<string, number> {
    const existing = this.nonces.get(peerId);
    if (existing !== undefined) {
      return existing;
    }
    const created = new Map<string, number>();
    this.nonces.set(peerId, created);
    return created;
  }

  private cleanupSequences(window: PeerWindow, nowMs: number): void {
    const cutoff = nowMs - this.ttlMs;
    for (const [sequence, seenAt] of window.seen) {
      if (seenAt < cutoff) {
        window.seen.delete(sequence);
      }
    }
  }

  private cleanupNonces(nonces: Map<string, number>, nowMs: number): void {
    const cutoff = nowMs - this.ttlMs;
    for (const [nonce, seenAt] of nonces) {
      if (seenAt < cutoff) {
        nonces.delete(nonce);
      }
    }
  }
}
