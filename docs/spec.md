# AXCP Core – Technical Specification (Draft v0.3)

## Problem Statement

AXCP provides a transport-agnostic, delta-based coordination layer…

## Envelope schema

```protobuf
message AxcpEnvelope {
  uint32  version        = 1;
  string  trace_id       = 2;
  …
}
```

## Security Profiles

<inserisci tabella profili 0-2 definitiva>