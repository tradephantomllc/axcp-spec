package netquic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

// InsecureTLSConfig returns a TLS 1.3 configuration with a freshly generated,
// self-signed certificate for localhost and certificate verification DISABLED.
//
// It exists for local demos and tests only. It provides no server
// authentication: never use it for traffic that leaves the machine. AXCP
// Secure Baseline still authenticates every message with Ed25519 signatures,
// but channel confidentiality against an active attacker requires verified
// TLS certificates.
func InsecureTLSConfig() *tls.Config {
	cert, err := generateCert()
	if err != nil {
		panic(fmt.Sprintf("netquic: generate development certificate: %v", err))
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // #nosec G402 -- explicitly insecure development helper
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{"axcp/1"},
	}
}

func generateCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"AXCP development"},
			CommonName:   "localhost",
		},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}
