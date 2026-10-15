import type { Duplex } from "node:stream";
import { decodeEnvelope, encodeEnvelope } from "../envelope.js";
import {
  TransportClosedError,
  TransportFrameError,
  TransportTimeoutError,
} from "../errors.js";
import type { AxcpEnvelope } from "../pb/schema.js";
import {
  DEFAULT_MAX_FRAME_SIZE,
  type FrameEndian,
  encodeFrame,
  validateMaxFrameSize,
} from "./framing.js";

/** Default timeout applied to receive calls that do not pass `timeoutMs` (D11). */
export const DEFAULT_READ_TIMEOUT_MS = 5 * 60 * 1000;
const MAX_TIMER_MS = 2_147_483_647;
const FRAME_HEADER_SIZE = 4;

export interface ReceiveOptions {
  readonly timeoutMs?: number;
}

export interface ReceiveEnvelopeOptions extends ReceiveOptions {
  /** Strict wire decoding (default true). See `decodeEnvelope(..., { strict: true })`. */
  readonly strict?: boolean;
}

export interface NodeStreamConnectionOptions {
  readonly maxFrameSize?: number;
  /** Default receive timeout in milliseconds (default 5 minutes). */
  readonly readTimeoutMs?: number;
}

interface PendingRead {
  readonly endian: FrameEndian;
  readonly resolve: (payload: Uint8Array) => void;
  readonly reject: (error: Error) => void;
}

/**
 * Length-prefixed frame connection over a Node.js duplex stream.
 *
 * Incoming chunks are kept in a list (no quadratic concatenation). Unread buffered bytes
 * never exceed `maxFrameSize + 4`; a peer that sends more is disconnected. A frame header
 * announcing more than `maxFrameSize` fails the connection as soon as it is read, and no
 * buffer is pre-allocated for the announced size.
 */
export class NodeStreamConnection {
  private readonly stream: Duplex;
  private readonly maxFrameSize: number;
  private readonly maxBufferedBytes: number;
  private readonly readTimeoutMs: number;
  private chunks: Buffer[] = [];
  private bufferedBytes = 0;
  private pendingRead: PendingRead | undefined;
  private readChain: Promise<void> = Promise.resolve();
  private writeChain: Promise<void> = Promise.resolve();
  private closedInternal = false;
  private failure: Error | undefined;

  constructor(stream: Duplex, options: NodeStreamConnectionOptions = {}) {
    this.stream = stream;
    this.maxFrameSize = validateMaxFrameSize(options.maxFrameSize ?? DEFAULT_MAX_FRAME_SIZE);
    this.maxBufferedBytes = this.maxFrameSize + FRAME_HEADER_SIZE;
    this.readTimeoutMs = validateTimeout(options.readTimeoutMs ?? DEFAULT_READ_TIMEOUT_MS);
    this.stream.on("data", (chunk: Buffer | Uint8Array) => this.handleData(chunk));
    this.stream.once("end", () => this.markClosed(new TransportClosedError("stream ended")));
    this.stream.once("close", () => this.markClosed(new TransportClosedError("stream closed")));
    this.stream.on("error", (error: Error) => this.markClosed(error));
  }

  get closed(): boolean {
    return this.closedInternal || this.stream.destroyed;
  }

  /** Number of received bytes not yet consumed by a read. */
  get bufferedByteCount(): number {
    return this.bufferedBytes;
  }

  async sendMessage(data: Uint8Array): Promise<void> {
    await this.writeFrame(data, "big");
  }

  async receiveMessage(options: ReceiveOptions = {}): Promise<Uint8Array> {
    return this.enqueueRead("big", options.timeoutMs);
  }

  async sendEnvelope(envelope: AxcpEnvelope): Promise<void> {
    await this.writeFrame(encodeEnvelope(envelope), "little");
  }

  async receiveEnvelope(options: ReceiveEnvelopeOptions = {}): Promise<AxcpEnvelope> {
    const payload = await this.enqueueRead("little", options.timeoutMs);
    return decodeEnvelope(payload, { strict: options.strict ?? true });
  }

  async close(): Promise<void> {
    if (this.closedInternal) {
      return;
    }
    this.markClosed(new TransportClosedError("stream closed"));
    if (!this.stream.destroyed) {
      this.stream.destroy();
    }
    await Promise.resolve();
  }

  private async writeFrame(data: Uint8Array, endian: FrameEndian): Promise<void> {
    const frame = encodeFrame(data, { endian, maxFrameSize: this.maxFrameSize });
    const operation = this.writeChain.then(() => this.writeBuffer(frame));
    this.writeChain = operation.catch(() => undefined);
    await operation;
  }

  private async writeBuffer(frame: Buffer): Promise<void> {
    this.ensureOpen();
    await new Promise<void>((resolve, reject) => {
      const cleanup = (): void => {
        this.stream.off("error", onError);
        this.stream.off("close", onClose);
      };
      const onError = (error: Error): void => {
        cleanup();
        reject(error);
      };
      const onClose = (): void => {
        cleanup();
        reject(new TransportClosedError("stream closed before write completed"));
      };
      this.stream.once("error", onError);
      this.stream.once("close", onClose);
      this.stream.write(frame, (error?: Error | null) => {
        cleanup();
        if (error) {
          reject(error);
          return;
        }
        resolve();
      });
    });
  }

  private enqueueRead(endian: FrameEndian, timeoutMs: number | undefined): Promise<Uint8Array> {
    const effectiveTimeout = validateTimeout(timeoutMs ?? this.readTimeoutMs);
    const operation = this.readChain.then(() => this.readFrame(endian, effectiveTimeout));
    this.readChain = operation.then(
      () => undefined,
      () => undefined,
    );
    return operation;
  }

  private readFrame(endian: FrameEndian, timeoutMs: number): Promise<Uint8Array> {
    this.ensureOpen();
    let decoded: Uint8Array | undefined;
    try {
      decoded = this.tryReadBufferedFrame(endian);
    } catch (error) {
      const failure = error instanceof Error ? error : new TransportFrameError("failed to decode frame");
      this.fail(failure);
      return Promise.reject(failure);
    }
    if (decoded !== undefined) {
      return Promise.resolve(decoded);
    }

    return new Promise<Uint8Array>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pendingRead = undefined;
        reject(new TransportTimeoutError("timed out reading frame"));
      }, timeoutMs);

      this.pendingRead = {
        endian,
        resolve: (payload) => {
          clearTimeout(timer);
          resolve(payload);
        },
        reject: (error) => {
          clearTimeout(timer);
          reject(error);
        },
      };
      this.tryResolvePendingRead();
    });
  }

  private handleData(chunk: Buffer | Uint8Array): void {
    if (this.failure !== undefined || chunk.length === 0) {
      return;
    }
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk.buffer, chunk.byteOffset, chunk.length);
    if (this.bufferedBytes + buffer.length > this.maxBufferedBytes) {
      this.fail(
        new TransportFrameError(`buffered stream data exceeds maxFrameSize + 4 (${this.maxBufferedBytes} bytes)`),
      );
      return;
    }
    this.chunks.push(buffer);
    this.bufferedBytes += buffer.length;
    this.tryResolvePendingRead();
  }

  private tryResolvePendingRead(): void {
    const pending = this.pendingRead;
    if (pending === undefined) {
      return;
    }
    try {
      const decoded = this.tryReadBufferedFrame(pending.endian);
      if (decoded === undefined) {
        return;
      }
      this.pendingRead = undefined;
      pending.resolve(decoded);
    } catch (error) {
      const failure = error instanceof Error ? error : new TransportFrameError("failed to decode frame");
      this.fail(failure);
    }
  }

  /** Copy `length` bytes from the front of the chunk list without consuming them. */
  private peek(length: number): Buffer {
    const out = Buffer.allocUnsafe(length);
    let offset = 0;
    for (const chunk of this.chunks) {
      const take = Math.min(chunk.length, length - offset);
      chunk.copy(out, offset, 0, take);
      offset += take;
      if (offset === length) {
        break;
      }
    }
    return out;
  }

  /** Remove and return `length` bytes from the front of the chunk list. */
  private consume(length: number): Buffer {
    const out = Buffer.allocUnsafe(length);
    let offset = 0;
    while (offset < length) {
      const chunk = this.chunks[0]!;
      const take = Math.min(chunk.length, length - offset);
      chunk.copy(out, offset, 0, take);
      offset += take;
      if (take === chunk.length) {
        this.chunks.shift();
      } else {
        this.chunks[0] = chunk.subarray(take);
      }
    }
    this.bufferedBytes -= length;
    return out;
  }

  private tryReadBufferedFrame(endian: FrameEndian): Uint8Array | undefined {
    if (this.bufferedBytes < FRAME_HEADER_SIZE) {
      return undefined;
    }
    const header = this.peek(FRAME_HEADER_SIZE);
    const frameLength = endian === "big" ? header.readUInt32BE(0) : header.readUInt32LE(0);
    if (frameLength > this.maxFrameSize) {
      throw new TransportFrameError(`frame length ${frameLength} exceeds maxFrameSize ${this.maxFrameSize}`);
    }
    if (this.bufferedBytes < FRAME_HEADER_SIZE + frameLength) {
      return undefined;
    }
    this.consume(FRAME_HEADER_SIZE);
    return new Uint8Array(this.consume(frameLength));
  }

  private ensureOpen(): void {
    if (this.failure !== undefined) {
      throw this.failure;
    }
    if (this.closed) {
      throw new TransportClosedError("stream is closed");
    }
  }

  private fail(error: Error): void {
    if (this.failure === undefined) {
      this.failure = error;
    }
    this.chunks = [];
    this.bufferedBytes = 0;
    this.markClosed(error);
    if (!this.stream.destroyed) {
      this.stream.destroy();
    }
  }

  private markClosed(error: Error): void {
    if (this.closedInternal && this.pendingRead === undefined) {
      return;
    }
    this.closedInternal = true;
    const pending = this.pendingRead;
    if (pending !== undefined) {
      this.pendingRead = undefined;
      pending.reject(error);
    }
  }
}

function validateTimeout(timeoutMs: number): number {
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0 || timeoutMs > MAX_TIMER_MS) {
    throw new TransportTimeoutError(`timeoutMs must be a positive integer no larger than ${MAX_TIMER_MS}`);
  }
  return timeoutMs;
}
