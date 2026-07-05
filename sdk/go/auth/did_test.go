package auth

import (
	"context"
	"testing"
	"time"
)

// FakeResolver is a mock DID resolver for testing.
// It returns predefined documents for known DIDs.
type FakeResolver struct {
	Documents map[string]*DIDDocument
}

// Resolve implements DIDResolver for FakeResolver.
func (f *FakeResolver) Resolve(ctx context.Context, did string) (*DIDDocument, error) {
	if doc, ok := f.Documents[did]; ok {
		return doc, nil
	}
	return nil, ErrDIDNotFound
}

// --- DID Parsing Tests ---

func TestParseDID_Valid(t *testing.T) {
	tests := []struct {
		input  string
		method string
		id     string
	}{
		{"did:example:alice", "example", "alice"},
		{"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK", "key", "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"},
		{"did:web:example.com", "web", "example.com"},
		{"did:ethr:0x1234567890abcdef", "ethr", "0x1234567890abcdef"},
		{"did:ion:test:EiBDr5SrxvZ-Yp7UJ3VdKfXJ", "ion", "test:EiBDr5SrxvZ-Yp7UJ3VdKfXJ"}, // ID can contain colons
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			did, err := ParseDID(tt.input)
			if err != nil {
				t.Fatalf("ParseDID(%q) failed: %v", tt.input, err)
			}

			if did.String() != tt.input {
				t.Errorf("String() = %q, want %q", did.String(), tt.input)
			}

			if did.Method() != tt.method {
				t.Errorf("Method() = %q, want %q", did.Method(), tt.method)
			}

			if did.ID() != tt.id {
				t.Errorf("ID() = %q, want %q", did.ID(), tt.id)
			}
		})
	}
}

func TestParseDID_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   error
	}{
		{"missing prefix", "alice", ErrInvalidDIDFormat},
		{"wrong prefix", "uri:example:alice", ErrInvalidDIDFormat},
		{"empty method", "did::alice", ErrDIDMethodEmpty},
		{"empty id", "did:example:", ErrDIDIDEmpty},
		{"no id part", "did:example", ErrInvalidDIDFormat},
		{"just did:", "did:", ErrInvalidDIDFormat},
		{"empty string", "", ErrInvalidDIDFormat},
		{"contains space", "did:example:alice bob", ErrDIDContainsInvalid},
		{"contains tab", "did:example:alice\tbob", ErrDIDContainsInvalid},
		{"contains newline", "did:example:alice\nbob", ErrDIDContainsInvalid},
		{"contains control char", "did:example:alice\x00bob", ErrDIDContainsInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDID(tt.input)
			if err != tt.err {
				t.Errorf("ParseDID(%q) error = %v, want %v", tt.input, err, tt.err)
			}
		})
	}
}

func TestResolveEd25519PublicKey_MissingResolver(t *testing.T) {
	_, err := ResolveEd25519PublicKey(context.Background(), nil, "did:example:alice")
	if err != ErrDIDResolverMissing {
		t.Fatalf("error = %v, want %v", err, ErrDIDResolverMissing)
	}
}

// --- Resolver Tests ---

func TestResolveEd25519PublicKey_Success(t *testing.T) {
	// Generate a real keypair
	publicKey, _, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "Ed25519VerificationKey2020",
						PublicKeyBytes: publicKey,
					},
				},
			},
		},
	}

	ctx := context.Background()
	resolvedKey, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:alice")
	if err != nil {
		t.Fatalf("ResolveEd25519PublicKey failed: %v", err)
	}

	if string(resolvedKey) != string(publicKey) {
		t.Error("resolved key does not match expected key")
	}
}

func TestResolveEd25519PublicKey_NotFound(t *testing.T) {
	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{},
	}

	ctx := context.Background()
	_, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:unknown")
	if err != ErrDIDNotFound {
		t.Errorf("expected ErrDIDNotFound, got %v", err)
	}
}

func TestResolveEd25519PublicKey_DIDMismatch(t *testing.T) {
	publicKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:bob", // Mismatch!
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "Ed25519VerificationKey2020",
						PublicKeyBytes: publicKey,
					},
				},
			},
		},
	}

	ctx := context.Background()
	_, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:alice")
	if err != ErrDIDMismatch {
		t.Errorf("expected ErrDIDMismatch, got %v", err)
	}
}

func TestResolveEd25519PublicKey_WrongKeyType(t *testing.T) {
	publicKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "RsaVerificationKey2018", // Not in allowlist
						PublicKeyBytes: publicKey,
					},
				},
			},
		},
	}

	ctx := context.Background()
	_, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:alice")
	if err != ErrNoAcceptableKey {
		t.Errorf("expected ErrNoAcceptableKey, got %v", err)
	}
}

func TestResolveEd25519PublicKey_WrongKeyLength(t *testing.T) {
	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "Ed25519VerificationKey2020",
						PublicKeyBytes: make([]byte, 16), // Wrong length!
					},
				},
			},
		},
	}

	ctx := context.Background()
	_, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:alice")
	if err != ErrNoAcceptableKey {
		t.Errorf("expected ErrNoAcceptableKey, got %v", err)
	}
}

func TestResolveEd25519PublicKey_MalformedDID(t *testing.T) {
	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{},
	}

	ctx := context.Background()
	_, err := ResolveEd25519PublicKey(ctx, resolver, "not-a-did")
	if err != ErrInvalidDIDFormat {
		t.Errorf("expected ErrInvalidDIDFormat, got %v", err)
	}
}

func TestResolveEd25519PublicKey_SelectsFirstValidKey(t *testing.T) {
	correctKey, _, _ := GenerateEd25519Keypair()
	wrongKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "RsaVerificationKey2018", // Wrong type, skip
						PublicKeyBytes: wrongKey,
					},
					{
						ID:             "key-2",
						Type:           "Ed25519VerificationKey2020",
						PublicKeyBytes: make([]byte, 16), // Wrong length, skip
					},
					{
						ID:             "key-3",
						Type:           "Ed25519VerificationKey2020", // Correct!
						PublicKeyBytes: correctKey,
					},
				},
			},
		},
	}

	ctx := context.Background()
	resolvedKey, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:alice")
	if err != nil {
		t.Fatalf("ResolveEd25519PublicKey failed: %v", err)
	}

	if string(resolvedKey) != string(correctKey) {
		t.Error("resolved wrong key")
	}
}

func TestResolveEd25519PublicKey_Accepts2018KeyType(t *testing.T) {
	publicKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "Ed25519VerificationKey2018", // 2018 version
						PublicKeyBytes: publicKey,
					},
				},
			},
		},
	}

	ctx := context.Background()
	resolvedKey, err := ResolveEd25519PublicKey(ctx, resolver, "did:example:alice")
	if err != nil {
		t.Fatalf("ResolveEd25519PublicKey failed: %v", err)
	}

	if string(resolvedKey) != string(publicKey) {
		t.Error("resolved key does not match expected key")
	}
}

// --- Transcript Tests ---

func TestBuildDIDAuthTranscript_Deterministic(t *testing.T) {
	clientDID := "did:example:client"
	serverDID := "did:example:server"
	signingPayload := []byte("canonical-signing-payload")
	ts := time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)

	transcript1 := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload, ts)
	transcript2 := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload, ts)

	if string(transcript1) != string(transcript2) {
		t.Error("transcripts should be deterministic")
	}
}

func TestBuildDIDAuthTranscript_Format(t *testing.T) {
	clientDID := "did:example:client"
	serverDID := "did:example:server"
	signingPayload := []byte("test")
	ts := time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)

	transcript := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload, ts)
	transcriptStr := string(transcript)

	// Expected format:
	// AXCP-DID-AUTH-v1\ndid:example:client\ndid:example:server\ndGVzdA==\n2026-01-27T12:00:00Z
	expected := "AXCP-DID-AUTH-v1\ndid:example:client\ndid:example:server\ndGVzdA==\n2026-01-27T12:00:00Z"

	if transcriptStr != expected {
		t.Errorf("transcript = %q, want %q", transcriptStr, expected)
	}
}

func TestBuildDIDAuthTranscript_DifferentInputsProduceDifferentTranscripts(t *testing.T) {
	ts := time.Now()
	signingPayload := []byte("canonical-signing-payload")

	t1 := BuildDIDAuthTranscript("did:example:a", "did:example:b", signingPayload, ts)
	t2 := BuildDIDAuthTranscript("did:example:c", "did:example:b", signingPayload, ts) // Different client

	if string(t1) == string(t2) {
		t.Error("different clients should produce different transcripts")
	}
}

// --- DID Auth Signature Verification Tests ---

func TestVerifyDIDAuthSignature_Success(t *testing.T) {
	// Generate keypair for alice
	publicKey, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	// Create resolver with alice's key
	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{
						ID:             "key-1",
						Type:           "Ed25519VerificationKey2020",
						PublicKeyBytes: publicKey,
					},
				},
			},
		},
	}

	// Build transcript
	clientDID := "did:example:alice"
	serverDID := "did:example:server"
	signingPayload := []byte("server-payload-12345")
	ts := time.Now()

	transcript := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload, ts)

	// Sign transcript with alice's private key
	signature, err := SignEd25519(privateKey, transcript)
	if err != nil {
		t.Fatalf("SignEd25519 failed: %v", err)
	}

	// Verify
	ctx := context.Background()
	err = VerifyDIDAuthSignature(ctx, resolver, clientDID, transcript, signature)
	if err != nil {
		t.Errorf("VerifyDIDAuthSignature failed: %v", err)
	}
}

func TestVerifyDIDAuthSignature_MalformedDID(t *testing.T) {
	resolver := &FakeResolver{Documents: map[string]*DIDDocument{}}
	ctx := context.Background()

	err := VerifyDIDAuthSignature(ctx, resolver, "not-a-did", []byte("transcript"), []byte("sig"))
	if err != ErrInvalidDIDFormat {
		t.Errorf("expected ErrInvalidDIDFormat, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_ResolverNotFound(t *testing.T) {
	resolver := &FakeResolver{Documents: map[string]*DIDDocument{}}
	ctx := context.Background()

	err := VerifyDIDAuthSignature(ctx, resolver, "did:example:unknown", []byte("transcript"), []byte("sig"))
	if err != ErrDIDNotFound {
		t.Errorf("expected ErrDIDNotFound, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_DIDMismatch(t *testing.T) {
	publicKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:bob", // Mismatch
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: publicKey},
				},
			},
		},
	}

	ctx := context.Background()
	err := VerifyDIDAuthSignature(ctx, resolver, "did:example:alice", []byte("transcript"), []byte("sig"))
	if err != ErrDIDMismatch {
		t.Errorf("expected ErrDIDMismatch, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_WrongKeyType(t *testing.T) {
	publicKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "RsaVerificationKey2018", PublicKeyBytes: publicKey},
				},
			},
		},
	}

	ctx := context.Background()
	err := VerifyDIDAuthSignature(ctx, resolver, "did:example:alice", []byte("transcript"), []byte("sig"))
	if err != ErrNoAcceptableKey {
		t.Errorf("expected ErrNoAcceptableKey, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_BadSignature(t *testing.T) {
	publicKey, privateKey, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: publicKey},
				},
			},
		},
	}

	// Sign different data
	signature, _ := SignEd25519(privateKey, []byte("different transcript"))

	ctx := context.Background()
	err := VerifyDIDAuthSignature(ctx, resolver, "did:example:alice", []byte("actual transcript"), signature)
	if err != ErrVerificationFailed {
		t.Errorf("expected ErrVerificationFailed, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_WrongSigner(t *testing.T) {
	// Alice's key in resolver
	alicePublic, _, _ := GenerateEd25519Keypair()
	// Bob signs with his key
	_, bobPrivate, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: alicePublic},
				},
			},
		},
	}

	transcript := []byte("some transcript")
	signature, _ := SignEd25519(bobPrivate, transcript) // Signed by Bob, not Alice

	ctx := context.Background()
	err := VerifyDIDAuthSignature(ctx, resolver, "did:example:alice", transcript, signature)
	if err != ErrVerificationFailed {
		t.Errorf("expected ErrVerificationFailed, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_PayloadBinding(t *testing.T) {
	// This test verifies that the signature is bound to the specific signing payload
	publicKey, privateKey, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: publicKey},
				},
			},
		},
	}

	clientDID := "did:example:alice"
	serverDID := "did:example:server"
	signingPayload1 := []byte("payload-1")
	signingPayload2 := []byte("payload-2")
	ts := time.Now()

	// Sign with signingPayload1
	transcript1 := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload1, ts)
	signature, _ := SignEd25519(privateKey, transcript1)

	// Verify with signingPayload1 - should succeed
	ctx := context.Background()
	err := VerifyDIDAuthSignature(ctx, resolver, clientDID, transcript1, signature)
	if err != nil {
		t.Errorf("verification with correct signing payload failed: %v", err)
	}

	// Verify with signingPayload2 - should fail (different transcript)
	transcript2 := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload2, ts)
	err = VerifyDIDAuthSignature(ctx, resolver, clientDID, transcript2, signature)
	if err != ErrVerificationFailed {
		t.Errorf("expected ErrVerificationFailed for wrong signing payload, got %v", err)
	}
}

func TestVerifyDIDAuthSignature_InvalidSignatureSize(t *testing.T) {
	publicKey, _, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: publicKey},
				},
			},
		},
	}

	ctx := context.Background()

	// Test various invalid signature sizes
	invalidSigs := [][]byte{
		nil,
		{},
		make([]byte, 32),  // Too short
		make([]byte, 128), // Too long
	}

	for _, sig := range invalidSigs {
		err := VerifyDIDAuthSignature(ctx, resolver, "did:example:alice", []byte("transcript"), sig)
		if err != ErrVerificationFailed {
			t.Errorf("expected ErrVerificationFailed for invalid sig size %d, got %v", len(sig), err)
		}
	}
}

// --- Integration Test: Full Auth Flow ---

func TestDIDAuth_FullFlow(t *testing.T) {
	// Setup: Generate keypairs for client and server
	clientPublic, clientPrivate, _ := GenerateEd25519Keypair()
	serverPublic, serverPrivate, _ := GenerateEd25519Keypair()

	// Both parties have their DIDs registered
	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:client": {
				ID: "did:example:client",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: clientPublic},
				},
			},
			"did:example:server": {
				ID: "did:example:server",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: serverPublic},
				},
			},
		},
	}

	ctx := context.Background()
	clientDID := "did:example:client"
	serverDID := "did:example:server"

	// Step 1: Build canonical signing payload bytes
	signingPayload := []byte("server-canonical-payload-xyz")
	ts := time.Now()

	// Step 2: Client builds and signs transcript
	clientTranscript := BuildDIDAuthTranscript(clientDID, serverDID, signingPayload, ts)
	clientSignature, err := SignEd25519(clientPrivate, clientTranscript)
	if err != nil {
		t.Fatalf("client signing failed: %v", err)
	}

	// Step 3: Server verifies client's signature
	err = VerifyDIDAuthSignature(ctx, resolver, clientDID, clientTranscript, clientSignature)
	if err != nil {
		t.Fatalf("server failed to verify client: %v", err)
	}

	// Step 4: Server responds with its own signature (mutual auth)
	serverTranscript := BuildDIDAuthTranscript(serverDID, clientDID, signingPayload, ts)
	serverSignature, err := SignEd25519(serverPrivate, serverTranscript)
	if err != nil {
		t.Fatalf("server signing failed: %v", err)
	}

	// Step 5: Client verifies server's signature
	err = VerifyDIDAuthSignature(ctx, resolver, serverDID, serverTranscript, serverSignature)
	if err != nil {
		t.Fatalf("client failed to verify server: %v", err)
	}

	// Both parties are now mutually authenticated
}

// --- Benchmark ---

func BenchmarkBuildDIDAuthTranscript(b *testing.B) {
	clientDID := "did:example:client"
	serverDID := "did:example:server"
	signingPayload := make([]byte, 32)
	ts := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildDIDAuthTranscript(clientDID, serverDID, signingPayload, ts)
	}
}

func BenchmarkVerifyDIDAuthSignature(b *testing.B) {
	publicKey, privateKey, _ := GenerateEd25519Keypair()

	resolver := &FakeResolver{
		Documents: map[string]*DIDDocument{
			"did:example:alice": {
				ID: "did:example:alice",
				PublicKeys: []PublicKeyRecord{
					{ID: "key-1", Type: "Ed25519VerificationKey2020", PublicKeyBytes: publicKey},
				},
			},
		},
	}

	transcript := BuildDIDAuthTranscript("did:example:alice", "did:example:server", []byte("signing payload"), time.Now())
	signature, _ := SignEd25519(privateKey, transcript)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyDIDAuthSignature(ctx, resolver, "did:example:alice", transcript, signature)
	}
}
