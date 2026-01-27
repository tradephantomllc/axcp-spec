package auth

import (
	"crypto/ed25519"
)

// VerifyEd25519 verifies that the signature is valid for the payload
// using the provided Ed25519 public key.
//
// Returns true if the signature is valid, false otherwise.
// Returns false for invalid key sizes, invalid signature sizes,
// or if the signature doesn't match the payload.
//
// Empty payloads are valid inputs for verification.
func VerifyEd25519(publicKey ed25519.PublicKey, payload []byte, signature []byte) bool {
	// Validate public key size
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}

	// Validate signature size
	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(publicKey, payload, signature)
}
