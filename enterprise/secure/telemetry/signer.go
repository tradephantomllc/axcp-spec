package telemetry

import (
    "crypto/ed25519"
    "crypto/rand"
    "errors"
)

// Signer encapsulates an Ed25519 keypair and provides helper methods
// for signing and verifying telemetry payloads.
//
// This type is intentionally minimal: private key management (e.g. loading
// from file or HSM) is out of scope and should be handled by the caller.
type Signer struct {
    priv ed25519.PrivateKey
    pub  ed25519.PublicKey
}

// NewSigner returns a signer from a private key.
// The public key is derived automatically.
func NewSigner(priv ed25519.PrivateKey) (*Signer, error) {
    if l := len(priv); l != ed25519.PrivateKeySize {
        return nil, errors.New("invalid private key length")
    }
    pub := priv.Public().(ed25519.PublicKey)
    return &Signer{priv: priv, pub: pub}, nil
}

// NewSignerFromSeed is a helper that derives the keypair from a 32-byte seed.
func NewSignerFromSeed(seed []byte) (*Signer, error) {
    if len(seed) != ed25519.SeedSize {
        return nil, errors.New("seed must be 32 bytes")
    }
    priv := ed25519.NewKeyFromSeed(seed)
    return NewSigner(priv)
}

// GenerateSigner creates a brand-new random keypair.
func GenerateSigner() (*Signer, error) {
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return nil, err
    }
    return &Signer{priv: priv, pub: pub}, nil
}

// Sign returns the Ed25519 signature of data.
func (s *Signer) Sign(data []byte) []byte {
    return ed25519.Sign(s.priv, data)
}

// Verify verifies the given signature against the provided data.
func (s *Signer) Verify(data, sig []byte) bool {
    return ed25519.Verify(s.pub, data, sig)
}

// PublicKey exposes the signer's public key.
func (s *Signer) PublicKey() ed25519.PublicKey { return s.pub }
