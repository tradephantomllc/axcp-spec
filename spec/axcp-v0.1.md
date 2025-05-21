# AXCP v0.1 – Adaptive eXchange Context Protocol

_Work in progress – structure auto-generated._

> This document defines the initial AXCP protocol specification.  
> Version: **v0.1 Draft**  
> Status: **Exploratory Draft (Do not implement)**  
> Last updated: {{INSERT_DATE}}

---

## Table of Contents

## 1. Preface
AXCP (Adaptive eXchange Context Protocol) nasce dall’esigenza di orchestrare agenti AI distribuiti che devono:

* scambiarsi contesto in modo **delta-efficiente**, riducendo il token-overhead
* negoziare capacità e tool in maniera **decentralizzata** (zero vendor-lock-in)
* preservare **privacy** e fornire **verificabilità** del calcolo (enclave, attestazione)

AXCP v0.1 è una **Draft Exploratory**: fissa lessico, formati wire e flussi minimi per un Proof-of-Concept interoperabile.

### 1.1 Motivation
Protocollo | Limite attuale | Come AXCP lo supera
-----------|----------------|---------------------
**MCP** (Anthropic) | Single-vendor stack | Multi-vendor & edge-aware  
**A2A** (OpenAI) | Niente delta-context | Delta-synced cache  
**ACP** | JSON verboso | QUIC + Protobuf binario  

### 1.2 Status of This Document
* **v0.1 Draft** – NON usare in produzione.  
* Cambiamenti sostanziali possibili fino alla tag `v0.1-rc`.

### 1.3 Conventions Used
* “MUST/SHOULD/MAY” secondo [RFC 2119].  
* Diagrammi sequenza in **mermaid**.  
* Tipi Protobuf in `CamelCase`, campi in `snake_case`.

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

---

## 3. Terminology
Termine | Definizione
------- | -----------
**Node** | Processo che parla AXCP (edge, cloud, gateway)  
**Agent** | Modulo logico che esegue un task (p.es. “QA-bot”)  
**Tool** | Funzione invocabile dall’agente (p.es. HTTP GET)  
**Context Segment** | Oggetto JSON versione-ato contenente stato/dati  
**Delta Patch** | Serie di `DeltaOp {ADD | REPLACE | REMOVE}`  
**Capability** | Funzionalità dichiarata da un nodo (“search”)  
**Envelope** | Struttura Protobuf di trasporto (`AxcpEnvelope`)  
**Gateway** | Nodo che fa routing edge/cloud e policy

*(Glossary esteso a fine documento)*


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
