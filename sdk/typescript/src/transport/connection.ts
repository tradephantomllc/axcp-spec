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

/** Chunks at least this large are queued as they are; smaller ones are coalesced. */
const COALESCE_BELOW = 4 * 1024;
/** Capacity of each buffer that small chunks are copied into. */
const COALESCE_CAPACITY = 64 * 1024;

/**
 * FIFO of received bytes with amortized constant-time appends and reads. Small chunks
 * are copied into shared buffers, so many tiny chunks cost neither one queue entry each
 * nor repeated array shifts.
 */
class ByteQueue {
  private chunks: Buffer[] = [];
  /** Index of the first unread chunk and bytes already read from it. */
  private head = 0;
  private headOffset = 0;
  /** Coalescing buffer backing the last chunk, while it still has room. */
  private tail: Buffer | undefined;
  private tailLength = 0;
  length = 0;

  push(data: Buffer): void {
    if (data.length >= COALESCE_BELOW) {
      this.tail = undefined;
      this.chunks.push(data);
    } else {
      if (this.tail === undefined || this.tailLength + data.length > this.tail.length) {
        this.tail = Buffer.allocUnsafe(COALESCE_CAPACITY);
        this.tailLength = 0;
        this.chunks.push(this.tail.subarray(0, 0));
      }
      data.copy(this.tail, this.tailLength);
      this.tailLength += data.length;
      this.chunks[this.chunks.length - 1] = this.tail.subarray(0, this.tailLength);
    }
    this.length += data.length;
  }

  /** Copy `length` bytes from the front; remove them when `consume` is set. */
  read(length: number, consume: boolean): Buffer {
    const out = Buffer.allocUnsafe(length);
    let offset = 0;
    let index = this.head;
    let start = this.headOffset;
    while (offset < length) {
      const chunk = this.chunks[index]!;
      const take = Math.min(chunk.length - start, length - offset);
      chunk.copy(out, offset, start, start + take);
      offset += take;
      if (start + take === chunk.length) {
        index += 1;
        start = 0;
      } else {
        start += take;
      }
    }
    if (consume) {
      this.head = index;
      this.headOffset = start;
      this.length -= length;
      if (this.length === 0) {
        this.clear();
      } else if (this.head >= 1024 && this.head * 2 >= this.chunks.length) {
        this.chunks = this.chunks.slice(this.head);
        this.head = 0;
      }
    }
    return out;
  }

  clear(): void {
    this.chunks = [];
    this.head = 0;
    this.headOffset = 0;
    this.tail = undefined;
    this.tailLength = 0;
    this.length = 0;
  }
}

/**
 * Length-prefixed frame connection over a Node.js duplex stream.
 *
 * Incoming bytes are kept in a queue with amortized constant-time appends and reads
 * (no quadratic concatenation or shifting). Unread buffered bytes
 * never exceed `maxFrameSize + 4`; a peer that sends more is disconnected. A frame header
 * announcing more than `maxFrameSize` fails the connection as soon as it is read, and no
 * buffer is pre-allocated for the announced size.
 */
export class NodeStreamConnection {
  private readonly stream: Duplex;
  private readonly maxFrameSize: number;
  private readonly maxBufferedBytes: number;
  private readonly readTimeoutMs: number;
  private readonly queue = new ByteQueue();
  private framesReceivedCount = 0;
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

  /** Complete frames delivered to readers so far. */
  get framesReceived(): number {
    return this.framesReceivedCount;
  }

  /** Number of received bytes not yet consumed by a read. */
  get bufferedByteCount(): number {
    return this.queue.length;
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
    if (this.queue.length + buffer.length > this.maxBufferedBytes) {
      this.fail(
        new TransportFrameError(`buffered stream data exceeds maxFrameSize + 4 (${this.maxBufferedBytes} bytes)`),
      );
      return;
    }
    this.queue.push(buffer);
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

  private tryReadBufferedFrame(endian: FrameEndian): Uint8Array | undefined {
    if (this.queue.length < FRAME_HEADER_SIZE) {
      return undefined;
    }
    const header = this.queue.read(FRAME_HEADER_SIZE, false);
    const frameLength = endian === "big" ? header.readUInt32BE(0) : header.readUInt32LE(0);
    if (frameLength > this.maxFrameSize) {
      throw new TransportFrameError(`frame length ${frameLength} exceeds maxFrameSize ${this.maxFrameSize}`);
    }
    if (this.queue.length < FRAME_HEADER_SIZE + frameLength) {
      return undefined;
    }
    this.queue.read(FRAME_HEADER_SIZE, true);
    this.framesReceivedCount += 1;
    return new Uint8Array(this.queue.read(frameLength, true));
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
    this.queue.clear();
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
