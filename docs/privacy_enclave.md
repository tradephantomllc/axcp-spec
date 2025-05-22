© 2025 TradePhantom LLC – BSL 1.1 / Apache-2.0 fallback

# Privacy Enclaves and Confidential Execution

AXCP supports execution in trusted computing environments such as Intel SGX or confidential VMs.

These allow nodes to:
- Process sensitive payloads in isolated memory
- Seal secrets that are unreadable to the host OS
- Provide cryptographic proofs of computation integrity (attestation)

## Use Cases
- Secure query processing (e.g., filtered search over private datasets)
- Local evaluation of policies (e.g., "is user allowed to...") without leaking logic
- Distributed federated learning with confidential gradient sharing

## Integration with AXCP

Fields like `AxcpEnvelope.signature` and `policy_blob` (in `RoutePolicyMessage`) can be generated or validated within enclaves.

While not mandatory in v0.1, enclave integration is documented for reference and prototyping.
