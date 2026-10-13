package netquic

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MaxMessageSize bounds stream frames before allocation and before uint32 conversion.
const MaxMessageSize = 10 * 1024 * 1024

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
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
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
