# Licensing FAQ

## General Questions

### Q: What license is AXCP Core under?
**A:** Apache License 2.0 – a permissive open source license that allows commercial use, modification, and distribution.

### Q: Is AXCP Core really free?
**A:** Yes. AXCP Core is completely free for any use, including commercial products and SaaS platforms. No fees, no royalties, no special permissions needed.

### Q: What's the difference between Core, Advanced, and Enterprise?
**A:**
- **Core** (Apache 2.0): Full protocol implementation with DID authentication, Ed25519 signatures, QUIC transport, and basic telemetry
- **Advanced** (Commercial): Adds Context-Sync, mTLS, SGX/SEV enclaves, differential privacy options
- **Enterprise** (Commercial): Adds mandatory privacy enforcement, compliance tools, audit trails, TRI-AI orchestration, dedicated support

---

## Using AXCP Core

### Q: Can I use AXCP Core in a commercial product?
**A:** Yes. Apache 2.0 explicitly permits commercial use.

### Q: Can I use AXCP Core in a SaaS platform?
**A:** Yes. There are no restrictions on how you deploy or offer AXCP Core.

### Q: Can I modify AXCP Core?
**A:** Yes. You can modify it freely. If you redistribute the modified code, you must indicate that changes were made.

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

## Advanced & Enterprise Features

### Q: Which features require a commercial license?
**A:** See the [Commercial Use Guide](./commercial-use.md) for the complete list. Key features include:
- Context-Sync with CRDT delta patches
- Differential Privacy
- mTLS client certificates
- SGX/SEV enclave support
- Compliance reporting (GDPR, HIPAA, SOC2)
- Audit trails with Merkle tree verification
- TRI-AI Orchestration System

### Q: Can I evaluate Advanced/Enterprise features before buying?
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
**A:** No. Contributions are accepted under the same Apache 2.0 license via the Developer Certificate of Origin (DCO).

### Q: Who owns my contributions?
**A:** You retain copyright of your contributions. By contributing, you grant a license for your code to be used under Apache 2.0.

### Q: Can my contributions end up in the commercial versions?
**A:** Contributions to AXCP Core remain Apache 2.0. Commercial features are developed separately.

---

## Contact

For licensing questions not answered here:
- Email: enterprise@getaxcp.com
- Website: https://tradephantom.com
