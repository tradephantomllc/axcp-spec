# AXCP v0.1 – Adaptive eXchange Context Protocol

_Work in progress – structure auto-generated._

> This document defines the initial AXCP protocol specification.  
> Version: **v0.1 Draft**  
> Status: **Exploratory Draft (Do not implement)**  
> Last updated: {{INSERT_DATE}}

---

## Table of Contents

## 1. Preface

AXCP (Adaptive eXchange Context Protocol) was born out of the need to orchestrate distributed AI agents that must:

- Exchange context in a **delta-efficient** way, reducing token overhead
- Negotiate capabilities and tools in a **decentralized** manner (zero vendor-lock-in)
- Preserve **privacy** and enable **verifiability** of computation (enclaves, attestations)

AXCP v0.1 is an **Exploratory Draft**: it defines lexicon, wire formats, and minimal flows for a Proof-of-Concept interoperability framework.

### 1.1 Motivation

| Protocol       | Current Limitation         | How AXCP Overcomes It         |
|----------------|----------------------------|-------------------------------|
| MCP (Anthropic)| Single-vendor stack        | Multi-vendor & edge-aware     |
| A2A (OpenAI)   | No delta-context support   | Delta-synced cache            |
| ACP            | Verbose JSON               | QUIC + binary Protobuf        |

### 1.2 Status of This Document

- **v0.1 Draft** – NOT production-ready  
- Significant changes expected until tag `v0.1-rc`

### 1.3 Conventions Used

- “**MUST/SHOULD/MAY**” follow [RFC 2119]
- Sequence diagrams use **Mermaid**
- Protobuf types in `CamelCase`, fields in `snake_case`


---

## 2. Scope & Non-Goals
### 2.1 Supported Topologies
* **Edge → Cloud** (smart-device ⇄ LLM-backend)  
* **Mesh** tra agenti paritari  
* **Hierarchical** (gateway aggregatore)

### 2.2 Out-of-scope (v0.1)
* Settlement on-chain (rinviato a v0.2)  
* Streaming audio/video nativo  
* Multi-tenant quota enforcement

### 2.3 Operational Profiles

AXCP supports four progressive profiles that balance security, functionality and runtime overhead.

|
 Feature / Layer                     
|
 Profile-0 
**
Basic
**
|
 Profile-1 
**
Secure-Lite
**
|
 Profile-2 
**
Secure + Sync
**
|
 Profile-3 
**
Enterprise-Privacy
**
|
|
------------------------------------
|
:-------------------:
|
:-------------------------:
|
:---------------------------:
|
:--------------------------------:
|
|
 Transport (QUIC + Protobuf)        
|
 ✓ 
|
 ✓ 
|
 ✓ 
|
 ✓ 
|
|
 TLS 1.3                            
|
 ✓ 
|
 ✓ 
|
 ✓ 
|
 ✓ 
|
|
 DID mutual auth (ECDH)             
|
 ✗ 
|
 ✓ 
|
 ✓ 
|
 ✓ 
|
|
 Context-Sync deltas                
|
 ✗ 
|
 ✗ 
|
 ✓ 
|
 ✓ 
|
|
 Enclave execution (SGX / SEV)      
|
 ✗ 
|
 ✗ 
|
 ✓* 
|
 ✓ 
|
|
 Differential-Privacy module        
|
 ✗ 
|
 ✗ 
|
 optional 
|
 ✓ 
|
|
 Advanced metadata anonymisation    
|
 ✗ 
|
 ✗ 
|
 ✗ 
|
 ✓ 
|
|
 ZK-Proof payloads (future)         
|
 ✗ 
|
 ✗ 
|
 ✗ 
|
 roadmap 
|

\* If enclave hardware absent, nodes MAY fall back to standard execution while retaining other Profile-2 guarantees.

**Rationale.**  
Profiles allow adopters to start with a lightweight core and progressively enable advanced layers as their threat-model or regulatory requirements grow.

**Header signalling.**  
Each AXCP envelope carries a 2-bit `profile` field (values 0–3). Nodes **MUST** refuse payloads that request a higher profile than they support.

*Future work*: runtime negotiation (capability handshake) will appear in v0.2; see Appendix D.

---

## 3. Terminology

| Term             | Definition                                                |
|------------------|-----------------------------------------------------------|
| Node             | Process speaking AXCP (edge, cloud, gateway)              |
| Agent            | Logical module that executes a task (e.g., “QA-bot”)      |
| Tool             | Function invocable by the agent (e.g., HTTP GET)          |
| Context Segment  | Versioned JSON object containing state/data               |
| Delta Patch      | Series of DeltaOps {ADD, REPLACE, REMOVE}                 |
| Capability       | Feature declared by a node (e.g., “search”)               |
| Envelope         | Transport Protobuf structure (`AxcpEnvelope`)             |
| Gateway          | Node that handles edge/cloud routing and policy           |

_(Extended glossary at end of document)_



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

---

## 6. Context-Sync Layer

### 6.1 Versioned Context Graph
(TODO: Define how versioning of context segments is tracked, including timestamped updates, segment IDs, and reconciliation logic)

### 6.2 Delta Patch Format
(TODO: Describe DeltaOp {ADD | REPLACE | REMOVE}, segment targeting, and protobuf representation)

### 6.3 Subscription / Invalidation
(TODO: Describe how agents subscribe to segments, and how invalidations are propagated in peer topologies)

### 6.4 Persistence Requirements
(TODO: Define minimal persistence rules for edge nodes and optional caching at gateways)

---

## 7. Capability-Negotiation Layer

### 7.1 DIDComm v2 Handshake
(TODO: Specify fields used in initial capability handshake, including auth, encryption, and mutual features)

### 7.2 Capability Descriptor
(TODO: Define schema used to declare exposed functionality, parameters, and types)

### 7.3 Policy & Access Control
(TODO: Describe WASM policies applied at gateways to accept/reject invocation requests)

### 7.4 Error Handling
(TODO: Provide error codes and handling procedures for invalid offers, failed negotiation, or policy rejection)

---

## 9. Privacy & Confidential-Execution

### 9.1 SGX / Confidential-VM Envelope
(TODO: Define envelope fields for attestation reports, measurement hashes, and enclave identity)

### 9.2 Differential-Privacy Filter
(TODO: Specify filter schemas, privacy budgets, and token-based access)

### 9.3 Audit & Logging
(TODO: Describe tamper-resistant audit trails for envelope usage, including log formats and retention)

---
