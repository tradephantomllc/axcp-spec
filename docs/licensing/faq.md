# Licensing FAQ

## General Questions

### Q: What license is AXCP Core under?
**A:** Apache License 2.0 – a permissive open source license that allows commercial use, modification, and distribution.

### Q: Is AXCP Core really free?
**A:** Yes. AXCP Core is completely free for any use, including commercial products and SaaS platforms. No fees, no royalties, no special permissions needed.

### Q: What is the boundary between Core and the commercial product?
**A:** Core contains the protocol and explicit integration building blocks listed in the [current scope](../../README.md#what-core-provides). The private commercial product adds licensed capabilities. Advanced and Enterprise are historical repository/feature labels, not two public Core editions. Core does not implement automated orchestration or claim complete MCP/A2A/ACP adapters.

---

## Using AXCP Core

### Q: Can I use AXCP Core in a commercial product?
**A:** Yes. Apache 2.0 explicitly permits commercial use.

### Q: Can I use AXCP Core in a SaaS platform?
**A:** Yes. There are no restrictions on how you deploy or offer AXCP Core.

### Q: Can I modify AXCP Core?
**A:** Yes. You can modify it freely. If you redistribute the modified code, you must indicate that changes were made.

### Q: Can I extend AXCP Core myself, for example with my own mTLS setup or other security features?
**A:** Yes. Apache 2.0 places no field-of-use restrictions on Core. You may build any extension yourself, including features similar to commercial capabilities, without a commercial license.

### Q: Do I have to open source my modifications?
**A:** No. Apache 2.0 does not require you to share modifications. You can keep them private.

### Q: Do I have to share my product's source code if I use AXCP?
**A:** No. Apache 2.0 is not a copyleft license. Your proprietary code remains yours.

### Q: What do I need to include when redistributing AXCP Core?
**A:** You must include:
1. The Apache 2.0 license text
2. The NOTICE file
3. Any copyright/attribution notices from the original
4. If modified, a note indicating changes were made

---

## Commercial capabilities

### Q: Which vendor-provided components require a commercial license?
**A:** Only TradePhantom's own commercial components, which are distributed separately from this repository. See the [Commercial Use Guide](./commercial-use.md) for an overview. Examples of capability areas outside Core include:
- Automatic context synchronization (CRDT)
- Differential privacy controls
- Managed mTLS certificate lifecycle and attestation
- Compliance-related tooling (no certification guarantee)
- Audit and governance tooling
- Automatic orchestration and routing

Contact enterprise@getaxcp.com for the current commercial scope.

### Q: Can I evaluate commercial features before buying?
**A:** Contact us at enterprise@getaxcp.com to discuss evaluation options.

### Q: How is the commercial license priced?
**A:** Pricing depends on your use case and scale. Contact enterprise@getaxcp.com for a quote.

---

## Compatibility

### Q: Is Apache 2.0 compatible with GPL?
**A:** Apache 2.0 is compatible with GPLv3 but not with GPLv2 (due to patent clause differences). Check compatibility for your specific use case.

### Q: Can I combine AXCP Core with MIT-licensed code?
**A:** Yes. Apache 2.0 is compatible with MIT and most permissive licenses.

### Q: Can I use AXCP Core in a proprietary product?
**A:** Yes. Apache 2.0 allows combining with proprietary code.

---

## Contributing

### Q: Do I need to sign a CLA to contribute?
**A:** Yes. Contributors must sign the Contributor License Agreement through [CLA Assistant](https://cla-assistant.io/tradephantomllc/axcp-spec); CLA Assistant prompts you on your first pull request, and the `license/cla` check must pass before merge. In addition, every commit needs a Developer Certificate of Origin sign-off (`Signed-off-by`, added with `git commit -s`). See [CONTRIBUTING.md](../../CONTRIBUTING.md).

### Q: Who owns my contributions, and what rights do I grant?
**A:** The rights you grant are defined by the CLA presented by CLA Assistant. Please read it before signing. Accepted contributions are distributed as part of AXCP Core under Apache 2.0.

### Q: Can my contributions end up in the commercial versions?
**A:** Contributions to AXCP Core are distributed under Apache 2.0 as part of Core. Commercial features are developed separately. For any further use of contributions, the CLA terms apply.

---

## Contact

For licensing questions not answered here:
- Email: enterprise@getaxcp.com
- Website: https://tradephantom.com
