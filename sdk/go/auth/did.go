package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

// DID errors
var (
	ErrInvalidDIDFormat   = errors.New("auth: invalid DID format, expected did:<method>:<id>")
	ErrDIDMethodEmpty     = errors.New("auth: DID method cannot be empty")
	ErrDIDIDEmpty         = errors.New("auth: DID identifier cannot be empty")
	ErrDIDContainsInvalid = errors.New("auth: DID contains invalid characters (spaces or control chars)")
	ErrDIDTooLong         = errors.New("auth: DID exceeds maximum length")
	ErrDIDNotFound        = errors.New("auth: DID not found by resolver")
	ErrDIDMismatch        = errors.New("auth: resolved document ID does not match requested DID")
	ErrNoAcceptableKey    = errors.New("auth: no acceptable Ed25519 public key found in DID document")
	ErrInvalidKeyType     = errors.New("auth: key type not in allowlist")
	ErrInvalidKeyLength   = errors.New("auth: public key length does not match Ed25519 key size")
	ErrVerificationFailed = errors.New("auth: DID auth signature verification failed")
	ErrDIDResolverMissing = errors.New("auth: DID resolver is not configured")
)

// Allowed Ed25519 key types for DID documents
var allowedEd25519KeyTypes = map[string]bool{
	"Ed25519VerificationKey2020": true,
	"Ed25519VerificationKey2018": true,
}

// DID represents a Decentralized Identifier string.
// Format: did:<method>:<id>
type DID string

// ParseDID validates and returns a DID from a string.
//
// The syntax follows the W3C DID Core ABNF without path, query or fragment:
//
//	did                = "did:" method-name ":" method-specific-id
//	method-name        = 1*( %x61-7A / DIGIT )
//	method-specific-id = *( *idchar ":" ) 1*idchar
//	idchar             = ALPHA / DIGIT / "." / "-" / "_" / pct-encoded
//
// Inputs longer than MaxDIDLength bytes are rejected before any other work.
func ParseDID(s string) (DID, error) {
	if len(s) > MaxDIDLength {
		return "", ErrDIDTooLong
	}
	if !strings.HasPrefix(s, "did:") {
		for i := 0; i < len(s); i++ {
			if c := s[i]; c <= 0x20 || c >= 0x7f {
				return "", ErrDIDContainsInvalid
			}
		}
		return "", ErrInvalidDIDFormat
	}
	rest := s[len("did:"):]
	sep := strings.IndexByte(rest, ':')
	if sep < 0 {
		return "", ErrInvalidDIDFormat
	}
	method, id := rest[:sep], rest[sep+1:]
	if method == "" {
		return "", ErrDIDMethodEmpty
	}
	for i := 0; i < len(method); i++ {
		c := method[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			if c <= 0x20 || c >= 0x7f {
				return "", ErrDIDContainsInvalid
			}
			return "", ErrInvalidDIDFormat
		}
	}
	if id == "" {
		return "", ErrDIDIDEmpty
	}
	if id[len(id)-1] == ':' {
		return "", ErrInvalidDIDFormat
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '.', c == '-', c == '_', c == ':':
		case c == '%':
			if i+2 >= len(id) || !isHexDigit(id[i+1]) || !isHexDigit(id[i+2]) {
				return "", ErrInvalidDIDFormat
			}
			i += 2
		case c <= 0x20 || c >= 0x7f:
			return "", ErrDIDContainsInvalid
		default:
			return "", ErrInvalidDIDFormat
		}
	}
	return DID(s), nil
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// String returns the DID as a string.
func (d DID) String() string {
	return string(d)
}

// Method returns the DID method component.
func (d DID) Method() string {
	parts := strings.SplitN(string(d), ":", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// ID returns the DID identifier component.
func (d DID) ID() string {
	parts := strings.SplitN(string(d), ":", 3)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// PublicKeyRecord represents a public key in a DID document.
type PublicKeyRecord struct {
	// ID is the key identifier or fragment (e.g., "key-1" or "#keys-1")
	ID string

	// Type specifies the key type (e.g., "Ed25519VerificationKey2020")
	Type string

	// PublicKeyBytes contains the raw public key bytes
	PublicKeyBytes []byte
}

// DIDDocument represents a minimal DID document structure.
type DIDDocument struct {
	// ID is the DID that this document describes
	ID string

	// PublicKeys contains the verification keys for this DID
	PublicKeys []PublicKeyRecord
}

// DIDResolver is an interface for resolving DIDs to DID documents.
// Implementations may be network-based or mock/fake for testing.
// Core does not provide a network implementation.
type DIDResolver interface {
	// Resolve looks up a DID and returns its document.
	// Returns ErrDIDNotFound if the DID cannot be resolved.
	Resolve(ctx context.Context, did string) (*DIDDocument, error)
}

// ResolveEd25519PublicKey resolves a DID and extracts the first acceptable
// Ed25519 public key from the document.
//
// Validates:
//   - DID parses correctly
//   - Resolver returns a document
//   - Document ID matches the requested DID exactly
//   - At least one key has an allowed Ed25519 type
//   - The key length matches ed25519.PublicKeySize (32 bytes)
func ResolveEd25519PublicKey(ctx context.Context, r DIDResolver, did string) (ed25519.PublicKey, error) {
	if r == nil {
		return nil, ErrDIDResolverMissing
	}

	// Parse and validate DID format
	parsedDID, err := ParseDID(did)
	if err != nil {
		return nil, err
	}

	// Resolve the DID document
	doc, err := r.Resolve(ctx, parsedDID.String())
	if err != nil {
		return nil, err
	}

	return SelectEd25519Key(parsedDID.String(), doc)
}

// SelectEd25519Key validates a resolved DID document and returns its first
// acceptable Ed25519 verification key.
//
// The document ID must equal the requested DID, the key type must be in the
// Ed25519 allowlist, the key must be 32 bytes and must not have small order.
// For did:key identifiers the selected key must also equal the key encoded in
// the identifier, so a resolver cannot bind a did:key to a different key.
func SelectEd25519Key(did string, doc *DIDDocument) (ed25519.PublicKey, error) {
	if doc == nil {
		return nil, ErrDIDNotFound
	}

	// Verify document ID matches requested DID exactly
	if doc.ID != did {
		return nil, ErrDIDMismatch
	}

	// Find first acceptable Ed25519 key
	for _, key := range doc.PublicKeys {
		// Check if key type is in allowlist
		if !allowedEd25519KeyTypes[key.Type] {
			continue
		}

		// Validate key length
		if len(key.PublicKeyBytes) != ed25519.PublicKeySize {
			continue
		}

		if IsSmallOrderPublicKey(key.PublicKeyBytes) {
			return nil, ErrSmallOrderKey
		}

		publicKey := ed25519.PublicKey(append([]byte(nil), key.PublicKeyBytes...))
		if IsDIDKey(did) {
			encoded, err := DecodeDIDKey(did)
			if err != nil {
				return nil, err
			}
			if !encoded.Equal(publicKey) {
				return nil, ErrDIDMismatch
			}
		}
		return publicKey, nil
	}

	return nil, ErrNoAcceptableKey
}

// DID Auth Transcript Constants
const (
	// DIDAuthTranscriptVersion is the version prefix for transcript binding
	DIDAuthTranscriptVersion = "AXCP-DID-AUTH-v1"
)

// BuildDIDAuthTranscript creates a canonical payload for DID authentication signing.
// The transcript binds the signature to specific parties, canonical signing
// payload bytes, and timestamp.
//
// Format:
//
//	AXCP-DID-AUTH-v1\n
//	<senderDID>\n
//	<recipientDID>\n
//	<base64(canonicalSigningPayload)>\n
//	<timestampRFC3339>
//
// This deterministic format ensures both parties compute identical transcripts.
func BuildDIDAuthTranscript(senderDID, recipientDID string, canonicalSigningPayload []byte, ts time.Time) []byte {
	payloadB64 := base64.StdEncoding.EncodeToString(canonicalSigningPayload)
	timestampStr := ts.UTC().Format(time.RFC3339)

	transcript := strings.Join([]string{
		DIDAuthTranscriptVersion,
		senderDID,
		recipientDID,
		payloadB64,
		timestampStr,
	}, "\n")

	return []byte(transcript)
}

// VerifyDIDAuthSignature verifies a DID authentication signature.
//
// This function:
//  1. Parses the signerDID to validate format
//  2. Resolves the DID document using the provided resolver
//  3. Extracts the Ed25519 public key from the document
//  4. Verifies the signature over the transcript
//
// The transcript should be built using BuildDIDAuthTranscript to ensure
// proper binding of the signature to the authentication context.
//
// Returns nil on success, or an error describing the failure.
func VerifyDIDAuthSignature(ctx context.Context, r DIDResolver, signerDID string, transcript []byte, signature []byte) error {
	// Resolve the public key (this validates DID format, resolves doc, checks ID match)
	publicKey, err := ResolveEd25519PublicKey(ctx, r, signerDID)
	if err != nil {
		return err
	}

	// Verify the signature using M2.1's VerifyEd25519
	if !VerifyEd25519(publicKey, transcript, signature) {
		return ErrVerificationFailed
	}

	return nil
}
