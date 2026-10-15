# AXCP Core Public Roadmap

This roadmap covers the open-source AXCP Core in this repository. It states direction, not delivery dates. Commercial capabilities are offered separately and are not tracked here.

## Released

**AXCP Core v1.0 (Secure Baseline)**, first released 2026-01-29. The specification has since been revised; see its [Revision History](spec/axcp-v1.0.md#revision-history).

- QUIC transport with TLS 1.3 (ALPN `axcp/1`)
- Protobuf envelopes
- DID-based identity with Ed25519 signatures over a canonical transcript
- Recipient binding, timestamp freshness and per-sender replay protection
- Go reference SDK, Python and TypeScript SDKs, shared cross-SDK test vectors
- Reference QUIC-to-MQTT gateway with Secure Baseline enabled by default

See the [README](README.md#what-core-provides) for the exact scope and limits of each component.

## In progress

- Security hardening of the Go, Python and TypeScript SDKs and the gateway to the revised Secure Baseline rules (strict envelope parsing, DID and `did:key` validation, bounded replay state and transport buffers)
- Broader shared test vectors covering rejection cases, so that all SDKs accept and reject the same envelopes
- Documentation and CI alignment with the implemented scope

## Under consideration

- A specified on-wire profile negotiation, if deployments need one (today the profile is configured out of band)
- `did:key` support in every SDK and examples that use conformant identifiers
- Published, versioned SDK packages

Items under consideration are not commitments.

## Governance

Changes to the specification and SDKs go through public pull requests. An open governance working group for the specification may be formed in the future; there is no fixed date.

## License

AXCP Core is licensed under Apache 2.0. See [licensing information](docs/licensing/README.md).

Contact: enterprise@getaxcp.com
