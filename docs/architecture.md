# AXCP Architecture

```
Agent ─┐
       ├─ QUIC/TLS ─> Gateway Core ─┐
Agent ─┘                           │
                                   └─ Gossip Discovery / CRDT Store
```

Explain each component in max 300 words.