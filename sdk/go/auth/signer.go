// Package auth provides Ed25519 cryptographic signing and verification
// for AXCP Core message authentication.
//
// This package uses only Go standard library (crypto/ed25519, crypto/rand)
// and has no external dependencies.
package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
)

var (
	// ErrInvalidPrivateKeySize is returned when the private key has incorrect size.
	ErrInvalidPrivateKeySize = errors.New("auth: invalid private key size, expected 64 bytes")

	// ErrSigningFailed is returned when the signing operation fails.
	ErrSigningFailed = errors.New("auth: signing operation failed")
)

// GenerateEd25519Keypair generates a new Ed25519 public/private key pair
// using cryptographically secure random bytes.
//
// Returns the public key (32 bytes), private key (64 bytes), and any error.
func GenerateEd25519Keypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return publicKey, privateKey, nil
}

// SignEd25519 signs the payload using the provided Ed25519 private key.
//
// The private key must be exactly 64 bytes (ed25519.PrivateKeySize).
// Returns the 64-byte signature or an error if the key is invalid.
//
// Empty payloads are valid and will produce a valid signature.
func SignEd25519(privateKey ed25519.PrivateKey, payload []byte) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, ErrInvalidPrivateKeySize
	}

	signature := ed25519.Sign(privateKey, payload)
	return signature, nil
}
