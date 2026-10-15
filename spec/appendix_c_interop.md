# Appendix C — Interop Profiles

> **⚠️ SUPERSEDED / INFORMATIVE**: This appendix belongs to the superseded v0.2 draft. It is retained for historical reference only and contains no requirements. The normative contract is **[AXCP Core Specification v1.0](./axcp-v1.0.md)**.
>
> AXCP Core does not ship an A2A adapter or a complete MCP adapter. The repository contains only a narrow MCP conversion example (`gateway/mcp_bridge.py`). The mappings below are illustrative ideas, not implemented or supported behavior.

The v0.2 draft envisioned interoperating with existing agent protocols through stateless gateways:

| Legacy proto | Direction | Illustrative mapping |
|--------------|-----------|----------------------|
| **MCP v0.1** | MCP → AXCP | `McpRequest` ➜ `AxcpEnvelope{ capability_msg.offer }` |
|              | AXCP → MCP | `ContextPatch` ➜ MCP `context_delta` |
| **A2A AgentCard 2025-05** | A2A → AXCP | `agentCard.task` ➜ `CapabilityRequest` |
|              | AXCP → A2A | `CapabilityAck` ➜ `TaskAccepted` |

Notes:

1. A translating component would only repack JSON ⇄ Protobuf and would not alter payload semantics.
2. A translating component never lowers the security profile. Under v1.0, envelopes it emits or forwards are Secure Baseline envelopes (`profile = 1`) signed by an identity it controls and addressed to an explicit recipient; see [v1.0 §4](./axcp-v1.0.md#4-security-profiles) and [§6](./axcp-v1.0.md#6-identity-and-authentication).
3. Correlation: `trace_id = legacy_request.id`.
