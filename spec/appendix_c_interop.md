# Appendix C — Interop Profiles

AXCP can interoperate with existing agent protocols via **stateless gateways**.

| Legacy proto | Direction | Mapping |
|--------------|-----------|---------|
| **MCP v0.1** | MCP → AXCP | `McpRequest` ➜ `AxcpEnvelope{ capability_msg.offer }` |
|              | AXCP → MCP | `ContextPatch` ➜ MCP `context_delta` |
| **A2A AgentCard 2025-05** | A2A → AXCP | `agentCard.task` ➜ `CapabilityRequest` |
|              | AXCP → A2A | `CapabilityAck` ➜ `TaskAccepted` |

Rules:

1. Gateway **does not alter payload semantics**; it only repacks JSON → Protobuf or vice-versa.  
2. DP & profile flags propagate 1-to-1 (if incoming MCP lacks DP, gateway sets `profile=0`).  
3. ID forwarding: `trace_id = legacy_request.id`.

_This appendix will expand as new legacy formats appear._
