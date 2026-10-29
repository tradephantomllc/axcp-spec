package internal

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
	"google.golang.org/protobuf/proto"
)

const maxEnvelopeBytes = 10 * 1024 * 1024

// Resource limits for unauthenticated peers. Authentication happens per
// message, after a frame has been read, so the transport itself is bounded.
const (
	// DefaultMaxConnections bounds concurrent QUIC connections.
	DefaultMaxConnections = 512
	// DefaultMaxStreamsPerConnection bounds concurrent streams per connection.
	DefaultMaxStreamsPerConnection = 16
	// DefaultMaxPreAuthConnections bounds connections that have not yet
	// delivered an authenticated envelope.
	DefaultMaxPreAuthConnections = 64
	// DefaultMaxPreAuthConnectionsPerIP bounds such connections per remote IP.
	DefaultMaxPreAuthConnectionsPerIP = 8
	// DefaultPreAuthTimeout is how long a connection may stay unauthenticated.
	DefaultPreAuthTimeout = 10 * time.Second
	// DefaultMaxPreAuthBufferedBytes bounds frame bytes buffered, across all
	// unauthenticated connections, before their envelopes are verified.
	DefaultMaxPreAuthBufferedBytes = 64 * 1024 * 1024
	// frameReadChunk is the most memory committed ahead of received bytes.
	frameReadChunk = 4 * 1024
)

// frameBodyTimeout bounds how long a peer may take to send the rest of a frame
// once its first byte has arrived. It is a variable only so tests can shorten it.
var frameBodyTimeout = 30 * time.Second

var (
	errEnvelopeFrameEmpty    = errors.New("empty AXCP envelope frame")
	errEnvelopeFrameTooLarge = errors.New("AXCP envelope frame exceeds maximum size")
)

// EnvelopeHandler handles incoming AXCP messages
type EnvelopeHandler func(*pb.AxcpEnvelope)

// AuthenticatedEnvelopeHandler handles authenticated AXCP messages
type AuthenticatedEnvelopeHandler func(*pb.AxcpEnvelope, *AuthResult)

// TelemetryHandler handles telemetry datagrams
type TelemetryHandler func(*pb.TelemetryDatagram)

// ServerConfig holds configuration for the QUIC server
type ServerConfig struct {
	// Context cancels the listener and its active connections. Nil means Background.
	Context context.Context
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

	// MaxConnections bounds concurrent connections (0 = DefaultMaxConnections).
	MaxConnections int

	// MaxStreamsPerConnection bounds concurrent streams per connection
	// (0 = DefaultMaxStreamsPerConnection).
	MaxStreamsPerConnection int64

	// MaxPreAuthConnections bounds connections that have not yet delivered an
	// authenticated envelope (0 = DefaultMaxPreAuthConnections, capped at
	// MaxConnections).
	MaxPreAuthConnections int

	// MaxPreAuthConnectionsPerIP bounds unauthenticated connections per remote
	// IP address (0 = DefaultMaxPreAuthConnectionsPerIP).
	MaxPreAuthConnectionsPerIP int

	// PreAuthTimeout closes a connection that has not delivered an
	// authenticated envelope in time (0 = DefaultPreAuthTimeout).
	PreAuthTimeout time.Duration

	// MaxPreAuthBufferedBytes bounds frame bytes buffered across all
	// unauthenticated connections (0 = DefaultMaxPreAuthBufferedBytes).
	MaxPreAuthBufferedBytes int64
}

// quicConfig returns the QUIC limits applied to every listener.
func (c ServerConfig) quicConfig() *quic.Config {
	streams := c.MaxStreamsPerConnection
	if streams <= 0 {
		streams = DefaultMaxStreamsPerConnection
	}
	return &quic.Config{
		EnableDatagrams:       true,
		MaxIncomingStreams:    streams,
		MaxIncomingUniStreams: -1,
		HandshakeIdleTimeout:  10 * time.Second,
		MaxIdleTimeout:        60 * time.Second,
	}
}

func (c ServerConfig) maxConnections() int64 {
	if c.MaxConnections <= 0 {
		return DefaultMaxConnections
	}
	return int64(c.MaxConnections)
}

func (c ServerConfig) maxPreAuthConnections() int64 {
	n := int64(c.MaxPreAuthConnections)
	if n <= 0 {
		n = DefaultMaxPreAuthConnections
	}
	return min(n, c.maxConnections())
}

func (c ServerConfig) maxPreAuthConnectionsPerIP() int {
	if c.MaxPreAuthConnectionsPerIP <= 0 {
		return DefaultMaxPreAuthConnectionsPerIP
	}
	return c.MaxPreAuthConnectionsPerIP
}

func (c ServerConfig) preAuthTimeout() time.Duration {
	if c.PreAuthTimeout <= 0 {
		return DefaultPreAuthTimeout
	}
	return c.PreAuthTimeout
}

func (c ServerConfig) maxPreAuthBufferedBytes() int64 {
	if c.MaxPreAuthBufferedBytes <= 0 {
		return DefaultMaxPreAuthBufferedBytes
	}
	return c.MaxPreAuthBufferedBytes
}

// RunQuicServer is the explicit transport-only compatibility entry point.
func RunQuicServer(addr string, tlsConf *tls.Config, h EnvelopeHandler, dgram TelemetryHandler) error {
	return RunAuthenticatedQuicServer(ServerConfig{Addr: addr, TLSConf: tlsConf, Profile: negotiate.ProfileTransportOnly}, func(env *pb.AxcpEnvelope, _ *AuthResult) {
		if h != nil {
			h(env)
		}
	}, dgram)
}

// RunAuthenticatedQuicServer shares authentication and replay state across transports.
func RunAuthenticatedQuicServer(config ServerConfig, h AuthenticatedEnvelopeHandler, dgram TelemetryHandler) error {
	if err := validateServerConfig(config); err != nil {
		return err
	}
	if config.TLSConf == nil {
		return errors.New("TLS configuration is required")
	}
	tlsConf := config.TLSConf.Clone()
	if tlsConf.MinVersion < tls.VersionTLS13 {
		tlsConf.MinVersion = tls.VersionTLS13
	}
	listener, err := quic.ListenAddr(config.Addr, tlsConf, config.quicConfig())
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return serveQUIC(ctx, listener, config, h, dgram)
}

func validateServerConfig(config ServerConfig) error {
	if err := negotiate.ValidateProfile(config.Profile); err != nil {
		return err
	}
	if config.Profile == negotiate.ProfileSecureBaseline && config.Authenticator == nil {
		return ErrAuthRequired
	}
	return nil
}

func serveQUIC(ctx context.Context, listener *quic.Listener, config ServerConfig, h AuthenticatedEnvelopeHandler, dgram TelemetryHandler) error {
	if err := validateServerConfig(config); err != nil {
		return err
	}
	limits := newAdmission(config)
	return serveAdmitted(ctx, listener, config, limits, h, dgram)
}

func serveAdmitted(ctx context.Context, listener *quic.Listener, config ServerConfig, limits *admission, h AuthenticatedEnvelopeHandler, dgram TelemetryHandler) error {
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		ip := remoteIP(conn.RemoteAddr())
		if !limits.admit(ip) {
			_ = conn.CloseWithError(0x2, "server busy")
			continue
		}
		go serveConnection(ctx, conn, &connState{a: limits, ip: ip}, config, h, dgram)
	}
}

// serveConnection handles one admitted connection. It is closed if it does not
// deliver an authenticated envelope within the pre-auth timeout, which frees
// its slots for other peers.
func serveConnection(ctx context.Context, c *quic.Conn, state *connState, config ServerConfig, h AuthenticatedEnvelopeHandler, dgram TelemetryHandler) {
	defer state.release()
	stop := context.AfterFunc(ctx, func() { _ = c.CloseWithError(0, "gateway shutdown") })
	defer stop()
	defer c.CloseWithError(0, "connection finished")
	timer := time.AfterFunc(config.preAuthTimeout(), func() {
		if !state.isAuthenticated() {
			_ = c.CloseWithError(0x3, "authentication timeout")
		}
	})
	defer timer.Stop()
	go func() {
		for {
			data, err := c.ReceiveDatagram(c.Context())
			if err != nil {
				return
			}
			if err := handleDatagram(data, config, h, dgram); err != nil {
				log.Printf("[quic] rejected datagram: %v", err)
				continue
			}
			state.markAuthenticated()
		}
	}()
	for {
		stream, err := c.AcceptStream(ctx)
		if err != nil {
			return
		}
		go handleAuthenticatedStream(stream, config, h, state)
	}
}

// authenticateEnvelope validates the wire contract before consuming replay state.
func authenticateEnvelope(env *pb.AxcpEnvelope, config ServerConfig) AuthResult {
	malformed := func(err error) AuthResult {
		return AuthResult{Error: err, ErrorCode: uint32(pb.ErrorCode_MALFORMED_REQUEST)}
	}
	if env == nil || env.Version != 1 {
		return malformed(negotiate.ErrInvalidVersion)
	}
	if env.Payload == nil {
		return malformed(errors.New("envelope payload is required"))
	}
	switch config.Profile {
	case negotiate.ProfileTransportOnly:
		if env.Profile != 0 {
			return malformed(errors.New("envelope profile does not match transport-only listener"))
		}
		return AuthResult{Authenticated: true}
	case negotiate.ProfileSecureBaseline:
		if env.Profile != 1 {
			return malformed(errors.New("envelope profile does not match secure-baseline listener"))
		}
		if config.Authenticator == nil {
			return AuthResult{Error: ErrAuthRequired, ErrorCode: uint32(pb.ErrorCode_UNAUTHORIZED)}
		}
	default:
		return malformed(negotiate.ErrUnknownProfile)
	}
	payload, err := axcp.SigningPayloadFromProto(env)
	if err != nil {
		return malformed(err)
	}
	return config.Authenticator.VerifyEnvelope(context.Background(), config.Profile, EnvelopeAuth{
		SenderDID: env.SenderDid, RecipientDID: env.RecipientDid, TimestampMs: env.TimestampMs,
		Sequence: env.Sequence, Signature: env.Signature, PayloadBytes: payload,
	})
}

func handleDatagram(data []byte, config ServerConfig, h AuthenticatedEnvelopeHandler, dgram TelemetryHandler) error {
	if len(data) == 0 {
		return errors.New("empty telemetry datagram")
	}
	if len(data) > netquic.MaxDatagramSize {
		return netquic.ErrDatagramTooLarge
	}
	switch data[0] {
	case 0xA0:
		if config.Profile != negotiate.ProfileTransportOnly {
			return ErrAuthRequired
		}
		var td pb.TelemetryDatagram
		if err := proto.Unmarshal(data[1:], &td); err != nil {
			return fmt.Errorf("invalid legacy telemetry: %w", err)
		}
		if td.Payload == nil {
			return errors.New("telemetry payload is required")
		}
		if dgram != nil {
			dgram(&td)
		}
		return nil
	case 0xA1:
		decoded, err := axcp.FromBytesStrict(data[1:])
		if err != nil {
			return fmt.Errorf("invalid telemetry envelope: %w", err)
		}
		env := &decoded.AxcpEnvelope
		if env.GetTelemetry() == nil || env.GetTelemetry().Payload == nil {
			return errors.New("datagram requires a telemetry payload")
		}
		result := authenticateEnvelope(env, config)
		if !result.Authenticated {
			return result.Error
		}
		if h != nil {
			h(env, &result)
		}
		return nil
	default:
		return errors.New("unknown telemetry datagram prefix; expected 0xA0 or 0xA1")
	}
}

func handleAuthenticatedStream(s *quic.Stream, config ServerConfig, h AuthenticatedEnvelopeHandler, state *connState) {
	defer s.Close()
	defer s.CancelRead(0)
	for {
		// Frames of a connection that has not authenticated yet are charged to
		// the shared pre-auth budget until their envelope is verified.
		var budget *byteBudget
		if !state.isAuthenticated() {
			budget = state.a.budget
		}
		env, charged, err := readFramedEnvelopeBudgeted(s, budget)
		if err != nil {
			budget.release(charged)
			if !errors.Is(err, io.EOF) {
				log.Printf("[quic] rejected stream frame: %v", err)
				sendErrorResponse(s, uint32(pb.ErrorCode_MALFORMED_REQUEST))
			}
			return
		}
		result := authenticateEnvelope(env, config)
		budget.release(charged)
		if !result.Authenticated {
			log.Printf("[quic] rejected envelope: %v", result.Error)
			sendErrorResponse(s, result.ErrorCode)
			return
		}
		state.markAuthenticated()
		if h != nil {
			h(env, &result)
		}
	}
}

// sendErrorResponse writes an error envelope response to the stream.
//
// The response is not signed (the gateway holds no message signing key), so
// clients must treat it as advisory. Only the error code and a fixed reason are
// sent; detailed causes are logged locally and never returned to the peer.
func sendErrorResponse(w io.Writer, code uint32) {
	reason := pb.ErrorCode(code).String()
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
	env, _, err := readFramedEnvelopeBudgeted(r, nil)
	return env, err
}

// readFramedEnvelopeBudgeted reads one frame. Waiting for the first byte has no
// deadline (an idle stream between frames is normal); the rest of the header
// and the body must arrive within frameBodyTimeout. Memory grows with received
// bytes, and a non-nil budget is charged with them; the caller releases the
// returned charge.
func readFramedEnvelopeBudgeted(r io.Reader, budget *byteBudget) (*pb.AxcpEnvelope, int64, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:1]); err != nil {
		return nil, 0, err
	}
	if d, ok := r.(interface{ SetReadDeadline(time.Time) error }); ok {
		_ = d.SetReadDeadline(time.Now().Add(frameBodyTimeout))
		defer func() { _ = d.SetReadDeadline(time.Time{}) }()
	}
	if _, err := io.ReadFull(r, lenBuf[1:]); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, 0, err
	}

	frameLen := binary.LittleEndian.Uint32(lenBuf[:])
	if frameLen == 0 {
		return nil, 0, errEnvelopeFrameEmpty
	}
	if frameLen > maxEnvelopeBytes {
		return nil, 0, fmt.Errorf("%w: %d > %d", errEnvelopeFrameTooLarge, frameLen, maxEnvelopeBytes)
	}

	var buf bytes.Buffer
	var charged int64
	for remaining := int(frameLen); remaining > 0; {
		buf.Grow(min(remaining, frameReadChunk))
		chunk := buf.AvailableBuffer()[:min(remaining, frameReadChunk)]
		n, err := r.Read(chunk)
		if n > 0 {
			if budget != nil {
				if !budget.tryAcquire(int64(n)) {
					return nil, charged, errPreAuthBudgetExceeded
				}
				charged += int64(n)
			}
			buf.Write(chunk[:n])
			remaining -= n
		}
		if err != nil {
			if remaining == 0 && errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return nil, charged, err
		}
	}

	env, err := axcp.FromBytesStrict(buf.Bytes())
	if err != nil {
		return nil, charged, fmt.Errorf("invalid envelope format: %w", err)
	}

	return &env.AxcpEnvelope, charged, nil
}

func writeFramedEnvelope(w io.Writer, env *pb.AxcpEnvelope) error {
	if env == nil {
		return errors.New("cannot write nil AXCP envelope")
	}

	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(env)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errEnvelopeFrameEmpty
	}
	if len(data) > maxEnvelopeBytes {
		return fmt.Errorf("%w: %d > %d", errEnvelopeFrameTooLarge, len(data), maxEnvelopeBytes)
	}

	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	for _, part := range [][]byte{lenBuf[:], data} {
		n, err := w.Write(part)
		if err != nil {
			return err
		}
		if n != len(part) {
			return io.ErrShortWrite
		}
	}
	return nil
}
