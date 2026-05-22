import tls from "node:tls";
import { TransportConfigError } from "../errors.js";
import { NodeStreamConnection } from "./connection.js";
import {
  DEFAULT_ALPN,
  DEFAULT_CONNECT_TIMEOUT_MS,
  DEFAULT_MAX_FRAME_SIZE,
  validateMaxFrameSize,
} from "./framing.js";
import type { StreamHandler } from "./tcp.js";

type SecureMaterial = string | Buffer | Array<string | Buffer>;

export interface TlsClientConfig {
  readonly host: string;
  readonly port: number;
  readonly serverName?: string;
  readonly ca?: SecureMaterial;
  readonly cert?: SecureMaterial;
  readonly key?: SecureMaterial;
  readonly alpnProtocols?: readonly string[];
  readonly requireAlpn?: boolean;
  readonly insecureSkipVerify?: boolean;
  readonly connectTimeoutMs?: number;
  readonly maxFrameSize?: number;
}

export interface TlsServerConfig {
  readonly host?: string;
  readonly port: number;
  readonly cert: SecureMaterial;
  readonly key: SecureMaterial;
  readonly ca?: SecureMaterial;
  readonly requestCert?: boolean;
  readonly rejectUnauthorized?: boolean;
  readonly alpnProtocols?: readonly string[];
  readonly maxFrameSize?: number;
  readonly onConnectionError?: (error: Error) => void;
}

type NormalizedTlsClientConfig = Required<Omit<TlsClientConfig, "ca" | "cert" | "key">> & {
  readonly ca: SecureMaterial | undefined;
  readonly cert: SecureMaterial | undefined;
  readonly key: SecureMaterial | undefined;
};

type NormalizedTlsServerConfig = Required<Omit<TlsServerConfig, "ca" | "onConnectionError">> & {
  readonly ca: SecureMaterial | undefined;
  readonly onConnectionError: ((error: Error) => void) | undefined;
};

export class TlsClient extends NodeStreamConnection {
  static async connect(config: TlsClientConfig): Promise<TlsClient> {
    const normalized = validateTlsClientConfig(config);
    const socket = await connectTlsSocket(normalized);
    return new TlsClient(socket, { maxFrameSize: normalized.maxFrameSize });
  }
}

export class TlsServer {
  private readonly server: tls.Server;
  private readonly connections = new Set<NodeStreamConnection>();
  private closed = false;

  private constructor(
    server: tls.Server,
    private readonly config: NormalizedTlsServerConfig,
  ) {
    this.server = server;
  }

  static async listen(config: TlsServerConfig, handler: StreamHandler): Promise<TlsServer> {
    const normalized = validateTlsServerConfig(config);
    const serverWrapper = new TlsServer(
      tls.createServer(
        {
          cert: normalized.cert,
          key: normalized.key,
          ca: normalized.ca,
          requestCert: normalized.requestCert,
          rejectUnauthorized: normalized.rejectUnauthorized,
          ALPNProtocols: [...normalized.alpnProtocols],
        },
        (socket) => {
          const connection = new NodeStreamConnection(socket, { maxFrameSize: normalized.maxFrameSize });
          serverWrapper.connections.add(connection);
          socket.once("close", () => serverWrapper.connections.delete(connection));
          Promise.resolve(handler(connection))
            .catch((error: Error) => {
              normalized.onConnectionError?.(error);
            })
            .finally(() => {
              void connection.close();
            });
        },
      ),
      normalized,
    );

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
  }
}

function validateTlsClientConfig(config: TlsClientConfig): NormalizedTlsClientConfig {
  if (!config.host) {
    throw new TransportConfigError("host is required");
  }
  const alpnProtocols = config.alpnProtocols ?? [DEFAULT_ALPN];
  if (alpnProtocols.length === 0) {
    throw new TransportConfigError("at least one ALPN protocol is required");
  }
  return {
    host: config.host,
    port: validateClientPort(config.port),
    serverName: config.serverName ?? config.host,
    ca: config.ca,
    cert: config.cert,
    key: config.key,
    alpnProtocols,
    requireAlpn: config.requireAlpn ?? true,
    insecureSkipVerify: config.insecureSkipVerify ?? false,
    connectTimeoutMs: validateTimeout(config.connectTimeoutMs ?? DEFAULT_CONNECT_TIMEOUT_MS),
    maxFrameSize: validateMaxFrameSize(config.maxFrameSize ?? DEFAULT_MAX_FRAME_SIZE),
  };
}

function validateTlsServerConfig(config: TlsServerConfig): NormalizedTlsServerConfig {
  if (!config.cert) {
    throw new TransportConfigError("cert is required");
  }
  if (!config.key) {
    throw new TransportConfigError("key is required");
  }
  const alpnProtocols = config.alpnProtocols ?? [DEFAULT_ALPN];
  if (alpnProtocols.length === 0) {
    throw new TransportConfigError("at least one ALPN protocol is required");
  }
  return {
    host: config.host ?? "127.0.0.1",
    port: validateServerPort(config.port),
    cert: config.cert,
    key: config.key,
    ca: config.ca,
    requestCert: config.requestCert ?? false,
    rejectUnauthorized: config.rejectUnauthorized ?? true,
    alpnProtocols,
    maxFrameSize: validateMaxFrameSize(config.maxFrameSize ?? DEFAULT_MAX_FRAME_SIZE),
    onConnectionError: config.onConnectionError,
  };
}

function connectTlsSocket(config: NormalizedTlsClientConfig): Promise<tls.TLSSocket> {
  return new Promise((resolve, reject) => {
    const socket = tls.connect({
      host: config.host,
      port: config.port,
      servername: config.serverName,
      ca: config.ca,
      cert: config.cert,
      key: config.key,
      ALPNProtocols: [...config.alpnProtocols],
      rejectUnauthorized: !config.insecureSkipVerify,
    });

    const timer = setTimeout(() => {
      socket.destroy();
      reject(new TransportConfigError("TLS connect timed out"));
    }, config.connectTimeoutMs);

    const cleanup = (): void => {
      clearTimeout(timer);
      socket.off("secureConnect", onSecureConnect);
      socket.off("error", onError);
    };
    const onSecureConnect = (): void => {
      cleanup();
      if (!config.insecureSkipVerify && !socket.authorized) {
        socket.destroy();
        reject(new TransportConfigError(socket.authorizationError?.message ?? "TLS authorization failed"));
        return;
      }
      if (
        config.requireAlpn &&
        (typeof socket.alpnProtocol !== "string" || !config.alpnProtocols.includes(socket.alpnProtocol))
      ) {
        socket.destroy();
        reject(new TransportConfigError(`TLS ALPN negotiation failed for ${DEFAULT_ALPN}`));
        return;
      }
      resolve(socket);
    };
    const onError = (error: Error): void => {
      cleanup();
      reject(error);
    };
    socket.once("secureConnect", onSecureConnect);
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
