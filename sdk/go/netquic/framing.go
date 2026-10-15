package netquic

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

// MaxMessageSize bounds stream frames before allocation and before uint32 conversion.
const MaxMessageSize = 10 * 1024 * 1024

// FrameBodyTimeout bounds how long a peer may take to deliver the body of a
// frame once its header has been received.
const FrameBodyTimeout = 30 * time.Second

// initialFrameBuffer is the largest buffer allocated before frame bytes arrive.
const initialFrameBuffer = 64 * 1024

type readDeadliner interface {
	SetReadDeadline(time.Time) error
}

var ErrEmptyEnvelope = errors.New("empty AXCP envelope frame")

func readFrame(r io.Reader, order binary.ByteOrder, envelope bool) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := order.Uint32(header[:])
	if n > MaxMessageSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrMessageTooLarge, n, MaxMessageSize)
	}
	if envelope && n == 0 {
		return nil, ErrEmptyEnvelope
	}
	if d, ok := r.(readDeadliner); ok {
		_ = d.SetReadDeadline(time.Now().Add(FrameBodyTimeout))
		defer func() { _ = d.SetReadDeadline(time.Time{}) }()
	}
	// Grow the buffer as bytes arrive instead of trusting the announced size.
	var buf bytes.Buffer
	buf.Grow(int(min(n, initialFrameBuffer)))
	copied, err := io.CopyN(&buf, r, int64(n))
	if err != nil {
		if errors.Is(err, io.EOF) && copied < int64(n) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeFrame(w io.Writer, data []byte, order binary.ByteOrder, envelope bool) error {
	if len(data) > MaxMessageSize {
		return fmt.Errorf("%w: %d > %d", ErrMessageTooLarge, len(data), MaxMessageSize)
	}
	if envelope && len(data) == 0 {
		return ErrEmptyEnvelope
	}
	var header [4]byte
	order.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
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
