import { createRequire } from "node:module";
import { TransportUnsupportedError } from "../errors.js";

export interface QuicAvailability {
  readonly available: boolean;
  readonly runtime: "node";
  readonly reason: string;
}

export function detectNativeQuic(): QuicAvailability {
  const require = createRequire(import.meta.url);
  try {
    require("node:quic");
    return {
      available: true,
      runtime: "node",
      reason: "node:quic is available in this runtime",
    };
  } catch {
    return {
      available: false,
      runtime: "node",
      reason: "node:quic is not available; use the stream transport API or a vetted optional QUIC adapter",
    };
  }
}

export function assertNativeQuicAvailable(): void {
  const status = detectNativeQuic();
  if (!status.available) {
    throw new TransportUnsupportedError(status.reason);
  }
}
