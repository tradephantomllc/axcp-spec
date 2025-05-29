package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/quic-go/quic-go"
)

func main() {
	// Configurazione del server
	addr := "localhost:61300"
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatal("Failed to generate certificate:", err)
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"axcp-echo"},
	}

	// Avvia il server in una goroutine
	go func() {
		listener, err := quic.ListenAddr(addr, tlsConf, nil)
		if err != nil {
			log.Fatal("Failed to start server:", err)
		}
		defer listener.Close()

		log.Println("Server: listening on", addr)

		// Accetta una singola connessione
		sess, err := listener.Accept(context.Background())
		if err != nil {
			log.Fatal("Failed to accept connection:", err)
		}

		log.Println("Server: client connected")

		// Accetta un singolo stream
		stream, err := sess.AcceptStream(context.Background())
		if err != nil {
			log.Fatal("Failed to accept stream:", err)
		}

		log.Println("Server: stream accepted")

		// Leggi e rispondi ai messaggi
		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if err != nil {
				log.Println("Server read error:", err)
				return
			}

			msg := string(buf[:n])
			log.Printf("Server: received: %s\n", msg)

			// Invia indietro il messaggio ricevuto
			_, err = stream.Write([]byte("ECHO: " + msg))
			if err != nil {
				log.Println("Server write error:", err)
				return
			}
		}
	}()

	// Aspetta che il server sia pronto
	time.Sleep(1 * time.Second)

	// Client
	log.Println("Client: dialing server...")
	session, err := quic.DialAddr(context.Background(), addr, tlsConf, nil)
	if err != nil {
		log.Fatal("Failed to dial server:", err)
	}

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatal("Failed to open stream:", err)
	}
	defer stream.Close()

	log.Println("Client: connected to server")

	// Invia un messaggio di test
	testMsg := "Hello, AXCP!"
	log.Printf("Client: sending: %s\n", testMsg)
	_, err = stream.Write([]byte(testMsg))
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}

	// Leggi la risposta
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil {
		log.Fatal("Failed to read response:", err)
	}

	log.Printf("Client: received: %s\n", string(buf[:n]))
	fmt.Printf("Success! Received response: %s\n", string(buf[:n]))
}

// generateSelfSignedCert genera un certificato autofirmato per i test
func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
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
		return tls.Certificate{}, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}
