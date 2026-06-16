# AXCP Gateway Setup

This document describes how to configure the AXCP gateway for **Secure Baseline (`secure-baseline-v1`)** enforcement.

## What "Secure Baseline Enforced" Means

When the gateway is configured with `secure-baseline-v1`:

1. **All envelopes must be authenticated** - Missing signatures are rejected
2. **All signatures must be valid** - Invalid signatures are rejected
3. **DID resolution must succeed** - Unknown DIDs are rejected
4. **Replay protection is active** - Duplicate sequences are rejected
5. **No silent downgrade** - The gateway will not fall back to `transport-only-v0`

## Gateway Configuration

### ServerConfig

The gateway is configured via `ServerConfig` in `edge/gateway/internal/quic_server.go`:

```go
type ServerConfig struct {
    Addr          string                    // Listen address (e.g., ":4433")
    TLSConf       *tls.Config               // TLS configuration for QUIC
    Profile       negotiate.Profile         // Security profile (e.g., ProfileSecureBaseline)
    Authenticator *EnvelopeAuthenticator    // Authentication handler
}
```

### EnvelopeAuthenticator

The authenticator is created with a DID resolver:

```go
authenticator, err := NewEnvelopeAuthenticator(
    resolver,      // DIDResolver implementation
    serverDID,     // Server's DID (e.g., "did:key:my-gateway")
    authConfig,    // AuthConfig with TTL and window
    clock,         // Clock for time operations (use nil for real time)
)
```

### AuthConfig

```go
type AuthConfig struct {
    ReplayTTL       time.Duration  // TTL for replay protection entries
    ReplayWindow    uint64         // Sliding window size
    MaxTimestampAge time.Duration  // Maximum age for message timestamps
}
```

## Injecting the DID Resolver

### Development: In-Memory Resolver

For local development and testing, use `MemoryDIDResolver`:

```go
import "github.com/tradephantomllc/axcp-spec/sdk/go/auth"

// Create resolver
resolver := auth.NewMemoryDIDResolver()

// Add known DIDs
resolver.AddDID("did:key:client-1", clientPublicKey)
resolver.AddDID("did:key:client-2", client2PublicKey)

// Create authenticator
authenticator, err := NewEnvelopeAuthenticator(
    resolver,
    "did:key:my-gateway",
    AuthConfig{
        ReplayTTL:       5 * time.Minute,
        ReplayWindow:    1000,
        MaxTimestampAge: 30 * time.Second,
    },
    nil, // Use real clock
)
```

### Production: Adapter Pattern

For production, implement the `DIDResolver` interface with your DID infrastructure:

```go
type ProductionDIDResolver struct {
    // Your DID registry client
}

func (r *ProductionDIDResolver) Resolve(ctx context.Context, did string) (*auth.DIDDocument, error) {
    // Query your DID registry
    // Return DIDDocument with public keys
}
```

## Rejection Reasons

When `secure-baseline-v1` is enforced, the gateway rejects envelopes for these reasons:

| Rejection | Error Code | Description |
|-----------|------------|-------------|
| Missing signature | `UNAUTHORIZED` | Envelope has no signature field |
| Missing sender DID | `UNAUTHORIZED` | Envelope has no sender_did field |
| DID resolution failed | `UNAUTHORIZED` | Sender's DID cannot be resolved |
| Invalid signature | `UNAUTHORIZED` | Ed25519 signature verification failed |
| Replay detected | `UNAUTHORIZED` | Sequence was already seen within TTL |
| Sequence too old | `UNAUTHORIZED` | Sequence is outside sliding window |
| Timestamp too old | `UNAUTHORIZED` | Message timestamp exceeds MaxTimestampAge |
| No authenticator | `UNAUTHORIZED` | Secure profile but no authenticator configured |

## Operational Behavior

### Logging

The gateway logs authentication events at INFO level:

```
[quic] authenticated server listening on :4433 (profile: secure-baseline-v1)
[quic] auth failed for envelope trace_id=xxx: signature verification failed
```

**Security note**: The gateway does not log sensitive information such as signatures, private keys, or full DID documents.

### Error Responses

When authentication fails, the gateway sends an error response with:
- Error code (e.g., `UNAUTHORIZED`)
- Error message (high-level, non-sensitive)

The client connection is not immediately closed, allowing for protocol-level error handling.

## Development Walkthrough

### 1. Start the Gateway

The gateway can be started programmatically or via the example:

```go
// Create configuration
config := ServerConfig{
    Addr:          ":4433",
    TLSConf:       tlsConfig,
    Profile:       negotiate.ProfileSecureBaseline,
    Authenticator: authenticator,
}

// Run authenticated server
err := RunAuthenticatedQuicServer(config, envelopeHandler, telemetryHandler)
```

For the `edge/gateway/cmd/gateway` binary, production startup must provide a
persistent server TLS keypair:

```bash
cd edge/gateway
go run ./cmd/gateway \
  -tls-cert /etc/axcp/server.crt \
  -tls-key /etc/axcp/server.key \
  -secure-baseline \
  -server-did did:key:server \
  -trusted-did did:key:agent=<base64-public-key>
```

Local demos that intentionally use an ephemeral self-signed certificate must opt
in explicitly with `-allow-insecure-demo-tls`.

### 2. Run Authenticated Client

Use the authenticated_chat example as a client:

```bash
# Terminal 1: Start server (acts as gateway)
cd examples/go/authenticated_chat
go run . -server

# Terminal 2: Run client
cd examples/go/authenticated_chat
go run .
```

The example demonstrates the full authentication flow with `secure-baseline-v1`.

## Code References

| Component | Location |
|-----------|----------|
| QUIC server with auth | `edge/gateway/internal/quic_server.go` |
| Envelope authenticator | `edge/gateway/internal/auth.go` |
| DID resolver interface | `sdk/go/auth/did.go` |
| Memory resolver | `sdk/go/auth/session.go` |
| Profile definitions | `sdk/go/negotiate/profile.go` |

## Related Documentation

- [Authentication](authentication.md) - DID model and Ed25519 signing
- [Getting Started](getting-started.md) - Quick start guide
- [Specification v1.0](../spec/axcp-v1.0.md) - Full protocol specification
