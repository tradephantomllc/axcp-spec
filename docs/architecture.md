# AXCP Core – Architecture Overview

```
+-------------------+        QUIC/TLS         +-----------------------+
|       Agent A     |  ───────────────────▶   |     Gateway Core      |
|  (edge device)    |                         |  (cluster ingress)    |
+-------------------+                         +-----------┬-----------+
        ▲                                           ▲     │
        │                                           │     │ Gossip /
        │ QUIC/TLS                                   │     │ CRDT Store
        │                                           │     ▼
+-------------------+        QUIC/TLS         +-----------------------+
|       Agent B     |  ◀───────────────────    |   Agent Discovery    |
|  (edge device)    |                         |      Service         |
+-------------------+                         +-----------------------+
```

*Figure 1 – Minimal deployment with two agents and a single gateway*

## Component Roles (≤ 300 words)

**Agent** – A lightweight runtime embedded into applications or services. It exchanges *context deltas* and executes policy-driven actions based on the shared state. Agents maintain an in-memory CRDT store and expose a gRPC/HTTP control port.

**Gateway Core** – Acts as a rendez-vous and policy enforcement point. It terminates secure QUIC/TLS sessions from Agents, validates capability envelopes and forwards deltas according to routing rules (pub-sub, multicast, directed).

**Agent Discovery Service** – Optional component that provides a list of available Gateways and peer Agents via a gossip protocol. It persists CRDT shards for cold-start recovery.

**Gossip / CRDT Store** – A distributed key-value store (e.g., Redis CRDT or Litestream) that holds the replicated context graph. Gateways and Agents update the store asynchronously, enabling eventual consistency across the mesh.

Data flow: Agents serialise local state changes into **AxcpEnvelope** deltas. These are signed and transmitted over QUIC/TLS to the Gateway Core. The Gateway validates signatures, applies privacy budget checks and rebroadcasts relevant deltas to subscribed peers. Telemetry datagrams flow out-of-band over QUIC DATAGRAM frames to the monitoring back-end.
