package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	defaultAddr = "localhost:61300"
)

func main() {
	// Parse command line flags
	serverMode := flag.Bool("server", false, "Run in server mode")
	flag.Parse()

	// Generate self-signed certificate for testing
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("Failed to generate certificate: %v", err)
	}

	// Configure TLS
	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		NextProtos:         []string{"simple-chat"},
		InsecureSkipVerify: true, // Only for testing!
	}

	// Run in server or client mode based on flag
	if *serverMode {
		runServer(tlsConf)
	} else {
		runClient(tlsConf)
	}
}

func runServer(tlsConf *tls.Config) {
	log.Println("Starting server...")

	// Configure QUIC listener
	listener, err := quic.ListenAddr(defaultAddr, tlsConf, &quic.Config{})
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("Server listening on %s\n", defaultAddr)

	// Accept a single connection
	conn, err := listener.Accept(context.Background())
	if err != nil {
		log.Fatalf("Failed to accept connection: %v", err)
	}

	log.Println("Client connected")

	// Accept a stream
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		log.Fatalf("Failed to accept stream: %v", err)
	}
	defer stream.Close()

	// Read message from client
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		log.Fatalf("Failed to read from stream: %v", err)
	}

	log.Printf("Received: %s\n", buf[:n])

	// Send response
	_, err = stream.Write([]byte("Hello from server!"))
	if err != nil {
		log.Fatalf("Failed to write to stream: %v", err)
	}

	// Keep the server running
	select {}
}

func runClient(tlsConf *tls.Config) {
	log.Println("Starting client...")

	// Configure QUIC client
	session, err := quic.DialAddr(context.Background(), defaultAddr, tlsConf, &quic.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer session.CloseWithError(0, "Client closing")

	// Open a new stream
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Send a message
	message := "Hello, QUIC server!"
	_, err = stream.Write([]byte(message))
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}
	log.Printf("Sent: %s\n", message)

	// Read response
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		log.Fatalf("Failed to read response: %v", err)
	}

	log.Printf("Received: %s\n", buf[:n])
}

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"AXCP Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create key pair: %w", err)
	}

	return cert, nil
}
