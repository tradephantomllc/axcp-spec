package netquic

import "log"

// Client represents a simple UDP client for benchmarking
type Client struct {
	// No connection state needed for now as we're just doing UDP benchmarking
}

// QUICConn is an interface for connection operations
type QUICConn interface {
	// SendDatagram sends a datagram over the connection
	SendDatagram(data []byte) error
	// Close closes the connection
	Close() error
}

// Dial creates a new client (placeholder for UDP benchmarking)
func Dial(addr string, tlsConf interface{}) (*Client, error) {
	log.Printf("Running in UDP benchmark mode. No QUIC connection will be established.")
	return &Client{}, nil
}

// Close cleans up client resources
func (c *Client) Close() error {
	// Nothing to close in UDP benchmark mode
	return nil
}

// SendDatagram simulates sending a datagram (logs the action in benchmark mode)
func (c *Client) SendDatagram(data []byte) error {
	// Check if the datagram is too large
	if len(data) > MaxDatagramSize {
		log.Printf("Datagram too large: %d bytes (max %d)", len(data), MaxDatagramSize)
	}

	// Log the action in benchmark mode
	log.Printf("Would send datagram: %d bytes", len(data))
	return nil
}
