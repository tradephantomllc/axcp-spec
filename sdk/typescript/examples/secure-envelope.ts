import { Agent, DeltaOpType, Identity } from "../src/index.js";

const alice = new Agent(Identity.generate());
const bob = new Agent(Identity.generate());

const envelope = alice.newEnvelope({ traceId: "trace-001" });
envelope.contextPatch = {
  contextId: "ctx",
  baseVersion: 1,
  ops: [
    {
      op: DeltaOpType.ADD,
      path: "/message",
      data: new TextEncoder().encode("hello from TypeScript"),
      ts: 1,
    },
  ],
};

alice.signMessage(envelope, { recipientDid: bob.identity.did });
await bob.verify(envelope);

console.log(`verified ${envelope.traceId} from ${envelope.senderDid}`);
