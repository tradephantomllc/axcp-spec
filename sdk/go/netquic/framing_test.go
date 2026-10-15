package netquic

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
	"testing"
)

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestBoundedFraming(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, size := range []uint32{MaxMessageSize + 1, ^uint32(0)} {
			var header [4]byte
			order.PutUint32(header[:], size)
			_, err := readFrame(bytes.NewReader(header[:]), order, true)
			if !errors.Is(err, ErrMessageTooLarge) {
				t.Fatalf("size %d: %v", size, err)
			}
		}
		for _, size := range []int{1, MaxMessageSize} {
			payload := bytes.Repeat([]byte{42}, size)
			var wire bytes.Buffer
			if err := writeFrame(&wire, payload, order, true); err != nil {
				t.Fatal(err)
			}
			got, err := readFrame(&wire, order, true)
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("roundtrip size %d: %v", size, err)
			}
		}
		if _, err := readFrame(bytes.NewReader([]byte{0, 0, 0, 0}), order, true); !errors.Is(err, ErrEmptyEnvelope) {
			t.Fatal(err)
		}
		if _, err := readFrame(bytes.NewReader([]byte{0, 0, 0, 0}), order, false); err != nil {
			t.Fatal(err)
		}
		var wire bytes.Buffer
		if err := writeFrame(&wire, []byte{1, 2}, order, true); err != nil {
			t.Fatal(err)
		}
		if _, err := readFrame(bytes.NewReader(wire.Bytes()[:5]), order, true); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
		if err := writeFrame(shortWriter{}, []byte{1}, order, true); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal(err)
		}
		if err := writeFrame(io.Discard, make([]byte, MaxMessageSize+1), order, true); !errors.Is(err, ErrMessageTooLarge) {
			t.Fatal(err)
		}
	}
}

func TestDisconnectedEnvelopeMethods(t *testing.T) {
	for _, c := range []*Client{nil, {}} {
		if err := c.SendEnvelope(nil); !errors.Is(err, ErrNotConnected) {
			t.Fatal(err)
		}
		if _, err := c.RecvEnvelope(); !errors.Is(err, ErrNotConnected) {
			t.Fatal(err)
		}
	}
	c := &ServerConnection{}
	if err := c.SendEnvelope(nil); !errors.Is(err, ErrNotConnected) {
		t.Fatal(err)
	}
	if _, err := c.RecvEnvelope(); !errors.Is(err, ErrNotConnected) {
		t.Fatal(err)
	}
}

// slowReader returns its data one byte at a time and counts reads.
type slowReader struct {
	data []byte
}

func (r *slowReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}

func TestReadFrameDoesNotTrustAnnouncedSize(t *testing.T) {
	// A peer announces a maximum-size frame but sends only a few bytes:
	// the reader must fail without having allocated the announced size.
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, MaxMessageSize)
	r := &slowReader{data: append(header, []byte("tiny")...)}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := readFrame(r, binary.LittleEndian, true)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v, want io.ErrUnexpectedEOF", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > MaxMessageSize/4 {
		t.Fatalf("allocated %d bytes for a 4-byte body", grew)
	}
}
