import { TimestampExpiredError, TimestampFutureError, TimestampMissingError } from "./errors.js";

export const DEFAULT_MAX_TIMESTAMP_AGE_MS = 5 * 60 * 1000;
export const DEFAULT_CLOCK_SKEW_MS = 30 * 1000;

export function nowMs(): number {
  return Date.now();
}

export function dateFromMs(timestampMs: number): Date {
  return new Date(timestampMs);
}

export function rfc3339SecondsFromMs(timestampMs: number): string {
  const date = dateFromMs(timestampMs);
  if (Number.isNaN(date.getTime())) {
    throw new TimestampMissingError("timestamp is invalid");
  }
  return date.toISOString().replace(/\.\d{3}Z$/u, "Z");
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
    if (this.maxAgeMs <= 0) {
      throw new TimestampMissingError("maxAgeMs must be positive");
    }
    if (this.clockSkewMs < 0) {
      throw new TimestampMissingError("clockSkewMs cannot be negative");
    }
  }

  validate(timestampMs: number, now: Date = new Date()): void {
    if (timestampMs === 0) {
      throw new TimestampMissingError("timestamp is missing or zero");
    }
    const messageTime = dateFromMs(timestampMs);
    if (Number.isNaN(messageTime.getTime())) {
      throw new TimestampMissingError("timestamp is invalid");
    }
    if (messageTime.getTime() < now.getTime() - this.maxAgeMs) {
      throw new TimestampExpiredError("timestamp expired");
    }
    if (messageTime.getTime() > now.getTime() + this.clockSkewMs) {
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
