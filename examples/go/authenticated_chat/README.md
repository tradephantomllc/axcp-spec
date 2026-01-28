# Authenticated Chat Example

Demonstrates AXCP **Secure Baseline (Profile 1)** authentication with:
- DID-based identity
- Ed25519 signature generation and verification
- Local, in-memory DID resolution (fully offline)
- Bidirectional authenticated message exchange

## Features

- **Deterministic keypairs**: Uses fixed seeds for reproducible DIDs across runs
- **Shared DID resolver**: Both client and server use the same pre-populated resolver
- **No external dependencies**: Works completely offline without network DID resolution
- **Full auth flow**: Demonstrates signing, transmission, and verification

## Running the Example

### Terminal 1 - Start Server
```bash
cd examples/go/authenticated_chat
go run . -server
```

### Terminal 2 - Run Client
```bash
cd examples/go/authenticated_chat
go run .
```

## Expected Output

### Server
```
=== AXCP Authenticated Chat Server ===
Profile: secure-baseline-v1
DID Resolver initialized with 2 identities
Server DID: did:key:axcp-auth-chat-server
Listening on localhost:61301
Client connected

--- Received Envelope ---
  Version:    1
  TraceID:    auth-chat-001
  Profile:    1 (secure-baseline-v1)
  SenderDID:  did:key:axcp-auth-chat-client
  ...

--- Verifying Signature ---
  Signature VALID
...
Authentication flow completed successfully!
```

### Client
```
=== AXCP Authenticated Chat Client ===
Profile: secure-baseline-v1
Client DID: did:key:axcp-auth-chat-client
Server DID: did:key:axcp-auth-chat-server
...

--- Sending Authenticated Envelope ---
  TraceID:    auth-chat-001
  SenderDID:  did:key:axcp-auth-chat-client
  ...

--- Received Response ---
...
--- Verifying Server Signature ---
  Server Signature VALID

Bidirectional authenticated exchange completed!
```

## Auth Transcript Format

The signature is computed over a canonical transcript:
```
AXCP-DID-AUTH-v1
<sender_did>
<recipient_did>
<base64(payload_bytes)>
<timestamp_rfc3339>
```

## Related Files

- `sdk/go/auth/session.go` - Session management with signing
- `sdk/go/auth/did.go` - DID types and transcript building
- `sdk/go/auth/verifier.go` - Ed25519 signature verification
