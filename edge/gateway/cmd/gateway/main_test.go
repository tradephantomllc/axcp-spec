package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestBuildGatewayTLSConfigRequiresExplicitTLS(t *testing.T) {
	if _, err := buildGatewayTLSConfig("", "", false); err == nil {
		t.Fatal("expected TLS config without keypair to fail by default")
	}
}

func TestBuildGatewayTLSConfigRejectsPartialKeypair(t *testing.T) {
	if _, err := buildGatewayTLSConfig("server.crt", "", false); err == nil {
		t.Fatal("expected partial TLS keypair to fail")
	}
	if _, err := buildGatewayTLSConfig("", "server.key", false); err == nil {
		t.Fatal("expected partial TLS keypair to fail")
	}
}

func TestBuildGatewayTLSConfigLoadsKeypair(t *testing.T) {
	certFile, keyFile := writeTestTLSKeypair(t)

	cfg, err := buildGatewayTLSConfig(certFile, keyFile, false)
	if err != nil {
		t.Fatalf("buildGatewayTLSConfig: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("certificates = %d, want 1", len(cfg.Certificates))
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != "axcp/1" {
		t.Fatalf("NextProtos = %#v", cfg.NextProtos)
	}
}

func writeTestTLSKeypair(t *testing.T) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "server.crt")
	keyFile := filepath.Join(dir, "server.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	return certFile, keyFile
}
