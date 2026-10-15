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

**Apache 2.0 is a permissive license.** You can use AXCP Core for any purpose, including commercial use, without paying fees or obtaining special permission. Apache 2.0 places no field-of-use restrictions on Core: you may extend it yourself in any direction, including your own mutual TLS, privacy, audit or orchestration features, without a commercial license.

### Requirements

When using AXCP Core, you must:

1. **Include the license**: Retain the Apache 2.0 license text in distributions
2. **Preserve notices**: Keep copyright and attribution notices intact
3. **State changes**: If you modify the code, indicate that changes were made

### What AXCP Core includes

See the maintained [Core scope matrix](../../README.md#what-core-provides) for per-SDK support and explicit limits. The gateway provides QUIC-to-MQTT forwarding; the MCP file is a narrow conversion demonstration. They do not constitute complete MCP/A2A/ACP protocol bridges or automatic tool discovery.

## Private commercial product

TradePhantom's own commercial implementations are distributed under a separate license, with capabilities enabled according to that license. A commercial license is only needed to use those vendor-provided components. Advanced and Enterprise are historical repository/feature names. Automatic orchestration, CRDT synchronization, privacy controls, managed certificates/attestation, PII and governance capabilities are outside this public repository. This describes the vendor's product boundary and does not restrict independent implementations under Core's Apache 2.0 license.

Confirm delivered capabilities and support terms with TradePhantom. This repository does not certify GDPR, HIPAA or SOC 2 compliance.

### How to Get a Commercial License

Contact TradePhantom LLC:
- Email: enterprise@getaxcp.com
- Website: https://tradephantom.com

---

## Examples

### Example 1: Startup Building an AI Agent Platform

> "I want to build a SaaS platform that orchestrates AI agents using AXCP."

**Answer:** You can use AXCP Core freely under Apache 2.0, and you may build any additional features yourself. If you want TradePhantom's commercial components (for example context synchronization or privacy controls), contact us for a commercial license.

### Example 2: Enterprise Internal Deployment

> "We want to use AXCP internally for our company's AI infrastructure."

**Answer:** AXCP Core is free for internal use. You may configure your own mutual TLS or build other extensions yourself under Apache 2.0. If you want TradePhantom's managed certificate lifecycle, audit or compliance tooling, contact us for a commercial license.

### Example 3: Open Source Project

> "I'm building an open source tool that integrates with AXCP."

**Answer:** AXCP Core is Apache 2.0, fully compatible with most open source licenses. You can integrate, modify, and redistribute freely.

### Example 4: Consulting/Services

> "I offer AI consulting and want to deploy AXCP for my clients."

**Answer:** You can use and deploy AXCP Core for your clients under Apache 2.0. If clients need commercial capabilities, they should obtain their own commercial licenses.

---

## Summary

| Question | Answer |
|----------|--------|
| Is AXCP Core free? | Yes, Apache 2.0 |
| Can I use it commercially? | Yes |
| Can I modify it? | Yes |
| Can I keep modifications private? | Yes |
| Do I need to pay anything? | No, for Core features |
| What requires payment? | Vendor-provided commercial components |
