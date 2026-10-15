import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import net from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { performance } from "node:perf_hooks";
import { PassThrough } from "node:stream";
import tls from "node:tls";
import {
  Agent,
  DeltaOpType,
  Identity,
  NodeStreamConnection,
  TcpClient,
  TcpServer,
  TlsClient,
  TlsServer,
  TransportClosedError,
  TransportFrameError,
  TransportTimeoutError,
  TransportUnsupportedError,
  assertNativeQuicAvailable,
  detectNativeQuic,
  encodeFrame,
  tryDecodeFrame,
  type AxcpEnvelope,
} from "../src/index.js";

interface TestCertificate {
  readonly cert: string;
  readonly key: string;
}

function generateTestCertificate(): TestCertificate {
  const dir = mkdtempSync(join(tmpdir(), "axcp-ts-tls-"));
  const certPath = join(dir, "cert.pem");
  const keyPath = join(dir, "key.pem");
  try {
    execFileSync(
      "openssl",
      [
        "req",
        "-x509",
        "-newkey",
        "rsa:2048",
        "-nodes",
        "-keyout",
        keyPath,
        "-out",
        certPath,
        "-days",
        "1",
        "-subj",
        "/CN=localhost",
        "-addext",
        "subjectAltName=DNS:localhost,IP:127.0.0.1",
      ],
      { stdio: "ignore" },
    );
    return {
      cert: readFileSync(certPath, "utf8"),
      key: readFileSync(keyPath, "utf8"),
    };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

function contextEnvelope(agent: Agent, recipientDid: string): AxcpEnvelope {
  const envelope = agent.newEnvelope({ traceId: "transport-trace" });
  envelope.contextPatch = {
    contextId: "ctx",
    baseVersion: 7,
    ops: [{ op: DeltaOpType.ADD, path: "/transport", data: new TextEncoder().encode("ok"), ts: 1 }],
  };
  return agent.signMessage(envelope, { recipientDid });
}

async function signedEcho(handlerTransport: "tcp" | "tls"): Promise<void> {
  const clientAgent = new Agent(Identity.generate());
  const serverAgent = new Agent(Identity.generate());
  const certificate = handlerTransport === "tls" ? generateTestCertificate() : undefined;
  const server =
    handlerTransport === "tcp"
      ? await TcpServer.listen({ host: "127.0.0.1", port: 0 }, async (connection) => {
          const incoming = await connection.receiveEnvelope({ timeoutMs: 2_000 });
          await serverAgent.verify(incoming);
          const reply = contextEnvelope(serverAgent, incoming.senderDid ?? "");
          await connection.sendEnvelope(reply);
        })
      : await TlsServer.listen(
          { host: "127.0.0.1", port: 0, cert: certificate!.cert, key: certificate!.key },
          async (connection) => {
            const incoming = await connection.receiveEnvelope({ timeoutMs: 2_000 });
            await serverAgent.verify(incoming);
            const reply = contextEnvelope(serverAgent, incoming.senderDid ?? "");
            await connection.sendEnvelope(reply);
          },
        );

  try {
    const address = server.address;
    const client =
      handlerTransport === "tcp"
        ? await TcpClient.connect({ host: "127.0.0.1", port: address.port })
        : await TlsClient.connect({
            host: "127.0.0.1",
            port: address.port,
            serverName: "localhost",
            ca: certificate!.cert,
          });
    try {
      await client.sendEnvelope(contextEnvelope(clientAgent, serverAgent.identity.did));
      const reply = await client.receiveEnvelope({ timeoutMs: 2_000 });
      await clientAgent.verify(reply);
      assert.equal(reply.senderDid, serverAgent.identity.did);
      assert.equal(reply.recipientDid, clientAgent.identity.did);
    } finally {
      await client.close();
    }
  } finally {
    await server.close();
  }
}

describe("transport framing", () => {
  it("encodes raw messages with big-endian length prefixes", () => {
    const frame = encodeFrame(new TextEncoder().encode("abc"), { endian: "big", maxFrameSize: 10 });
    assert.deepEqual([...frame.subarray(0, 4)], [0, 0, 0, 3]);
    const decoded = tryDecodeFrame(frame, { endian: "big", maxFrameSize: 10 });
    assert.equal(new TextDecoder().decode(decoded?.payload), "abc");
    assert.equal(decoded?.remaining.length, 0);
  });

  it("encodes envelopes with little-endian length prefixes", () => {
    const frame = encodeFrame(new Uint8Array([1, 2, 3]), { endian: "little", maxFrameSize: 10 });
    assert.deepEqual([...frame.subarray(0, 4)], [3, 0, 0, 0]);
  });

  it("rejects oversized frames before allocation", () => {
    const header = Buffer.alloc(4);
    header.writeUInt32BE(11, 0);
    assert.throws(() => tryDecodeFrame(header, { endian: "big", maxFrameSize: 10 }), TransportFrameError);
  });
});

describe("transport connections", () => {
  it("roundtrips signed envelopes over the development TCP adapter", async () => {
    await signedEcho("tcp");
  });

  it("roundtrips signed envelopes over the TLS adapter with ALPN", async () => {
    await signedEcho("tls");
  });
});

describe("QUIC availability", () => {
  it("reports native Node QUIC capability explicitly", () => {
    const status = detectNativeQuic();
    assert.equal(status.runtime, "node");
    assert.equal(typeof status.reason, "string");
    if (status.available) {
      assert.doesNotThrow(() => assertNativeQuicAvailable());
    } else {
      assert.throws(() => assertNativeQuicAvailable(), TransportUnsupportedError);
    }
  });
});

function frame(endian: "big" | "little", payload: Uint8Array): Buffer {
  return encodeFrame(payload, { endian, maxFrameSize: 0xffff_ffff });
}

describe("stream buffering bounds (D11)", () => {
  it("reassembles frames split across many chunks and several frames per chunk", async () => {
    const stream = new PassThrough();
    const connection = new NodeStreamConnection(stream, { maxFrameSize: 1024 });
    const first = frame("big", new TextEncoder().encode("first"));
    const second = frame("big", new TextEncoder().encode("second"));
    const both = Buffer.concat([first, second]);
    for (const byte of both) {
      stream.write(Buffer.of(byte));
    }
    assert.equal(new TextDecoder().decode(await connection.receiveMessage({ timeoutMs: 1_000 })), "first");
    assert.equal(new TextDecoder().decode(await connection.receiveMessage({ timeoutMs: 1_000 })), "second");
    assert.equal(connection.bufferedByteCount, 0);
    await connection.close();
  });

  it("never buffers more than maxFrameSize + 4 unread bytes", async () => {
    const stream = new PassThrough();
    const connection = new NodeStreamConnection(stream, { maxFrameSize: 1024 });
    let peak = 0;
    stream.on("data", () => {
      peak = Math.max(peak, connection.bufferedByteCount);
    });
    for (let index = 0; index < 64; index += 1) {
      stream.write(Buffer.alloc(100));
    }
    await new Promise((resolve) => setImmediate(resolve));
    assert.ok(peak <= 1028, `peak buffered bytes ${peak}`);
    assert.ok(connection.closed);
    await assert.rejects(() => connection.receiveMessage({ timeoutMs: 100 }), TransportFrameError);
  });

  it("rejects an oversized frame header immediately without waiting for the body", async () => {
    const stream = new PassThrough();
    const connection = new NodeStreamConnection(stream, { maxFrameSize: 1024 });
    const pending = connection.receiveMessage({ timeoutMs: 5_000 });
    const header = Buffer.alloc(4);
    header.writeUInt32BE(0xffff_ffff, 0);
    const started = performance.now();
    stream.write(header);
    await assert.rejects(pending, TransportFrameError);
    assert.ok(performance.now() - started < 1_000);
    assert.ok(connection.closed);
  });

  it("assembles a large frame from small chunks in linear time", async () => {
    const size = 4 * 1024 * 1024;
    const stream = new PassThrough();
    const connection = new NodeStreamConnection(stream, { maxFrameSize: size });
    const encoded = frame("big", Buffer.alloc(size, 7));
    const started = performance.now();
    const result = connection.receiveMessage({ timeoutMs: 10_000 });
    for (let offset = 0; offset < encoded.length; offset += 1024) {
      stream.write(encoded.subarray(offset, offset + 1024));
    }
    const bytes = await result;
    assert.equal(bytes.length, size);
    assert.equal(bytes[size - 1], 7);
    assert.ok(performance.now() - started < 3_000, "chunk assembly must not be quadratic");
    await connection.close();
  });

  it("applies the default read timeout", { timeout: 5_000 }, async () => {
    const connection = new NodeStreamConnection(new PassThrough(), { readTimeoutMs: 30 });
    await assert.rejects(() => connection.receiveMessage(), TransportTimeoutError);
    await connection.close();
    await assert.rejects(() => connection.receiveMessage(), TransportClosedError);
  });
});

describe("server handler errors (D11)", () => {
  it("surfaces handler failures from close() instead of dropping them", async () => {
    const server = await TcpServer.listen({ host: "127.0.0.1", port: 0 }, async () => {
      throw new Error("handler exploded");
    });
    const socket = net.connect({ host: "127.0.0.1", port: server.address.port });
    await new Promise<void>((resolve) => socket.once("connect", () => resolve()));
    await new Promise((resolve) => setTimeout(resolve, 50));
    assert.equal(server.handlerErrors.length, 1);
    socket.destroy();
    await assert.rejects(() => server.close(), /handler exploded/);
  });

  it("passes handler failures to onConnectionError when configured", async () => {
    const seen: Error[] = [];
    const server = await TcpServer.listen(
      { host: "127.0.0.1", port: 0, onConnectionError: (error) => seen.push(error) },
      () => {
        throw new Error("sync failure");
      },
    );
    const socket = net.connect({ host: "127.0.0.1", port: server.address.port });
    await new Promise<void>((resolve) => socket.once("connect", () => resolve()));
    await new Promise((resolve) => setTimeout(resolve, 50));
    socket.destroy();
    await server.close();
    assert.equal(seen.length, 1);
    assert.match(seen[0]!.message, /sync failure/);
  });
});

describe("TLS defaults (D11)", () => {
  it("requires TLS 1.3 by default", async () => {
    const certificate = generateTestCertificate();
    const server = await TlsServer.listen(
      { host: "127.0.0.1", port: 0, cert: certificate.cert, key: certificate.key, onConnectionError: () => undefined },
      async (connection) => {
        await connection.receiveMessage({ timeoutMs: 1_000 });
      },
    );
    try {
      const legacy = tls.connect({
        host: "127.0.0.1",
        port: server.address.port,
        ca: certificate.cert,
        servername: "localhost",
        maxVersion: "TLSv1.2",
        ALPNProtocols: ["axcp/1"],
      });
      await new Promise<void>((resolve, reject) => {
        legacy.once("secureConnect", () => reject(new Error("TLS 1.2 handshake unexpectedly succeeded")));
        legacy.once("error", () => resolve());
      });
      legacy.destroy();

      const client = await TlsClient.connect({
        host: "127.0.0.1",
        port: server.address.port,
        serverName: "localhost",
        ca: certificate.cert,
      });
      await client.sendMessage(new TextEncoder().encode("hi"));
      await client.close();
    } finally {
      await server.close();
    }
  });
});
