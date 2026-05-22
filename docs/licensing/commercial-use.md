# Commercial Use Guide

## AXCP Core (Apache 2.0)

**AXCP Core is fully open source under the Apache 2.0 license.** There are no commercial restrictions.

### What You Can Do (Free, No License Required)

| Use Case | Allowed? |
|----------|----------|
| Personal projects | ✅ Yes |
| Academic research | ✅ Yes |
| Non-profit organizations | ✅ Yes |
| Internal company tools | ✅ Yes |
| Commercial products | ✅ Yes |
| SaaS platforms | ✅ Yes |
| Consulting services | ✅ Yes |
| Redistributing AXCP Core | ✅ Yes |
| Modifying AXCP Core | ✅ Yes |
| Keeping modifications private | ✅ Yes |

**Apache 2.0 is a permissive license.** You can use AXCP Core for any purpose, including commercial use, without paying fees or obtaining special permission.

### Requirements

When using AXCP Core, you must:

1. **Include the license**: Retain the Apache 2.0 license text in distributions
2. **Preserve notices**: Keep copyright and attribution notices intact
3. **State changes**: If you modify the code, indicate that changes were made

### What AXCP Core Includes

- QUIC + TLS 1.3 + Protobuf transport
- DID mutual authentication
- Ed25519 message signatures
- Replay attack protection
- Profile negotiation
- Capability negotiation & tool discovery
- Basic telemetry
- Gateway for MCP ↔ AXCP ↔ A2A ↔ ACP bridging
- Go SDK, Rust SDK, Python SDK, TypeScript SDK

---

## AXCP Advanced & Enterprise (Commercial License Required)

Advanced features require a commercial license from TradePhantom LLC.

### Features Requiring Commercial License

**AXCP Advanced ($)**
- Context-Sync with delta patches (CRDT)
- mTLS client certificates
- SGX/SEV enclave support
- Differential Privacy (optional)
- Advanced rate limiting
- Structured logging

**AXCP Enterprise ($$)**
- Differential Privacy (mandatory enforcement)
- PII filtering & redaction
- Compliance reporting (GDPR, HIPAA, SOC2)
- Audit trails (Merkle tree verified)
- TRI-AI Orchestration System
- Multi-tenant isolation
- Dedicated support + SLA

### How to Get a Commercial License

Contact TradePhantom LLC:
- Email: enterprise@getaxcp.com
- Website: https://tradephantom.com

---

## Examples

### Example 1: Startup Building an AI Agent Platform

> "I want to build a SaaS platform that orchestrates AI agents using AXCP."

**Answer:** You can use AXCP Core freely under Apache 2.0. If you need Context-Sync, Differential Privacy, or compliance features, contact us for an Advanced or Enterprise license.

### Example 2: Enterprise Internal Deployment

> "We want to use AXCP internally for our company's AI infrastructure."

**Answer:** AXCP Core is free for internal use. If you need advanced features like mTLS, audit trails, or GDPR compliance tools, contact us for a commercial license.

### Example 3: Open Source Project

> "I'm building an open source tool that integrates with AXCP."

**Answer:** AXCP Core is Apache 2.0, fully compatible with most open source licenses. You can integrate, modify, and redistribute freely.

### Example 4: Consulting/Services

> "I offer AI consulting and want to deploy AXCP for my clients."

**Answer:** You can use and deploy AXCP Core for your clients under Apache 2.0. If clients need Advanced/Enterprise features, they should obtain their own commercial licenses.

---

## Summary

| Question | Answer |
|----------|--------|
| Is AXCP Core free? | Yes, Apache 2.0 |
| Can I use it commercially? | Yes |
| Can I modify it? | Yes |
| Can I keep modifications private? | Yes |
| Do I need to pay anything? | No, for Core features |
| What requires payment? | Advanced & Enterprise features |
