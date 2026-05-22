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
  tryDecodeFrame,
  validateMaxFrameSize,
} from "./framing.js";

export interface ReceiveOptions {
  readonly timeoutMs?: number;
}

export interface NodeStreamConnectionOptions {
  readonly maxFrameSize?: number;
}

interface PendingRead {
  readonly endian: FrameEndian;
  readonly resolve: (payload: Uint8Array) => void;
  readonly reject: (error: Error) => void;
  readonly timer: NodeJS.Timeout | undefined;
}

export class NodeStreamConnection {
  private readonly stream: Duplex;
  private readonly maxFrameSize: number;
  private buffer = Buffer.alloc(0);
  private pendingRead: PendingRead | undefined;
  private readChain: Promise<void> = Promise.resolve();
  private writeChain: Promise<void> = Promise.resolve();
  private closedInternal = false;
  private failure: Error | undefined;

  constructor(stream: Duplex, options: NodeStreamConnectionOptions = {}) {
    this.stream = stream;
    this.maxFrameSize = validateMaxFrameSize(options.maxFrameSize ?? DEFAULT_MAX_FRAME_SIZE);
    this.stream.on("data", (chunk: Buffer | Uint8Array) => this.handleData(chunk));
    this.stream.once("end", () => this.markClosed(new TransportClosedError("stream ended")));
    this.stream.once("close", () => this.markClosed(new TransportClosedError("stream closed")));
    this.stream.once("error", (error: Error) => this.markClosed(error));
  }

  get closed(): boolean {
    return this.closedInternal || this.stream.destroyed;
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

  async receiveEnvelope(options: ReceiveOptions = {}): Promise<AxcpEnvelope> {
    const payload = await this.enqueueRead("little", options.timeoutMs);
    return decodeEnvelope(payload);
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
    const operation = this.readChain.then(() => this.readFrame(endian, timeoutMs));
    this.readChain = operation.then(
      () => undefined,
      () => undefined,
    );
    return operation;
  }

  private readFrame(endian: FrameEndian, timeoutMs: number | undefined): Promise<Uint8Array> {
    this.ensureOpen();
    const decoded = this.tryReadBufferedFrame(endian);
    if (decoded !== undefined) {
      return Promise.resolve(decoded);
    }

    return new Promise<Uint8Array>((resolve, reject) => {
      const timer =
        timeoutMs === undefined
          ? undefined
          : setTimeout(() => {
              this.pendingRead = undefined;
              reject(new TransportTimeoutError("timed out reading frame"));
            }, validateTimeout(timeoutMs));

      this.pendingRead = {
        endian,
        resolve: (payload) => {
          if (timer !== undefined) {
            clearTimeout(timer);
          }
          resolve(payload);
        },
        reject: (error) => {
          if (timer !== undefined) {
            clearTimeout(timer);
          }
          reject(error);
        },
        timer,
      };
      this.tryResolvePendingRead();
    });
  }

  private handleData(chunk: Buffer | Uint8Array): void {
    this.buffer = Buffer.concat([this.buffer, Buffer.from(chunk)]);
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
      this.pendingRead = undefined;
      this.fail(error instanceof Error ? error : new TransportFrameError("failed to decode frame"));
      pending.reject(error instanceof Error ? error : new TransportFrameError("failed to decode frame"));
    }
  }

  private tryReadBufferedFrame(endian: FrameEndian): Uint8Array | undefined {
    const decoded = tryDecodeFrame(this.buffer, { endian, maxFrameSize: this.maxFrameSize });
    if (decoded === undefined) {
      return undefined;
    }
    this.buffer = Buffer.from(decoded.remaining);
    return decoded.payload;
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
    this.failure = error;
    if (!this.stream.destroyed) {
      this.stream.destroy(error);
    }
    this.markClosed(error);
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
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0) {
    throw new TransportTimeoutError("timeoutMs must be a positive safe integer");
  }
  return timeoutMs;
}
