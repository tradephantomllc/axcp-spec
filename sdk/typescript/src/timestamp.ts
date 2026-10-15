import { performance } from "node:perf_hooks";
import {
  TimestampExpiredError,
  TimestampFutureError,
  TimestampInvalidError,
  TimestampMissingError,
} from "./errors.js";

export const DEFAULT_MAX_TIMESTAMP_AGE_MS = 5 * 60 * 1000;
export const DEFAULT_CLOCK_SKEW_MS = 30 * 1000;
/** Largest accepted timestamp: 9999-12-31T23:59:59.999Z (D8). */
export const MAX_TIMESTAMP_MS = 253_402_300_799_999;

export function nowMs(): number {
  return Date.now();
}

/** Current Unix time in microseconds (used for time-based sequence allocation, D7). */
export function nowMicros(): number {
  return Math.floor((performance.timeOrigin + performance.now()) * 1000);
}

function assertTimestampInRange(timestampMs: number): void {
  if (typeof timestampMs !== "number" || !Number.isSafeInteger(timestampMs)) {
    throw new TimestampInvalidError("timestamp must be a safe integer number of milliseconds");
  }
  if (timestampMs < 0 || timestampMs > MAX_TIMESTAMP_MS) {
    throw new TimestampInvalidError("timestamp is out of range");
  }
}

export function dateFromMs(timestampMs: number): Date {
  assertTimestampInRange(timestampMs);
  return new Date(timestampMs);
}

export function rfc3339SecondsFromMs(timestampMs: number): string {
  return dateFromMs(timestampMs).toISOString().replace(/\.\d{3}Z$/u, "Z");
}

export interface TimestampValidatorOptions {
  readonly maxAgeMs?: number;
  readonly clockSkewMs?: number;
}

export class TimestampValidator {
  readonly maxAgeMs: number;
  readonly clockSkewMs: number;

  constructor(options: TimestampValidatorOptions = {}) {
    this.maxAgeMs = options.maxAgeMs ?? DEFAULT_MAX_TIMESTAMP_AGE_MS;
    this.clockSkewMs = options.clockSkewMs ?? DEFAULT_CLOCK_SKEW_MS;
    if (!Number.isSafeInteger(this.maxAgeMs) || this.maxAgeMs <= 0) {
      throw new TimestampMissingError("maxAgeMs must be a positive safe integer");
    }
    if (!Number.isSafeInteger(this.clockSkewMs) || this.clockSkewMs < 0) {
      throw new TimestampMissingError("clockSkewMs must be a non-negative safe integer");
    }
  }

  validate(timestampMs: number, now: Date = new Date()): void {
    if (timestampMs === 0) {
      throw new TimestampMissingError("timestamp is missing or zero");
    }
    const messageTime = dateFromMs(timestampMs).getTime();
    if (messageTime < now.getTime() - this.maxAgeMs) {
      throw new TimestampExpiredError("timestamp expired");
    }
    if (messageTime > now.getTime() + this.clockSkewMs) {
      throw new TimestampFutureError("timestamp is too far in the future");
    }
  }

  isExpired(timestampMs: number, now: Date = new Date()): boolean {
    try {
      this.validate(timestampMs, now);
    } catch (error) {
      if (error instanceof TimestampExpiredError) {
        return true;
      }
      throw error;
    }
    return false;
  }
}
