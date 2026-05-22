import { readFileSync } from "node:fs";
import { Agent, Identity, TlsClient, TlsServer } from "../src/index.js";

const serverAgent = new Agent(Identity.generate());
const clientAgent = new Agent(Identity.generate());

const cert = readFileSync("localhost-cert.pem", "utf8");
const key = readFileSync("localhost-key.pem", "utf8");

const server = await TlsServer.listen({ host: "127.0.0.1", port: 61300, cert, key }, async (connection) => {
  const incoming = await connection.receiveEnvelope({ timeoutMs: 5_000 });
  await serverAgent.verify(incoming);

  const reply = serverAgent.newEnvelope(incoming.traceId === undefined ? {} : { traceId: incoming.traceId });
  reply.contextPatch = { contextId: "echo", baseVersion: 1, ops: [] };
  serverAgent.signMessage(reply, { recipientDid: incoming.senderDid ?? "" });
  await connection.sendEnvelope(reply);
});

try {
  const client = await TlsClient.connect({
    host: "127.0.0.1",
    port: server.address.port,
    serverName: "localhost",
    ca: cert,
  });
  try {
    const envelope = clientAgent.newEnvelope({ traceId: "trace-ts-tls" });
    envelope.contextPatch = { contextId: "hello", baseVersion: 1, ops: [] };
    clientAgent.signMessage(envelope, { recipientDid: serverAgent.identity.did });
    await client.sendEnvelope(envelope);

    const reply = await client.receiveEnvelope({ timeoutMs: 5_000 });
    await clientAgent.verify(reply);
    console.log(`verified ${reply.traceId} from ${reply.senderDid}`);
  } finally {
    await client.close();
  }
} finally {
  await server.close();
}
