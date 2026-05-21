package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestTrustedDIDFlagBuildsResolver(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	const did = "did:key:test-client"
	var trusted trustedDIDFlag
	if err := trusted.Set(did + "=" + base64.RawURLEncoding.EncodeToString(pub)); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if trusted.Len() != 1 {
		t.Fatalf("Len = %d, want 1", trusted.Len())
	}

	doc, err := trusted.Resolver().Resolve(context.Background(), did)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got := doc.PublicKeys[0].PublicKeyBytes; string(got) != string(pub) {
		t.Fatalf("resolved public key mismatch")
	}
}

func TestTrustedDIDFlagRejectsInvalidPublicKeyLength(t *testing.T) {
	var trusted trustedDIDFlag
	err := trusted.Set("did:key:test-client=" + base64.StdEncoding.EncodeToString([]byte("short")))
	if err == nil {
		t.Fatal("expected invalid public key length error")
	}
}
