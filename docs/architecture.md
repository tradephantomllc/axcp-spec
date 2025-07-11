# AXCP Reference Architecture

```
         Agent A ─┐                     
                  │ QUIC/TLS            
         Agent B ─┤─────────────┐       
                  ▼             │       
              Gateway Core ─────┤       
                  │             │       
                  ▼             │       
           Discovery Service ◀──┘       
                  │                     
                  ▼                     
        CRDT/State Store (Gossip)
```

**Component overview (≤ 300 words)**

* **Agents**: Lightweight application or edge processes embedding the AXCP SDK. They emit context deltas, telemetry datagrams, and handle gateway control frames.
* **Gateway Core**: Terminates QUIC connections from agents, enforces rate limits & privacy budgets, validates envelopes, and forwards authorised traffic across security domains.
* **Discovery Service**: Simple DNS-SD or in-mesh gossip layer that advertises reachable gateways and their capabilities. Optional in single-gateway deployments.
* **State Store / CRDT**: Conflict-free replicated data store used by gateways to share enrolment state, capability descriptors, and topology changes with eventual consistency.

All components are container-friendly and can run at the edge or in the cloud. Communication defaults to QUIC + TLS 1.3; when DATAGRAM frames are available, AXCP piggybacks envelope payloads to minimise head-of-line blocking.
