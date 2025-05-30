// Telemetry example demonstrates how to use the AXCP telemetry functionality
// to send and receive telemetry data over QUIC.
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

// generateTLSConfig creates a basic TLS configuration for testing
func generateTLSConfig() (*tls.Config, error) {
	// Generate a self-signed certificate for testing
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"axcp/1.0"},
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// startServer starts a simple QUIC server that receives telemetry data
func startServer(addr string) error {
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %w", err)
	}

	listener, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer listener.Close()

	log.Printf("Server listening on %s\n", addr)

	for {
		// Accept a new connection
		session, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		// Handle the connection in a new goroutine
		go func(session quic.Connection) {
			defer session.CloseWithError(0, "closing connection")

			// Create a client to handle this connection
			client := &netquic.Client{Conn: session}

			log.Printf("New connection from %s\n", session.RemoteAddr())

			// Handle incoming telemetry data
			for {
				td, err := client.ReceiveTelemetry()
				if err != nil {
					log.Printf("Failed to receive telemetry: %v", err)
					return
				}

				// Process the received telemetry data
				if system := axcp.GetSystemStats(td); system != nil {
					log.Printf("Received system stats - CPU: %d%%, Memory: %d bytes, Temp: %d°C\n",
						system.CpuPercent, system.MemBytes, system.TemperatureC)
				}

				if tokens := axcp.GetTokenUsage(td); tokens != nil {
					log.Printf("Received token usage - Prompt: %d, Completion: %d\n",
						tokens.PromptTokens, tokens.CompletionTokens)
				}
			}
		}(session)
	}
}

// startClient starts a simple QUIC client that sends telemetry data
func startClient(serverAddr string) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Only for testing!
		NextProtos:         []string{"axcp/1.0"},
	}

	// Connect to the server
	client, err := netquic.Dial(serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}
	defer client.Close()

	log.Printf("Connected to %s\n", serverAddr)

	// Send some system stats
	if err := client.WithSystemStats(75, 1024*1024*1024, 45); err != nil {
		return fmt.Errorf("failed to send system stats: %w", err)
	}

	// Send some token usage
	if err := client.WithTokenUsage(123, 456); err != nil {
		return fmt.Errorf("failed to send token usage: %w", err)
	}

	log.Println("Telemetry data sent successfully")
	return nil
}

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	addr := flag.String("addr", "localhost:4242", "Server address (host:port)")
	flag.Parse()

	if *serverMode {
		if err := startServer(*addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		if err := startClient(*addr); err != nil {
			log.Fatalf("Client error: %v", err)
		}
	}
}
