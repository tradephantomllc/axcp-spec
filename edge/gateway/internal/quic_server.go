package internal

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

// EnvelopeHandler gestisce i messaggi AXCP in arrivo
type EnvelopeHandler func(*pb.AxcpEnvelope)

// AuthenticatedEnvelopeHandler handles authenticated AXCP messages
type AuthenticatedEnvelopeHandler func(*pb.AxcpEnvelope, *AuthResult)

// TelemetryHandler gestisce i datagrammi di telemetria
type TelemetryHandler func(*pb.TelemetryDatagram)

// ServerConfig holds configuration for the QUIC server
type ServerConfig struct {
	// Addr is the address to listen on
	Addr string

	// TLSConf is the TLS configuration
	TLSConf *tls.Config

	// Profile is the negotiated security profile
	Profile negotiate.Profile

	// Authenticator handles envelope authentication (required for Secure Baseline)
	Authenticator *EnvelopeAuthenticator

	// DIDResolver resolves DIDs to public keys (required for Secure Baseline)
	DIDResolver auth.DIDResolver

	// ServerDID is this server's DID for auth transcript binding
	ServerDID string
}

// RunQuicServer avvia il server QUIC con supporto per stream e datagrammi
func RunQuicServer(addr string, tlsConf *tls.Config, h EnvelopeHandler, dgram TelemetryHandler) error {
	listener, err := quic.ListenAddr(addr, tlsConf, nil)
	if err != nil {
		return err
	}
	log.Printf("[quic] in ascolto su %s", addr)

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}

		// Gestione stream
		go func(c quic.Connection) {
			for {
				stream, err := c.AcceptStream(context.Background())
				if err != nil {
					log.Printf("[quic] errore accettazione stream: %v", err)
					return
				}

				// Gestisci lo stream in una goroutine separata
				go func(s quic.Stream) {
					defer s.Close()
					// TODO: Implementa la gestione del messaggio AXCP
				}(stream)
			}
		}(conn)

		// Gestione datagrammi
		go func(c quic.Connection) {
			for {
				data, err := c.ReceiveDatagram(context.Background())
				if err != nil {
					log.Printf("[quic] errore ricezione datagramma: %v", err)
					return
				}

				// Se il datagramma inizia con 0xA0, è un datagramma di telemetria
				if len(data) > 0 && data[0] == 0xA0 {
					var td pb.TelemetryDatagram
					if err := proto.Unmarshal(data[1:], &td); err == nil {
						// Log per debug con informazioni di base sul datagramma di telemetria
						timestamp := td.GetTimestampMs()
						log.Printf("[quic] ricevuto datagramma telemetria, timestamp: %d", timestamp)
						dgram(&td)
					} else {
						log.Printf("[quic] errore unmarshal telemetria: %v", err)
					}
				}
			}
		}(conn)
	}
}

// RunAuthenticatedQuicServer runs a QUIC server with Secure Baseline authentication.
// When profile is secure-baseline-v1, all envelopes are verified before processing.
// Unauthenticated envelopes are rejected and an error response is sent.
func RunAuthenticatedQuicServer(config ServerConfig, h AuthenticatedEnvelopeHandler, dgram TelemetryHandler) error {
	listener, err := quic.ListenAddr(config.Addr, config.TLSConf, nil)
	if err != nil {
		return err
	}
	log.Printf("[quic] authenticated server listening on %s (profile: %s)", config.Addr, config.Profile)

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}

		// Handle streams (envelopes)
		go func(c quic.Connection) {
			for {
				stream, err := c.AcceptStream(context.Background())
				if err != nil {
					log.Printf("[quic] stream accept error: %v", err)
					return
				}

				go handleAuthenticatedStream(stream, config, h)
			}
		}(conn)

		// Handle datagrams (telemetry) - no auth required for datagrams
		go func(c quic.Connection) {
			for {
				data, err := c.ReceiveDatagram(context.Background())
				if err != nil {
					log.Printf("[quic] datagram receive error: %v", err)
					return
				}

				// 0xA0 prefix indicates telemetry datagram
				if len(data) > 0 && data[0] == 0xA0 {
					var td pb.TelemetryDatagram
					if err := proto.Unmarshal(data[1:], &td); err == nil {
						log.Printf("[quic] received telemetry datagram, timestamp: %d", td.GetTimestampMs())
						dgram(&td)
					} else {
						log.Printf("[quic] telemetry unmarshal error: %v", err)
					}
				}
			}
		}(conn)
	}
}

// handleAuthenticatedStream processes a single stream with authentication
func handleAuthenticatedStream(s quic.Stream, config ServerConfig, h AuthenticatedEnvelopeHandler) {
	defer s.Close()

	// Read the envelope from stream
	// TODO: Implement proper length-prefixed reading
	buf := make([]byte, 64*1024) // 64KB max envelope size
	n, err := s.Read(buf)
	if err != nil {
		log.Printf("[quic] stream read error: %v", err)
		return
	}

	var env pb.AxcpEnvelope
	if err := proto.Unmarshal(buf[:n], &env); err != nil {
		log.Printf("[quic] envelope unmarshal error: %v", err)
		sendErrorResponse(s, uint32(pb.ErrorCode_MALFORMED_REQUEST), "invalid envelope format")
		return
	}

	// Authenticate if authenticator is configured
	var authResult AuthResult
	if config.Authenticator != nil {
		envAuth := EnvelopeAuth{
			SenderDID:    env.GetSenderDid(),
			RecipientDID: env.GetRecipientDid(),
			TimestampMs:  env.GetTimestampMs(),
			Sequence:     env.GetSequence(),
			Signature:    env.GetSignature(),
			PayloadBytes: buf[:n], // Use serialized envelope as payload
		}

		authResult = config.Authenticator.VerifyEnvelope(context.Background(), config.Profile, envAuth)

		if !authResult.Authenticated {
			log.Printf("[quic] auth failed for envelope trace_id=%s: %v", env.GetTraceId(), authResult.Error)
			sendErrorResponse(s, authResult.ErrorCode, authResult.Error.Error())
			return
		}
	} else if negotiate.IsSecureProfile(config.Profile) {
		// No authenticator but secure profile required
		log.Printf("[quic] auth required but no authenticator configured")
		sendErrorResponse(s, uint32(pb.ErrorCode_UNAUTHORIZED), "authentication required")
		return
	}

	// Call handler with authenticated envelope
	h(&env, &authResult)
}

// sendErrorResponse writes an error envelope response to the stream
func sendErrorResponse(s quic.Stream, code uint32, reason string) {
	errEnv := &pb.AxcpEnvelope{
		Version: 1,
		Payload: &pb.AxcpEnvelope_Error{
			Error: &pb.ErrorMessage{
				Code:   code,
				Reason: reason,
			},
		},
	}

	data, err := proto.Marshal(errEnv)
	if err != nil {
		log.Printf("[quic] failed to marshal error response: %v", err)
		return
	}

	if _, err := s.Write(data); err != nil {
		log.Printf("[quic] failed to write error response: %v", err)
	}
}
