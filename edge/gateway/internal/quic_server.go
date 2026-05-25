package internal

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
	"google.golang.org/protobuf/proto"
)

const maxEnvelopeBytes = 10 * 1024 * 1024

var (
	errEnvelopeFrameEmpty    = errors.New("empty AXCP envelope frame")
	errEnvelopeFrameTooLarge = errors.New("AXCP envelope frame exceeds maximum size")
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
		go func(c *quic.Conn) {
			for {
				stream, err := c.AcceptStream(context.Background())
				if err != nil {
					log.Printf("[quic] errore accettazione stream: %v", err)
					return
				}

				// Gestisci lo stream in una goroutine separata
				go func(s *quic.Stream) {
					defer s.Close()
					for {
						env, err := readFramedEnvelope(s)
						if err != nil {
							if !errors.Is(err, io.EOF) {
								log.Printf("[quic] errore lettura envelope: %v", err)
								sendErrorResponse(s, uint32(pb.ErrorCode_MALFORMED_REQUEST), err.Error())
							}
							return
						}
						if h != nil {
							h(env)
						}
					}
				}(stream)
			}
		}(conn)

		// Gestione datagrammi
		go func(c *quic.Conn) {
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
						if dgram != nil {
							dgram(&td)
						}
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
		go func(c *quic.Conn) {
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
		go func(c *quic.Conn) {
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
						if dgram != nil {
							dgram(&td)
						}
					} else {
						log.Printf("[quic] telemetry unmarshal error: %v", err)
					}
				}
			}
		}(conn)
	}
}

// handleAuthenticatedStream processes a single stream with authentication
func handleAuthenticatedStream(s *quic.Stream, config ServerConfig, h AuthenticatedEnvelopeHandler) {
	defer s.Close()

	for {
		env, err := readFramedEnvelope(s)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[quic] envelope read error: %v", err)
				sendErrorResponse(s, uint32(pb.ErrorCode_MALFORMED_REQUEST), err.Error())
			}
			return
		}

		// Authenticate if authenticator is configured
		var authResult AuthResult
		if config.Authenticator != nil {
			payloadBytes, err := axcp.SigningPayloadFromProto(env)
			if err != nil {
				log.Printf("[quic] signing payload build error: %v", err)
				sendErrorResponse(s, uint32(pb.ErrorCode_MALFORMED_REQUEST), err.Error())
				return
			}

			envAuth := EnvelopeAuth{
				SenderDID:    env.GetSenderDid(),
				RecipientDID: env.GetRecipientDid(),
				TimestampMs:  env.GetTimestampMs(),
				Sequence:     env.GetSequence(),
				Signature:    env.GetSignature(),
				PayloadBytes: payloadBytes,
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

		if h != nil {
			h(env, &authResult)
		}
	}
}

// sendErrorResponse writes an error envelope response to the stream
func sendErrorResponse(w io.Writer, code uint32, reason string) {
	errEnv := &pb.AxcpEnvelope{
		Version: 1,
		Payload: &pb.AxcpEnvelope_Error{
			Error: &pb.ErrorMessage{
				Code:   code,
				Reason: reason,
			},
		},
	}

	if err := writeFramedEnvelope(w, errEnv); err != nil {
		log.Printf("[quic] failed to write error response: %v", err)
	}
}

func readFramedEnvelope(r io.Reader) (*pb.AxcpEnvelope, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}

	frameLen := binary.LittleEndian.Uint32(lenBuf[:])
	if frameLen == 0 {
		return nil, errEnvelopeFrameEmpty
	}
	if frameLen > maxEnvelopeBytes {
		return nil, fmt.Errorf("%w: %d > %d", errEnvelopeFrameTooLarge, frameLen, maxEnvelopeBytes)
	}

	buf := make([]byte, frameLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	var env pb.AxcpEnvelope
	if err := proto.Unmarshal(buf, &env); err != nil {
		return nil, fmt.Errorf("invalid envelope format: %w", err)
	}

	return &env, nil
}

func writeFramedEnvelope(w io.Writer, env *pb.AxcpEnvelope) error {
	if env == nil {
		return errors.New("cannot write nil AXCP envelope")
	}

	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(env)
	if err != nil {
		return err
	}
	if len(data) > maxEnvelopeBytes {
		return fmt.Errorf("%w: %d > %d", errEnvelopeFrameTooLarge, len(data), maxEnvelopeBytes)
	}

	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
