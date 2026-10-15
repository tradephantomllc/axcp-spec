import net from "node:net";
import { TransportConfigError } from "../errors.js";
import { DEFAULT_READ_TIMEOUT_MS, NodeStreamConnection } from "./connection.js";
import { DEFAULT_CONNECT_TIMEOUT_MS, DEFAULT_MAX_FRAME_SIZE, validateMaxFrameSize } from "./framing.js";

export type StreamHandler = (connection: NodeStreamConnection) => Promise<void> | void;

/** Maximum number of handler errors retained by a server when no `onConnectionError` is set. */
export const MAX_RETAINED_HANDLER_ERRORS = 100;

/**
 * Records stream-handler failures so they are never silently dropped (D11). When no
 * callback is configured, errors are retained (bounded) and the first one is re-thrown
 * by the server's `close()`.
 */
export class HandlerErrorSink {
  private readonly retained: Error[] = [];
  private droppedCount = 0;

  constructor(private readonly callback: ((error: Error) => void) | undefined) {}

  record(error: unknown): void {
    const normalized = error instanceof Error ? error : new Error(String(error));
    if (this.callback !== undefined) {
      this.callback(normalized);
      return;
    }
    if (this.retained.length < MAX_RETAINED_HANDLER_ERRORS) {
      this.retained.push(normalized);
    } else {
      this.droppedCount += 1;
    }
  }

  get errors(): readonly Error[] {
    return [...this.retained];
  }

  get dropped(): number {
    return this.droppedCount;
  }

  throwFirst(): void {
    const first = this.retained[0];
    if (first !== undefined) {
      throw first;
    }
  }
}

/**
 * Development-only PLAINTEXT TCP adapter: no confidentiality or server authentication.
 * Envelope signatures still authenticate senders, but metadata and payloads are visible
 * on the network. Use {@link TlsClient} outside local development.
 */
export interface TcpClientConfig {
  readonly host: string;
  readonly port: number;
  readonly connectTimeoutMs?: number;
  readonly maxFrameSize?: number;
  readonly readTimeoutMs?: number;
  readonly noDelay?: boolean;
  readonly keepAlive?: boolean;
}

/** Development-only PLAINTEXT TCP server; see {@link TcpClientConfig}. */
export interface TcpServerConfig {
  readonly host?: string;
  readonly port: number;
  readonly maxFrameSize?: number;
  readonly readTimeoutMs?: number;
  readonly noDelay?: boolean;
  readonly keepAlive?: boolean;
  readonly onConnectionError?: (error: Error) => void;
}

type NormalizedTcpServerConfig = Required<Omit<TcpServerConfig, "onConnectionError">> & {
  readonly onConnectionError: ((error: Error) => void) | undefined;
};

export class TcpClient extends NodeStreamConnection {
  static async connect(config: TcpClientConfig): Promise<TcpClient> {
    const normalized = validateTcpClientConfig(config);
    const socket = await connectSocket(normalized);
    return new TcpClient(socket, {
      maxFrameSize: normalized.maxFrameSize,
      readTimeoutMs: normalized.readTimeoutMs,
    });
  }
}

export class TcpServer {
  private readonly server: net.Server;
  private readonly connections = new Set<NodeStreamConnection>();
  private readonly errorSink: HandlerErrorSink;
  private closed = false;

  private constructor(
    server: net.Server,
    private readonly config: NormalizedTcpServerConfig,
  ) {
    this.server = server;
    this.errorSink = new HandlerErrorSink(config.onConnectionError);
  }

  /** Handler errors retained because no `onConnectionError` callback was configured. */
  get handlerErrors(): readonly Error[] {
    return this.errorSink.errors;
  }

  static async listen(config: TcpServerConfig, handler: StreamHandler): Promise<TcpServer> {
    const normalized = validateTcpServerConfig(config);
    const serverWrapper = new TcpServer(net.createServer(), normalized);

    serverWrapper.server.on("connection", (socket) => {
      socket.setNoDelay(normalized.noDelay);
      socket.setKeepAlive(normalized.keepAlive);
      const connection = new NodeStreamConnection(socket, {
        maxFrameSize: normalized.maxFrameSize,
        readTimeoutMs: normalized.readTimeoutMs,
      });
      serverWrapper.connections.add(connection);
      socket.once("close", () => serverWrapper.connections.delete(connection));
      Promise.resolve()
        .then(() => handler(connection))
        .catch((error: unknown) => {
          serverWrapper.errorSink.record(error);
        })
        .finally(() => {
          void connection.close();
        });
    });

    await new Promise<void>((resolve, reject) => {
      const onError = (error: Error): void => {
        serverWrapper.server.off("listening", onListening);
        reject(error);
      };
      const onListening = (): void => {
        serverWrapper.server.off("error", onError);
        resolve();
      };
      serverWrapper.server.once("error", onError);
      serverWrapper.server.once("listening", onListening);
      serverWrapper.server.listen(normalized.port, normalized.host);
    });

    return serverWrapper;
  }

  get address(): { host: string; port: number } {
    const address = this.server.address();
    if (address === null || typeof address === "string") {
      throw new TransportConfigError("server address is not available");
    }
    return { host: address.address, port: address.port };
  }

  /** Close the server; re-throws the first retained handler error, if any. */
  async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    await Promise.all([...this.connections].map((connection) => connection.close()));
    await new Promise<void>((resolve, reject) => {
      this.server.close((error?: Error) => {
        if (error) {
          reject(error);
          return;
        }
        resolve();
      });
    });
    this.errorSink.throwFirst();
  }
}

function validateTcpClientConfig(config: TcpClientConfig): Required<TcpClientConfig> {
  if (!config.host) {
    throw new TransportConfigError("host is required");
  }
  return {
    host: config.host,
    port: validateClientPort(config.port),
    connectTimeoutMs: validateTimeout(config.connectTimeoutMs ?? DEFAULT_CONNECT_TIMEOUT_MS),
    maxFrameSize: validateMaxFrameSize(config.maxFrameSize ?? DEFAULT_MAX_FRAME_SIZE),
    readTimeoutMs: config.readTimeoutMs ?? DEFAULT_READ_TIMEOUT_MS,
    noDelay: config.noDelay ?? true,
    keepAlive: config.keepAlive ?? true,
  };
}

function validateTcpServerConfig(config: TcpServerConfig): NormalizedTcpServerConfig {
  return {
    host: config.host ?? "127.0.0.1",
    port: validateServerPort(config.port),
    maxFrameSize: validateMaxFrameSize(config.maxFrameSize ?? DEFAULT_MAX_FRAME_SIZE),
    readTimeoutMs: config.readTimeoutMs ?? DEFAULT_READ_TIMEOUT_MS,
    noDelay: config.noDelay ?? true,
    keepAlive: config.keepAlive ?? true,
    onConnectionError: config.onConnectionError,
  };
}

function connectSocket(config: Required<TcpClientConfig>): Promise<net.Socket> {
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host: config.host, port: config.port });
    socket.setNoDelay(config.noDelay);
    socket.setKeepAlive(config.keepAlive);
    const timer = setTimeout(() => {
      socket.destroy();
      reject(new TransportConfigError("TCP connect timed out"));
    }, config.connectTimeoutMs);
    const cleanup = (): void => {
      clearTimeout(timer);
      socket.off("connect", onConnect);
      socket.off("error", onError);
    };
    const onConnect = (): void => {
      cleanup();
      resolve(socket);
    };
    const onError = (error: Error): void => {
      cleanup();
      reject(error);
    };
    socket.once("connect", onConnect);
    socket.once("error", onError);
  });
}

function validateClientPort(port: number): number {
  if (!Number.isInteger(port) || port < 1 || port > 65_535) {
    throw new TransportConfigError("port must be between 1 and 65535");
  }
  return port;
}

function validateServerPort(port: number): number {
  if (!Number.isInteger(port) || port < 0 || port > 65_535) {
    throw new TransportConfigError("port must be between 0 and 65535");
  }
  return port;
}

function validateTimeout(timeoutMs: number): number {
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0) {
    throw new TransportConfigError("connectTimeoutMs must be a positive safe integer");
  }
  return timeoutMs;
}
