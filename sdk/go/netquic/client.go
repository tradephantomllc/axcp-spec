package netquic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	defaultTimeout = 8 * time.Second
	// MaxDatagramSize is the maximum size of a QUIC datagram we'll accept
	MaxDatagramSize = 1200 // Standard QUIC MTU
)

var (
	// ErrNotConnected is returned when trying to use a closed or uninitialized client
	ErrNotConnected = errors.New("not connected")
	// ErrDatagramNotSupported is returned when the server doesn't support QUIC datagrams
	ErrDatagramNotSupported = errors.New("datagram not supported by server")
)

// Client represents a QUIC client connection to an AXCP server
type Client struct {
	conn      quic.Connection
	stream    quic.Stream
	recvMutex sync.Mutex
	sendMutex sync.Mutex
}

// Dial establishes a new QUIC connection to the server at the given address
func Dial(addr string, tlsConf *tls.Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Configure QUIC transport with datagram support
	config := &quic.Config{
		EnableDatagrams: true,             // Enable QUIC datagram support
		KeepAlivePeriod: 30 * time.Second, // Send a PING every 30 seconds
	}

	// Establish QUIC connection
	conn, err := quic.DialAddr(ctx, addr, tlsConf, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial QUIC server: %w", err)
	}

	// Open a bidirectional stream for control messages
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(quic.ApplicationErrorCode(1), "failed to open stream")
		return nil, fmt.Errorf("failed to open control stream: %w", err)
	}

	return &Client{
		conn:   conn,
		stream: stream,
	}, nil
}

// Close terminates the QUIC connection
func (c *Client) Close() error {
	if c.stream != nil {
		_ = c.stream.Close()
	}
	if c.conn != nil {
		return c.conn.CloseWithError(0, "client closed")
	}
	return nil
}

// SendMessage sends a message over the QUIC stream
func (c *Client) SendMessage(data []byte) error {
	if c.stream == nil {
		return ErrNotConnected
	}

	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	// Write message length as a 4-byte big-endian integer
	msgLen := uint32(len(data))
	header := []byte{
		byte(msgLen >> 24),
		byte(msgLen >> 16),
		byte(msgLen >> 8),
		byte(msgLen),
	}

	// Write header and data
	if _, err := c.stream.Write(header); err != nil {
		return fmt.Errorf("failed to write message header: %w", err)
	}

	if _, err := c.stream.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// ReceiveMessage receives a message from the QUIC stream
func (c *Client) ReceiveMessage() ([]byte, error) {
	if c.stream == nil {
		return nil, ErrNotConnected
	}

	c.recvMutex.Lock()
	defer c.recvMutex.Unlock()

	// Read message length (4 bytes)
	header := make([]byte, 4)
	if _, err := io.ReadFull(c.stream, header); err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	msgLen := uint32(header[0])<<24 |
		uint32(header[1])<<16 |
		uint32(header[2])<<8 |
		uint32(header[3])

	// Prevent allocating an excessively large buffer
	if msgLen > 10*1024*1024 { // 10MB max message size
		return nil, fmt.Errorf("message too large: %d bytes", msgLen)
	}

	// Read message data
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(c.stream, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	return data, nil
}

// SendDatagram sends a datagram using QUIC's unreliable datagram transport
func (c *Client) SendDatagram(data []byte) error {
	if c.conn == nil {
		return ErrNotConnected
	}

	if len(data) > MaxDatagramSize {
		return fmt.Errorf("datagram too large: %d > %d", len(data), MaxDatagramSize)
	}

	// Check if datagram is supported
	if !c.conn.ConnectionState().SupportsDatagrams {
		return ErrDatagramNotSupported
	}

	// Send the datagram using the datagram writer
	return c.conn.SendDatagram(data)
}

// ReceiveDatagram receives a datagram using QUIC's unreliable datagram transport
func (c *Client) ReceiveDatagram() ([]byte, error) {
	if c.conn == nil {
		return nil, ErrNotConnected
	}

	// Check if datagram is supported
	if !c.conn.ConnectionState().SupportsDatagrams {
		return nil, ErrDatagramNotSupported
	}

	// Receive the datagram with a context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the datagram reader to receive the message
	msg, err := c.conn.ReceiveDatagram(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive datagram: %w", err)
	}

	return msg, nil
}
