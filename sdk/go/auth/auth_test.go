package auth

import (
	"bytes"
	"crypto/ed25519"
	"testing"
)

func TestGenerateEd25519Keypair(t *testing.T) {
	publicKey, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		t.Errorf("public key size = %d, want %d", len(publicKey), ed25519.PublicKeySize)
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		t.Errorf("private key size = %d, want %d", len(privateKey), ed25519.PrivateKeySize)
	}
}

func TestSignAndVerify_Success(t *testing.T) {
	publicKey, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	payload := []byte("hello, AXCP!")

	signature, err := SignEd25519(privateKey, payload)
	if err != nil {
		t.Fatalf("SignEd25519 failed: %v", err)
	}

	if len(signature) != ed25519.SignatureSize {
		t.Errorf("signature size = %d, want %d", len(signature), ed25519.SignatureSize)
	}

	if !VerifyEd25519(publicKey, payload, signature) {
		t.Error("VerifyEd25519 returned false, want true")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	publicKey, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	originalPayload := []byte("hello, AXCP!")
	tamperedPayload := []byte("hello, AXCP?") // Changed last character

	signature, err := SignEd25519(privateKey, originalPayload)
	if err != nil {
		t.Fatalf("SignEd25519 failed: %v", err)
	}

	if VerifyEd25519(publicKey, tamperedPayload, signature) {
		t.Error("VerifyEd25519 returned true for tampered payload, want false")
	}
}

func TestVerify_WrongPublicKey(t *testing.T) {
	_, privateKey1, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	publicKey2, _, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	payload := []byte("hello, AXCP!")

	signature, err := SignEd25519(privateKey1, payload)
	if err != nil {
		t.Fatalf("SignEd25519 failed: %v", err)
	}

	// Verify with wrong public key should fail
	if VerifyEd25519(publicKey2, payload, signature) {
		t.Error("VerifyEd25519 returned true for wrong public key, want false")
	}
}

func TestSignAndVerify_EmptyPayload(t *testing.T) {
	publicKey, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	emptyPayload := []byte{}

	signature, err := SignEd25519(privateKey, emptyPayload)
	if err != nil {
		t.Fatalf("SignEd25519 failed for empty payload: %v", err)
	}

	if !VerifyEd25519(publicKey, emptyPayload, signature) {
		t.Error("VerifyEd25519 returned false for empty payload, want true")
	}
}

func TestSignAndVerify_NilPayload(t *testing.T) {
	publicKey, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	var nilPayload []byte = nil

	signature, err := SignEd25519(privateKey, nilPayload)
	if err != nil {
		t.Fatalf("SignEd25519 failed for nil payload: %v", err)
	}

	if !VerifyEd25519(publicKey, nilPayload, signature) {
		t.Error("VerifyEd25519 returned false for nil payload, want true")
	}
}

func TestSign_InvalidPrivateKeySize(t *testing.T) {
	tests := []struct {
		name string
		key  ed25519.PrivateKey
	}{
		{"empty key", ed25519.PrivateKey{}},
		{"too short", ed25519.PrivateKey(make([]byte, 32))},
		{"too long", ed25519.PrivateKey(make([]byte, 128))},
		{"nil key", nil},
	}

	payload := []byte("test payload")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SignEd25519(tt.key, payload)
			if err != ErrInvalidPrivateKeySize {
				t.Errorf("SignEd25519(%s) error = %v, want %v", tt.name, err, ErrInvalidPrivateKeySize)
			}
		})
	}
}

func TestVerify_InvalidPublicKeySize(t *testing.T) {
	_, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	payload := []byte("test payload")
	signature, _ := SignEd25519(privateKey, payload)

	tests := []struct {
		name string
		key  ed25519.PublicKey
	}{
		{"empty key", ed25519.PublicKey{}},
		{"too short", ed25519.PublicKey(make([]byte, 16))},
		{"too long", ed25519.PublicKey(make([]byte, 64))},
		{"nil key", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if VerifyEd25519(tt.key, payload, signature) {
				t.Errorf("VerifyEd25519(%s) = true, want false", tt.name)
			}
		})
	}
}

func TestVerify_InvalidSignatureSize(t *testing.T) {
	publicKey, _, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	payload := []byte("test payload")

	tests := []struct {
		name      string
		signature []byte
	}{
		{"empty signature", []byte{}},
		{"too short", make([]byte, 32)},
		{"too long", make([]byte, 128)},
		{"nil signature", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if VerifyEd25519(publicKey, payload, tt.signature) {
				t.Errorf("VerifyEd25519(%s) = true, want false", tt.name)
			}
		})
	}
}

func TestGenerateEd25519Keypair_Uniqueness(t *testing.T) {
	// Generate multiple keypairs and ensure they're unique
	keys := make(map[string]bool)
	iterations := 10

	for i := 0; i < iterations; i++ {
		publicKey, _, err := GenerateEd25519Keypair()
		if err != nil {
			t.Fatalf("GenerateEd25519Keypair failed: %v", err)
		}

		keyStr := string(publicKey)
		if keys[keyStr] {
			t.Errorf("duplicate public key generated on iteration %d", i)
		}
		keys[keyStr] = true
	}
}

func TestSignature_Deterministic(t *testing.T) {
	// Ed25519 signatures are deterministic for the same key and payload
	_, privateKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("GenerateEd25519Keypair failed: %v", err)
	}

	payload := []byte("deterministic test")

	sig1, err := SignEd25519(privateKey, payload)
	if err != nil {
		t.Fatalf("SignEd25519 failed: %v", err)
	}

	sig2, err := SignEd25519(privateKey, payload)
	if err != nil {
		t.Fatalf("SignEd25519 failed: %v", err)
	}

	if !bytes.Equal(sig1, sig2) {
		t.Error("Ed25519 signatures should be deterministic for same key and payload")
	}
}
