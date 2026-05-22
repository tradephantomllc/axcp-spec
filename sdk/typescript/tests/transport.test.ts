import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  Agent,
  DeltaOpType,
  Identity,
  TcpClient,
  TcpServer,
  TlsClient,
  TlsServer,
  TransportFrameError,
  TransportUnsupportedError,
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
    if (!status.available) {
      assert.throws(() => {
        throw new TransportUnsupportedError(status.reason);
      }, TransportUnsupportedError);
    }
  });
});
