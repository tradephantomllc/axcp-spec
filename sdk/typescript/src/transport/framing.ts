import { decodeEnvelope, encodeEnvelope } from "../envelope.js";
import { TransportFrameError } from "../errors.js";
import type { AxcpEnvelope } from "../pb/schema.js";

export const DEFAULT_MAX_FRAME_SIZE = 10 * 1024 * 1024;
export const DEFAULT_ALPN = "axcp/1";
export const DEFAULT_CONNECT_TIMEOUT_MS = 8_000;

export type FrameEndian = "big" | "little";

export interface FrameOptions {
  readonly endian: FrameEndian;
  readonly maxFrameSize?: number;
}

export interface DecodedFrame {
  readonly payload: Uint8Array;
  readonly remaining: Buffer;
}

export function encodeFrame(data: Uint8Array, options: FrameOptions): Buffer {
  const maxFrameSize = validateMaxFrameSize(options.maxFrameSize);
  if (data.length > maxFrameSize) {
    throw new TransportFrameError(`frame length ${data.length} exceeds maxFrameSize ${maxFrameSize}`);
  }
  const header = Buffer.allocUnsafe(4);
  if (options.endian === "big") {
    header.writeUInt32BE(data.length, 0);
  } else {
    header.writeUInt32LE(data.length, 0);
  }
  return Buffer.concat([header, Buffer.from(data)]);
}

export function tryDecodeFrame(buffer: Buffer, options: FrameOptions): DecodedFrame | undefined {
  const maxFrameSize = validateMaxFrameSize(options.maxFrameSize);
  if (buffer.length < 4) {
    return undefined;
  }

  const frameLength = options.endian === "big" ? buffer.readUInt32BE(0) : buffer.readUInt32LE(0);
  if (frameLength > maxFrameSize) {
    throw new TransportFrameError(`frame length ${frameLength} exceeds maxFrameSize ${maxFrameSize}`);
  }
  if (buffer.length < frameLength + 4) {
    return undefined;
  }

  return {
    payload: new Uint8Array(buffer.subarray(4, frameLength + 4)),
    remaining: buffer.subarray(frameLength + 4),
  };
}

export function encodeRawMessageFrame(data: Uint8Array, maxFrameSize = DEFAULT_MAX_FRAME_SIZE): Buffer {
  return encodeFrame(data, { endian: "big", maxFrameSize });
}

export function encodeEnvelopeFrame(envelope: AxcpEnvelope, maxFrameSize = DEFAULT_MAX_FRAME_SIZE): Buffer {
  return encodeFrame(encodeEnvelope(envelope), { endian: "little", maxFrameSize });
}

export function decodeEnvelopeFramePayload(data: Uint8Array): AxcpEnvelope {
  return decodeEnvelope(data);
}

export function validateMaxFrameSize(maxFrameSize = DEFAULT_MAX_FRAME_SIZE): number {
  if (!Number.isSafeInteger(maxFrameSize) || maxFrameSize <= 0) {
    throw new TransportFrameError("maxFrameSize must be a positive safe integer");
  }
  if (maxFrameSize > 0xffff_ffff) {
    throw new TransportFrameError("maxFrameSize cannot exceed uint32 max");
  }
  return maxFrameSize;
}
