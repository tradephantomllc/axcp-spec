# AXCP Core – Technical Specification (Draft v0.3)

## Problem Statement

AXCP (Adaptive eXchange Context Protocol) provides a transport-agnostic, delta-based coordination layer for distributed AI agent systems. It enables efficient context synchronization, capability negotiation, and telemetry collection across heterogeneous network environments.

### Key Challenges Addressed

- **Context Synchronization**: Efficient delta-based updates between agents
- **Capability Discovery**: Dynamic discovery and negotiation of agent capabilities
- **Privacy-Preserving Telemetry**: Differential privacy mechanisms for sensitive data
- **Transport Flexibility**: Support for QUIC, WebSockets, and other transport protocols
- **Multi-Language Support**: Consistent API across Go, Python, and Rust implementations

## Envelope Schema

The core message format uses Protocol Buffers for efficient serialization:

```protobuf
message AxcpEnvelope {
  uint32  version        = 1;  // Protocol version
  string  trace_id       = 2;  // Distributed tracing ID
  uint32  profile        = 3;  // Security/privacy profile
  
  oneof payload {
    ContextPatch         context_patch    = 4;
    CapabilityMessage    capability_msg   = 5;
    RoutePolicyMessage   route_msg        = 6;
    ErrorMessage         error            = 7;
    ProfileNegotiate     profile_neg      = 8;
    ProfileAck          profile_ack       = 9;
    RetryEnvelope       retry_env         = 10;
    TelemetryDatagram   telemetry         = 11;
  }
}
```

### Context Patches

Context updates use delta-based patches for efficiency:

```protobuf
message ContextPatch {
  repeated DeltaOp operations = 1;
  uint64 sequence_number = 2;
  string checkpoint_hash = 3;
}

message DeltaOp {
  enum OpType {
    INSERT = 0;
    UPDATE = 1;
    DELETE = 2;
  }
  
  OpType op_type = 1;
  string path = 2;
  bytes value = 3;
  uint64 timestamp = 4;
}
```

### Capability Messages

Agents discover and negotiate capabilities dynamically:

```protobuf
message CapabilityMessage {
  oneof msg_type {
    CapabilityOffer   offer   = 1;
    CapabilityRequest request = 2;
    CapabilityAck     ack     = 3;
  }
}

message CapabilityDescriptor {
  string name = 1;
  string version = 2;
  repeated string parameters = 3;
  map<string, string> metadata = 4;
}
```

## Security Profiles

AXCP supports multiple security profiles to balance privacy and performance:

| Profile | Description | Privacy Level | Performance |
|---------|-------------|---------------|-------------|
| **0** | Basic functionality, no privacy guarantees | None | High |
| **1** | Basic telemetry with minimal noise | Low | Medium |
| **2** | Enhanced privacy with Laplace noise | Medium | Medium |
| **3+** | Strong differential privacy with Gaussian noise | High | Lower |

### Profile Negotiation

Agents negotiate the highest mutually supported profile:

```protobuf
message ProfileNegotiate {
  repeated uint32 supported_profiles = 1;
  uint32 preferred_profile = 2;
  map<string, string> capabilities = 3;
}

message ProfileAck {
  uint32 selected_profile = 1;
  bool negotiation_success = 2;
  string error_message = 3;
}
```

## Telemetry System

### Telemetry Datagram

```protobuf
message TelemetryDatagram {
  uint64 timestamp_ms = 1;
  
  oneof payload {
    SystemStats system = 2;
    TokenUsage tokens = 3;
  }
}

message SystemStats {
  uint32 cpu_percent = 1;
  uint64 mem_bytes = 2;
  uint32 temperature_c = 3;
}

message TokenUsage {
  uint32 prompt_tokens = 1;
  uint32 completion_tokens = 2;
}
```

### Differential Privacy

For profiles 2+, telemetry data is protected using differential privacy:

```protobuf
message DpParams {
  double epsilon = 1;      // Privacy budget
  double delta = 2;        // Failure probability
  DpMechanism mechanism = 3;
}

enum DpMechanism {
  LAPLACE = 0;
  GAUSSIAN = 1;
}
```

## Error Handling

Standardized error codes and messages:

```protobuf
message ErrorMessage {
  ErrorCode code = 1;
  string message = 2;
  map<string, string> details = 3;
}

enum ErrorCode {
  UNKNOWN = 0;
  INVALID_CONTEXT = 1;
  UNAUTHORIZED = 2;
  TOOL_NOT_FOUND = 3;
  TIMEOUT = 4;
  UNSUPPORTED_VERSION = 5;
  BAD_DELTA = 6;
  PAYLOAD_TOO_LARGE = 7;
  MALFORMED_REQUEST = 8;
  TOO_MANY_REQUESTS = 9;
  PROFILE_MISMATCH = 12;
  PROFILE_UNSUPPORTED = 13;
  PROFILE_NEGOTIATION_FAILED = 14;
  MISSING_PATCH_RANGE = 15;
  DP_POLICY_CONFLICT = 16;
}
```

## Transport Layer

AXCP is transport-agnostic but optimized for:

- **QUIC**: Primary transport for performance and reliability
- **WebSockets**: Web-based clients and real-time applications
- **HTTP/2**: RESTful APIs and simple request-response patterns

### QUIC Integration

- **Streams**: Multiplexed message delivery
- **Datagrams**: Low-latency telemetry
- **Connection Migration**: Seamless network transitions

## Implementation Notes

### Multi-Language Support

- **Go**: Primary implementation with full feature set
- **Python**: Client library with asyncio support
- **Rust**: High-performance client with tokio integration

### Performance Considerations

- **Message Batching**: Aggregate small messages for efficiency
- **Compression**: Protocol Buffer compression for large payloads
- **Connection Pooling**: Reuse connections across requests

### Versioning

- **Semantic Versioning**: Major.Minor.Patch for protocol versions
- **Backward Compatibility**: Maintained within major versions
- **Migration Path**: Clear upgrade procedures between versions

## Future Enhancements

### Planned Features (v0.4+)

- **Signed Bundle Exchange**: Cryptographically signed capability bundles
- **Adaptive Encryption**: Dynamic encryption based on data sensitivity
- **OTEL Streaming**: Integration with OpenTelemetry for observability
- **HIPAA/GDPR Compliance**: Enhanced privacy controls for regulated environments

### NEXCP Convergence

AXCP will evolve toward convergence with NEXCP (Next Exchange Context Protocol), ensuring broader ecosystem compatibility while maintaining core design principles.

## References

- [AXCP Architecture](architecture.md)
- [Quick Start Guide](quickstart.md)
- [Contributing Guidelines](../CONTRIBUTING.md)
- [Project Roadmap](../ROADMAP.md)