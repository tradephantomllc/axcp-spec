package netquic

import (
	"encoding/binary"
	"net"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
)

// ServerConnection represents an incoming QUIC connection from a client.
type ServerConnection struct {
	quicConn *quic.Conn
	stream   *quic.Stream

	// remoteDID is the authenticated peer's DID (set after handshake)
	remoteDID string

	// localDID is this server's DID
	localDID string

	// negotiated indicates whether the handshake is complete
	negotiated bool

	// recvMutex protects concurrent reads
	recvMutex sync.Mutex

	// sendMutex protects concurrent writes
	sendMutex sync.Mutex

	// closed indicates the connection has been closed
	closed bool
	mu     sync.Mutex
}

// RemoteAddr returns the remote network address.
func (c *ServerConnection) RemoteAddr() net.Addr {
	return c.quicConn.RemoteAddr()
}

// LocalAddr returns the local network address.
func (c *ServerConnection) LocalAddr() net.Addr {
	return c.quicConn.LocalAddr()
}

// RemoteDID returns the authenticated peer's DID.
// Returns empty string if not yet authenticated.
func (c *ServerConnection) RemoteDID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.remoteDID
}

// LocalDID returns this server's DID.
func (c *ServerConnection) LocalDID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.localDID
}

// IsNegotiated returns whether the handshake has completed.
func (c *ServerConnection) IsNegotiated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.negotiated
}

// SetAuthenticated marks the connection as authenticated with the given DIDs.
// This should be called by the handshake orchestrator after successful negotiation.
func (c *ServerConnection) SetAuthenticated(localDID, remoteDID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localDID = localDID
	c.remoteDID = remoteDID
	c.negotiated = true
}

// SendEnvelope sends an AXCP envelope over the connection.
func (c *ServerConnection) SendEnvelope(env *axcp.Envelope) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	c.mu.Lock()
	if c.closed || c.stream == nil {
		c.mu.Unlock()
		return ErrNotConnected
	}
	c.mu.Unlock()

	raw, err := axcp.ToBytes(env)
	if err != nil {
		return err
	}

	return writeFrame(c.stream, raw, binary.LittleEndian, true)
}

// RecvEnvelope receives an AXCP envelope from the connection.
func (c *ServerConnection) RecvEnvelope() (*axcp.Envelope, error) {
	c.recvMutex.Lock()
	defer c.recvMutex.Unlock()

	c.mu.Lock()
	if c.closed || c.stream == nil {
		c.mu.Unlock()
		return nil, ErrNotConnected
	}
	c.mu.Unlock()

	data, err := readFrame(c.stream, binary.LittleEndian, true)
	if err != nil {
		return nil, err
	}
	return axcp.FromBytes(data)
}

// SendMessage sends raw bytes over the connection with length prefix.
func (c *ServerConnection) SendMessage(data []byte) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	c.mu.Lock()
	if c.closed || c.stream == nil {
		c.mu.Unlock()
		return ErrNotConnected
	}
	c.mu.Unlock()

	return writeFrame(c.stream, data, binary.BigEndian, false)
}

// ReceiveMessage receives raw bytes from the connection.
func (c *ServerConnection) ReceiveMessage() ([]byte, error) {
	c.recvMutex.Lock()
	defer c.recvMutex.Unlock()

	c.mu.Lock()
	if c.closed || c.stream == nil {
		c.mu.Unlock()
		return nil, ErrNotConnected
	}
	c.mu.Unlock()

	return readFrame(c.stream, binary.BigEndian, false)
}

// SendDatagram sends a datagram using QUIC's unreliable datagram transport.
func (c *ServerConnection) SendDatagram(data []byte) error {
	c.mu.Lock()
	if c.closed || c.quicConn == nil {
		c.mu.Unlock()
		return ErrNotConnected
	}
	c.mu.Unlock()

	if len(data) > MaxDatagramSize {
		return ErrDatagramTooLarge
	}

	if !supportsOutboundDatagrams(c.quicConn.ConnectionState()) {
		return ErrDatagramNotSupported
	}

	return c.quicConn.SendDatagram(data)
}

// Close terminates the connection.
func (c *ServerConnection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	if c.stream != nil {
		_ = c.stream.Close()
	}
	if c.quicConn != nil {
		return c.quicConn.CloseWithError(0, "server closed")
	}
	return nil
}

// CloseWithError terminates the connection with an error code and reason.
func (c *ServerConnection) CloseWithError(code uint64, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	if c.stream != nil {
		_ = c.stream.Close()
	}
	if c.quicConn != nil {
		return c.quicConn.CloseWithError(quic.ApplicationErrorCode(code), reason)
	}
	return nil
}

// Context returns a context that is canceled when the connection is closed.
func (c *ServerConnection) Context() <-chan struct{} {
	return c.quicConn.Context().Done()
}
