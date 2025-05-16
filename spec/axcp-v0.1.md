# AXCP v0.1 – Adaptive eXchange Context Protocol

_Work in progress – structure auto-generated._

> This document defines the initial AXCP protocol specification.  
> Version: **v0.1 Draft**  
> Status: **Exploratory Draft (Do not implement)**  
> Last updated: {{INSERT_DATE}}

---

## Table of Contents

1. Preface  
   1.1 Motivation  
   1.2 Status of This Document  
   1.3 Conventions Used

2. Scope & Non-Goals  
   2.1 Supported Topologies  
   2.2 Out-of-scope Features (v0.2+)

3. Terminology  
   3.1 Node, Agent, Tool  
   3.2 Context Segment, Delta Patch  
   3.3 Capability, Contract, Envelope  
   3.4 Edge Node, Cloud Node, Gateway

4. Reference Architecture  
   4.1 Layer Diagram  
   4.2 Sequence Overview  
   4.3 Trust & Threat Model

5. Transport Layer  
   5.1 QUIC Binding  
   5.2 Protobuf Envelope  
   5.3 Connection Establishment (0-RTT / 1-RTT)  
   5.4 Reliability & Flow Control  
   5.5 Security (mTLS, JWT)

6. Context-Sync Layer  
   6.1 Versioned Context Graph  
   6.2 Delta Patch Format  
   6.3 Subscription / Invalidation  
   6.4 Persistence Requirements

7. Capability-Negotiation Layer  
   7.1 DIDComm v2 Handshake  
   7.2 Capability Descriptor  
   7.3 Policy & Access Control  
   7.4 Error Handling

8. Orchestration Layer  
   8.1 Route Policy Language  
   8.2 Edge / Cloud Decision Matrix  
   8.3 Fallback & Retry

9. Privacy & Confidential-Execution  
   9.1 SGX / Confidential-VM Envelope  
   9.2 Differential-Privacy Filter  
   9.3 Audit & Logging

10. Message Types (IDL reference)  
    10.1 `AxcpEnvelope`  
    10.2 `ContextPatch`  
    10.3 `CapabilityOffer / Request`  
    10.4 `RoutePolicy`

11. Error Codes  
12. Security Considerations  
13. IANA / Registry Considerations (reserved type IDs)  
14. Change Log  

---

## Glossary

_(To be compiled after first pass)_

---

## Appendix

**A. Example Edge → Cloud Flow**  
**B. Comparison with MCP / A2A / ACP**

---

© 2025 TradePhantom LLC – All Rights Reserved
