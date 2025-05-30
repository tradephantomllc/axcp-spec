package netquic

import (
	"encoding/binary"
	"fmt"
	"io"
)

// StreamMessage represents a message that can be sent or received over a stream
type StreamMessage struct {
	Data []byte
}

// SendMessage sends a message over a stream with a length prefix
// The message format is: [4-byte length][message data]
func SendMessage(w io.Writer, data []byte) error {
	// Prefix the message with its length (4 bytes, little-endian)
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(data)))
	
	// Send the length prefix
	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}
	
	// Send the actual data (if any)
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}
	
	return nil
}

// ReceiveMessage receives a message from a stream with a length prefix
// The expected message format is: [4-byte length][message data]
func ReceiveMessage(r io.Reader) ([]byte, error) {
	// Read the length prefix (4 bytes, little-endian)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("failed to read length prefix: %w", err)
	}
	
	// Get the message length
	msgLen := binary.LittleEndian.Uint32(lenBuf)
	if msgLen > 10*1024*1024 { // Sanity check: limit message size to 10MB
		return nil, fmt.Errorf("message too large: %d bytes", msgLen)
	}
	
	// If there's no data, return an empty slice
	if msgLen == 0 {
		return []byte{}, nil
	}
	
	// Read the message data
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}
	
	return data, nil
}
