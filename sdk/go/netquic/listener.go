// Package netquic provides QUIC networking for AXCP protocol.
package netquic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
)

// Listener errors
var (
	// ErrListenerClosed is returned when the listener has been closed
	ErrListenerClosed = errors.New("netquic: listener closed")

	// ErrMaxConnectionsReached is returned when max connections limit is reached
	ErrMaxConnectionsReached = errors.New("netquic: max connections reached")

	// ErrShutdownTimeout is returned when graceful shutdown times out
	ErrShutdownTimeout = errors.New("netquic: shutdown timeout")
)

// ListenerConfig configures the QUIC listener.
type ListenerConfig struct {
	// Address to listen on (e.g., "0.0.0.0:61300")
	Address string

	// TLSConfig for secure connections
	TLSConfig *tls.Config

	// MaxIdleTimeout is the maximum time a connection can be idle before closure
	// Default: 5 minutes
	MaxIdleTimeout time.Duration

	// KeepAlivePeriod is how often to send PING frames
	// Default: 30 seconds
	KeepAlivePeriod time.Duration

	// MaxConnections limits concurrent connections (0 = unlimited)
	MaxConnections int

	// AcceptTimeout is the timeout for accepting a new connection
	// Default: 10 seconds
	AcceptTimeout time.Duration
}

// DefaultListenerConfig returns a ListenerConfig with sensible defaults.
func DefaultListenerConfig(address string, tlsConfig *tls.Config) ListenerConfig {
	return ListenerConfig{
		Address:         address,
		TLSConfig:       tlsConfig,
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 30 * time.Second,
		MaxConnections:  0, // unlimited
		AcceptTimeout:   10 * time.Second,
	}
}

// Listener accepts incoming QUIC connections for AXCP.
type Listener struct {
	quicListener *quic.Listener
	config       ListenerConfig
	closed       atomic.Bool
	connections  sync.Map // map[string]*ServerConnection
	connCount    atomic.Int32
	mu           sync.Mutex
}

// Listen creates a new QUIC listener on the given configuration.
func Listen(config ListenerConfig) (*Listener, error) {
	if config.Address == "" {
		return nil, errors.New("netquic: address is required")
	}
	if config.TLSConfig == nil {
		return nil, errors.New("netquic: TLS config is required")
	}

	// Set QUIC-specific TLS config
	tlsConfig := config.TLSConfig.Clone()
	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"axcp/1"}
	}

	// Apply defaults
	if config.MaxIdleTimeout == 0 {
		config.MaxIdleTimeout = 5 * time.Minute
	}
	if config.KeepAlivePeriod == 0 {
		config.KeepAlivePeriod = 30 * time.Second
	}
	if config.AcceptTimeout == 0 {
		config.AcceptTimeout = 10 * time.Second
	}

	// Configure QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:  config.MaxIdleTimeout,
		KeepAlivePeriod: config.KeepAlivePeriod,
		EnableDatagrams: true,
	}

	// Create QUIC listener
	ql, err := quic.ListenAddr(config.Address, tlsConfig, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("netquic: failed to listen: %w", err)
	}

	return &Listener{
		quicListener: ql,
		config:       config,
	}, nil
}

// Accept waits for and returns the next incoming connection.
// The connection must be authenticated by the caller before use.
func (l *Listener) Accept(ctx context.Context) (*ServerConnection, error) {
	if l.closed.Load() {
		return nil, ErrListenerClosed
	}

	// Check max connections
	if l.config.MaxConnections > 0 && int(l.connCount.Load()) >= l.config.MaxConnections {
		return nil, ErrMaxConnectionsReached
	}

	// Create context with timeout if configured
	acceptCtx := ctx
	if l.config.AcceptTimeout > 0 {
		var cancel context.CancelFunc
		acceptCtx, cancel = context.WithTimeout(ctx, l.config.AcceptTimeout)
		defer cancel()
	}

	// Accept QUIC connection
	conn, err := l.quicListener.Accept(acceptCtx)
	if err != nil {
		if l.closed.Load() {
			return nil, ErrListenerClosed
		}
		return nil, fmt.Errorf("netquic: accept failed: %w", err)
	}

	// Accept the first bidirectional stream (control stream)
	// Use a fresh context to avoid timeout issues from the connection accept phase
	streamCtx, streamCancel := context.WithTimeout(context.Background(), l.config.AcceptTimeout)
	defer streamCancel()

	stream, err := conn.AcceptStream(streamCtx)
	if err != nil {
		conn.CloseWithError(quic.ApplicationErrorCode(1), "failed to accept stream")
		return nil, fmt.Errorf("netquic: failed to accept stream: %w", err)
	}

	// Create server connection
	sc := &ServerConnection{
		quicConn: conn,
		stream:   stream,
	}

	// Track connection
	connID := conn.RemoteAddr().String()
	l.connections.Store(connID, sc)
	l.connCount.Add(1)

	// Set up cleanup on connection close
	go func() {
		<-conn.Context().Done()
		l.connections.Delete(connID)
		l.connCount.Add(-1)
	}()

	return sc, nil
}

// Close stops accepting new connections but does not close existing ones.
func (l *Listener) Close() error {
	if l.closed.Swap(true) {
		return nil // already closed
	}
	return l.quicListener.Close()
}

// Shutdown gracefully shuts down the listener.
// It stops accepting new connections and waits for existing ones to close
// or until the context is canceled.
func (l *Listener) Shutdown(ctx context.Context) error {
	// Stop accepting new connections
	if err := l.Close(); err != nil {
		return err
	}

	// Wait for all connections to close or context to cancel
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Force close all remaining connections
			l.forceCloseAll()
			return ErrShutdownTimeout
		case <-ticker.C:
			if l.connCount.Load() == 0 {
				return nil
			}
		}
	}
}

// forceCloseAll closes all active connections.
func (l *Listener) forceCloseAll() {
	l.connections.Range(func(key, value any) bool {
		if conn, ok := value.(*ServerConnection); ok {
			_ = conn.Close()
		}
		return true
	})
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.quicListener.Addr()
}

// ConnectionCount returns the current number of active connections.
func (l *Listener) ConnectionCount() int {
	return int(l.connCount.Load())
}
