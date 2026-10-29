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

	// MaxConnections limits concurrent connections.
	// Default: 1024. Negative means unlimited (not recommended).
	MaxConnections int

	// MaxIncomingStreams limits concurrent bidirectional streams per connection.
	// Default: 16.
	MaxIncomingStreams int64

	// HandshakeIdleTimeout bounds the TLS handshake. Default: 10 seconds.
	HandshakeIdleTimeout time.Duration

	// AcceptTimeout bounds how long Accept waits for a ready connection, and how
	// long a new connection may take to open its control stream.
	// Default: 10 seconds
	AcceptTimeout time.Duration

	// MaxPendingConnections limits connections that are still opening their
	// control stream or waiting to be returned by Accept; further connections
	// are refused until one is ready or gives up. Default: 64.
	MaxPendingConnections int
}

// DefaultListenerConfig returns a ListenerConfig with sensible defaults.
func DefaultListenerConfig(address string, tlsConfig *tls.Config) ListenerConfig {
	return ListenerConfig{
		Address:               address,
		TLSConfig:             tlsConfig,
		MaxIdleTimeout:        5 * time.Minute,
		KeepAlivePeriod:       30 * time.Second,
		MaxConnections:        DefaultMaxConnections,
		MaxIncomingStreams:    DefaultMaxIncomingStreams,
		HandshakeIdleTimeout:  10 * time.Second,
		AcceptTimeout:         10 * time.Second,
		MaxPendingConnections: DefaultMaxPendingConnections,
	}
}

// Listener accepts incoming QUIC connections for AXCP.
// Listener accepts QUIC connections in the background, so a peer that never
// opens its control stream delays nobody else; Accept returns connections in
// the order they become ready.
type Listener struct {
	quicListener *quic.Listener
	config       ListenerConfig
	closed       atomic.Bool
	closing      chan struct{}
	connections  sync.Map // map[string]*ServerConnection
	connCount    atomic.Int32
	pending      atomic.Int32
	ready        chan *ServerConnection
	loopDone     chan struct{}
	loopErr      error // set before loopDone is closed
}

// Listener defaults.
const (
	DefaultMaxConnections        = 1024
	DefaultMaxIncomingStreams    = 16
	DefaultMaxPendingConnections = 64
)

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
	if config.MaxConnections == 0 {
		config.MaxConnections = DefaultMaxConnections
	}
	if config.MaxIncomingStreams <= 0 {
		config.MaxIncomingStreams = DefaultMaxIncomingStreams
	}
	if config.HandshakeIdleTimeout <= 0 {
		config.HandshakeIdleTimeout = 10 * time.Second
	}
	if config.MaxPendingConnections <= 0 {
		config.MaxPendingConnections = DefaultMaxPendingConnections
	}
	if tlsConfig.MinVersion < tls.VersionTLS13 {
		tlsConfig.MinVersion = tls.VersionTLS13
	}

	// Configure QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:        config.MaxIdleTimeout,
		KeepAlivePeriod:       config.KeepAlivePeriod,
		HandshakeIdleTimeout:  config.HandshakeIdleTimeout,
		MaxIncomingStreams:    config.MaxIncomingStreams,
		MaxIncomingUniStreams: -1,
		EnableDatagrams:       true,
	}

	// Create QUIC listener
	ql, err := quic.ListenAddr(config.Address, tlsConfig, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("netquic: failed to listen: %w", err)
	}

	l := &Listener{
		quicListener: ql,
		config:       config,
		closing:      make(chan struct{}),
		ready:        make(chan *ServerConnection),
		loopDone:     make(chan struct{}),
	}
	go l.acceptLoop()
	return l, nil
}

// acceptLoop admits QUIC connections until the listener closes.
func (l *Listener) acceptLoop() {
	defer close(l.loopDone)
	for {
		conn, err := l.quicListener.Accept(context.Background())
		if err != nil {
			l.loopErr = err
			return
		}
		if !l.admitPending() {
			_ = conn.CloseWithError(quic.ApplicationErrorCode(2), "server busy")
			continue
		}
		go l.awaitControlStream(conn)
	}
}

// admitPending reserves a pending slot within the pending and total limits.
func (l *Listener) admitPending() bool {
	for {
		pending := l.pending.Load()
		if int(pending) >= l.config.MaxPendingConnections {
			return false
		}
		if l.config.MaxConnections > 0 && int(pending)+int(l.connCount.Load()) >= l.config.MaxConnections {
			return false
		}
		if l.pending.CompareAndSwap(pending, pending+1) {
			return true
		}
	}
}

// awaitControlStream waits for the connection's first bidirectional stream,
// then hands it to Accept. The pending slot is released by Accept or here.
func (l *Listener) awaitControlStream(conn *quic.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.AcceptTimeout)
	stream, err := conn.AcceptStream(ctx)
	cancel()
	if err != nil {
		l.pending.Add(-1)
		_ = conn.CloseWithError(quic.ApplicationErrorCode(1), "failed to accept stream")
		return
	}
	sc := &ServerConnection{
		quicConn: conn,
		stream:   stream,
	}
	select {
	case l.ready <- sc:
	case <-conn.Context().Done():
		l.pending.Add(-1)
	case <-l.closing:
		l.pending.Add(-1)
		_ = conn.CloseWithError(0, "listener closed")
	}
}

// Accept waits for and returns the next incoming connection whose control
// stream is open. The connection must be authenticated by the caller before use.
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

	for {
		select {
		case sc := <-l.ready:
			l.pending.Add(-1)
			if sc.quicConn.Context().Err() != nil {
				continue // closed while waiting to be accepted
			}
			l.track(sc)
			return sc, nil
		case <-l.closing:
			return nil, ErrListenerClosed
		case <-l.loopDone:
			if l.closed.Load() {
				return nil, ErrListenerClosed
			}
			return nil, fmt.Errorf("netquic: accept failed: %w", l.loopErr)
		case <-acceptCtx.Done():
			if l.closed.Load() {
				return nil, ErrListenerClosed
			}
			return nil, fmt.Errorf("netquic: accept failed: %w", acceptCtx.Err())
		}
	}
}

// track counts an accepted connection until it closes.
func (l *Listener) track(sc *ServerConnection) {
	conn := sc.quicConn
	connID := conn.RemoteAddr().String()
	l.connections.Store(connID, sc)
	l.connCount.Add(1)
	go func() {
		<-conn.Context().Done()
		l.connections.Delete(connID)
		l.connCount.Add(-1)
	}()
}

// Close stops accepting new connections but does not close existing ones.
func (l *Listener) Close() error {
	if l.closed.Swap(true) {
		return nil // already closed
	}
	close(l.closing)
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
