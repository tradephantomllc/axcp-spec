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

// FrameBodyTimeout bounds how long a peer may take to deliver the rest of a
// frame, header included, once its first byte has been received.
const FrameBodyTimeout = 30 * time.Second

// frameReadChunk is the most memory committed ahead of received frame bytes.
const frameReadChunk = 4 * 1024

type readDeadliner interface {
	SetReadDeadline(time.Time) error
}

var ErrEmptyEnvelope = errors.New("empty AXCP envelope frame")

// readFrame reads one length-prefixed frame. Waiting for the first byte has no
// deadline, because an idle stream between frames is normal; the rest of the
// header and the body must arrive within FrameBodyTimeout. Memory grows with
// received bytes, never with the announced length.
func readFrame(r io.Reader, order binary.ByteOrder, envelope bool) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:1]); err != nil {
		return nil, err
	}
	if d, ok := r.(readDeadliner); ok {
		_ = d.SetReadDeadline(time.Now().Add(FrameBodyTimeout))
		defer func() { _ = d.SetReadDeadline(time.Time{}) }()
	}
	if _, err := io.ReadFull(r, header[1:]); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	n := order.Uint32(header[:])
	if n > MaxMessageSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrMessageTooLarge, n, MaxMessageSize)
	}
	if envelope && n == 0 {
		return nil, ErrEmptyEnvelope
	}
	var buf bytes.Buffer
	for remaining := int(n); remaining > 0; {
		step := min(remaining, frameReadChunk)
		buf.Grow(step)
		chunk := buf.AvailableBuffer()[:step]
		read, err := r.Read(chunk)
		if read > 0 {
			buf.Write(chunk[:read])
			remaining -= read
		}
		if err != nil {
			if remaining == 0 && errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
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
