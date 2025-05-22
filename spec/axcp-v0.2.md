# AXCP v0.1 – Adaptive eXchange Context Protocol

© 2025 TradePhantom LLC – BSL 1.1 / Apache-2.0 fallback

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
   5.6 Profile negotiation & downgrade rules

1. **Capability announcement**  
   Every node sends two QUIC header fields in the first 1-RTT packet:  

   | Header                       | Type  | Meaning                              |
   |------------------------------|-------|--------------------------------------|
   | `axcp-supported-profiles`    | u8    | Bit-mask of profiles this node can **accept** (bit 0 = Profile-0 … bit 3 = Profile-3). |
   | `axcp-required-profile`      | u8    | Single value (0-3) the sender **requests** for this session. |

2. **Agreement algorithm**

   * If `required` ∉ `supported` of the peer ⇒ connection **fails** (QUIC error `axcp.profile.unsupported`).  
   * Else the session profile = `max( required_A , required_B )`.  
   * Nodes MAY **downgrade** later (e.g. to save battery). A `ProfileDowngrade` frame is exchanged; the lower level must still be ≥ both nodes’ *minimum*.

3. **Envelope validation**

   Each `AxcpEnvelope.profile` **MUST** ≤ session-profile.  
   *If higher* → reply with `ErrorMessage{ code = PROFILE_MISMATCH }`.

```mermaid
sequenceDiagram
    participant Edge
    participant Cloud
    Edge->>Cloud: QUIC Initial  (supported=0b1111, required=2)
    Cloud-->>Edge: QUIC Accept  (session-profile = 2)
    Note over Edge,Cloud: encrypted 1-RTT traffic
    Edge->>Cloud: AxcpEnvelope(profile=2,…)
    Cloud->>Edge: AxcpEnvelope(profile=1,…)
    ## 5.7 Capability Handshake – Dynamic Profile Negotiation

> _Status: v0.2-dev_

AXCP peers now agree on a session profile through an explicit two-step exchange
carried inside the 1-RTT QUIC channel, replacing the static header of v0.1.

### 5.7.1 Messages

| Message | Purpose | Field | Type | Notes |
|---------|---------|-------|------|-------|
| `CapabilityNegotiate` | Advertise supported profiles & minimum required | `supported_mask` | u8 | Bit-mask, bit 0 = Profile-0 … bit 3 = Profile-3. |
|  |  | `min_required` | u8 | Lowest acceptable profile. |
| `CapabilityAck` | Confirm final agreed level | `agreed_profile` | u8 | 0–3 |

### 5.7.2 Algorithm

1. Each side sends `CapabilityNegotiate`.
2. Let `intersection = maskA ∧ maskB`.  
   Let `required  = max(minA, minB)`.
3. If `intersection == 0` **OR** `highest(intersection) < required` → terminate (`PROFILE_NEGOTIATION_FAILED`).
4. Otherwise `session_profile = highest(intersection)`.
5. Both sides send `CapabilityAck(session_profile)`; subsequent envelopes **MUST** carry `profile ≤ session_profile`.

```mermaid
sequenceDiagram
  participant A
  participant B
  A->>B: CapabilityNegotiate{mask=0b0111,min=1}
  B-->>A: CapabilityNegotiate{mask=0b1011,min=0}
  Note over A,B: intersection = 0b0011 → chosen = 2
  B-->>A: CapabilityAck(2)
  A-->>B: CapabilityAck(2)

  ## 5.8 QUIC DATAGRAM Telemetry (optional)

Certain AXCP payloads are *loss-tolerant* (e.g. heartbeat, CPU temp, token-usage
counters).  For these the reliability of STREAM frames is wasteful.

AXCP therefore reserves QUIC DATAGRAM ID `0xA0` for **TelemetryDatagram**.

### 5.8.1 Encoding

+---------+------------+--------------+
| 1 byte | 2 bytes | … N bytes |
| type | seq (u16) | protobuf TLV |
+---------+------------+--------------+


* **type** – fixed `0xA0`  
* **seq**  – wrap-around sequence; receiver MAY ignore gaps  
* **body** – `TelemetryDatagram` (see proto)

### 5.8.2 Send rules

* MUST fit in a single QUIC DATAGRAM (max 1350 B typical).  
* Sender does **not** retransmit; receiver treats missing seq as normal loss.  
* Congestion control follows QUIC DATAGRAM RFC 9221.

### 5.8.3 Examples

```mermaid
sequenceDiagram
  Edge->>Cloud: DATAGRAM(type=A0, seq=17, cpu=42%, mem=310MB)
  Cloud--x Edge: (lost seq 18)
  Edge->>Cloud: DATAGRAM(seq=19, tokens=1500)



## 6. Context-Sync Layer

### 6.1 Versioned Context Graph

### 6.1 Versioned Context Graph

AXCP models shared knowledge as a directed **Context Graph**.  
Each node is a *Context Segment* (JSON object, max 64 KiB).  
Edges are causal links (`parent_id`).  A graph version is identified by
`<context_id>:<uint64 version>`.

### 6.2 Delta Patch Format (CRDT-inspired)

A patch is an **ordered list of `DeltaOp`**:

| Field | Type | Meaning |
|-------|------|---------|
| `op`      | enum {ADD, REPLACE, REMOVE, MERGE} | `MERGE` is new: CRDT LWW merge |
| `path`    | JSON-Pointer | segment/key affected |
| `data`    | bytes (gz)   | payload or CRDT payload |
| `ts`      | uint64 micros | Lamport timestamp |

Conflict resolution: **last-writer-wins** by `(ts, node_id)`.

### 6.3 Subscription / Invalidation

Nodes send `SyncSubscribe{context_id, from_version}` once per session.  
Updates flow as `ContextPatch`.  
Missing ranges → receiver sends `SyncRequest{missing_from, to}`.

### 6.4 Store-and-Forward (offline edge)

If a peer is offline, patches are buffered in **RetryEnvelope**
(see §8.3) with `ttl_ms`. Upon reconnection, buffered patches replay
in order; expired TTL patches are dropped.

```mermaid
sequenceDiagram
  Edge->>Cloud: SyncSubscribe(ctx=chat, v=42)
  Cloud-->>Edge: ContextPatch(v=43..45)
  Edge--x Cloud: offline
  Cloud--)Cloud: buffer patches 46..48
  Edge-)Cloud: reconnect
  Cloud-->>Edge: RetryEnvelope{patch46..48, ttl_ok}


Each AXCP node maintains a directed acyclic graph (DAG) of context segments.  
Each segment is uniquely identified by its `segment_id` and versioned via `context_version` fields.

Nodes MUST implement a versioning model that supports:
- Linear history per segment (e.g., `/user/status`)
- Optional DAG lineage for merged updates (e.g., `/shared/intent`)

Version IDs are strictly monotonic and MUST include a timestamp and author node hash.

### 6.2 Delta Patch Format

Updates between peers are exchanged as `DeltaPatch` messages, which contain a list of atomic `DeltaOp` entries.

Each operation follows this schema:

```json
{ "op": "replace", "path": "/user/intent", "value": "translate" }

Supported operations:

add (insert field or object)
replace (overwrite existing field)
remove (delete path)
Fields MAY be compressed using aliases and encoded as CBOR or binary protobuf in constrained environments.

6.3 Subscription & Invalidation
Agents MAY subscribe to segments using filter queries. Supported filter types include:

prefix=/user/ → all personal context
tag=intent → semantic category
timestamp > T → updates after T
Invalidated segments (e.g. revoked or expired) MUST trigger a ContextInvalidation event.

6.4 Persistence Requirements
Every AXCP node MUST persist its active context graph between sessions. Minimal persistence features:

snapshot export (JSON or protobuf)
journal replay (optional)
recovery mode on restart
Gateways MAY cache selected segments or act as authoritative stores for lightweight edge nodes.

## 7. Capability-Negotiation Layer

### 7.1 DIDComm v2 Handshake

Nodes initiate secure capability negotiation using DIDComm v2.  
Each peer exchanges a signed `CapabilityOffer` listing its available tools, constraints, and supported features.

The handshake includes:
- Sender’s DID + public key
- Timestamp and optional session UUID
- Signed list of supported capabilities

Handshake payloads are encoded as signed JSON-LD and transported via envelope headers.

### 7.2 Capability Descriptor

Each tool/function must be described using a standardized descriptor object.  
Fields include:

- `tool_id`: short identifier (e.g., `search`, `summarize`)
- `input_schema`: JSON schema describing expected input
- `output_schema`: JSON schema for tool responses
- `timeout_ms`: optional maximum execution time
- `resource_hint`: (e.g., `low-latency`, `gpu`, `secure-env`)
- `auth_scope`: access requirements (e.g., `read:user`, `admin:tasks`)

Descriptors MAY be versioned (`descriptor_version`) and signed individually.

### 7.3 Policy & Access Control

Gateways and agents MUST enforce capability access policies.

Supported enforcement methods:
- ACL-based: static allow/deny lists
- WASM policy engine: dynamic decision logic
- Auth tokens: scoped per capability group

Policies are declared using `RoutePolicyMessage` and validated during tool invocation.

### 7.4 Error Handling

If a capability is rejected or not found, the responder MUST reply with an `ErrorMessage`.

Relevant error codes:
- `TOOL_NOT_FOUND`
- `UNAUTHORIZED`
- `TIMEOUT`
- `MALFORMED_REQUEST`

All responses MUST include a structured `ErrorCode` enum and optional human-readable message.


8. Orchestration Layer  
   8.1 Route Policy Language  
   8.2 Edge / Cloud Decision Matrix  
   8.3 Fallback & Retry

## 9. Privacy & Confidential Execution

### 9.1 SGX / Confidential-VM Envelope

AXCP supports optional secure execution environments (SEE), including:

- Intel SGX
- AMD SEV
- GCP Confidential VM
- AWS Nitro Enclaves

Nodes MAY wrap tool execution in a secure enclave. Enclaves MUST support:

- Code attestation (e.g., quote signed by Intel/AWS)
- Encrypted input/output buffers
- Remote verification of identity and integrity

The `AxcpEnvelope` may include an `attestation_proof` field for runtime validation.

### 9.2 Differential Privacy Filter

Nodes MAY enable differential privacy (DP) when handling context data.  
The filter operates on outbound payloads and applies randomized noise based on:

- `ε` (epsilon): privacy budget
- `δ` (delta): confidence threshold
- Output sensitivity class (e.g., exact count vs. mean estimate)

The DP module MUST be declared in capability metadata and MUST be tunable per session.

### 9.2.1 Differential-Privacy Parameter Block

Each node MAY advertise a **DP parameter set** that governs how noise is added
before sharing aggregated results.

| Field | Description | Range / Notes |
|-------|-------------|---------------|
| `epsilon`  | Privacy budget (ε) | 0.1 – 10.0 (lower = stronger privacy) |
| `delta`    | Failure prob (δ)   | ≤ 1 e-5 |
| `mechanism`| `LAPLACE` \| `GAUSSIAN` | MUST be identical both sides |
| `clip_norm`| L2 clipping bound | 0 = no clipping |
| `granularity` | Unit applied (RECORD, BATCH, TOKEN) | affects noise scale |

Nodes embed this block in `CapabilityOffer` → receiver **MUST** echo in the
`CapabilityAck`.  If two nodes disagree on ε/δ they MUST down-shift to the
higher privacy (min ε, min δ) or abort with `ErrorCode.DP_POLICY_CONFLICT`.

```mermaid
sequenceDiagram
  Edge->>Cloud: CapabilityOffer{dp: ε=1.0,δ=1e-5}
  Cloud-->>Edge: CapabilityAck same ε/δ
  Edge->>Cloud: AxcpEnvelope{profile=3, dp=true}

  Implementation note: noise scale = clip_norm / ε (Laplace) or
σ = sqrt(2*ln(1.25/δ)) * clip_norm / ε (Gaussian).


### 9.3 Audit & Logging

AXCP nodes MAY log tool executions, errors, and envelope flow. Logs MAY include:

- Timestamp, sender/receiver IDs
- Envelope hash (SHA-256)
- Tool invoked and outcome (e.g., `ok`, `timeout`, `fail`)
- Error codes if applicable

Logs MUST be append-only and verifiable (Merkle tree or hash chain).  
Nodes MAY expose a `LogProof` query to third parties for audit purposes.


10. Message Types (IDL reference)  
    10.1 `AxcpEnvelope`  
    10.2 `ContextPatch`  
    10.3 `CapabilityOffer / Request`  
    10.4 `RoutePolicy`

11. Error Codes  
12. Security Considerations  
13. IANA / Registry Considerations (reserved type IDs)  
14. Change Log  
### v0.1-draft – 2025-05-22

- Initial public draft with:
  - QUIC + protobuf transport framing
  - Profile-based negotiation (0–3 levels)
  - Context-sync layer with DeltaOps
  - Capability descriptors & request model
  - Enclave & privacy foundations

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
