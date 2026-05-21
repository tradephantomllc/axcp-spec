package internal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
)

func TestFramedEnvelopeRoundTrip(t *testing.T) {
	env := &pb.AxcpEnvelope{
		Version: 1,
		TraceId: "trace-123",
	}

	var buf bytes.Buffer
	if err := writeFramedEnvelope(&buf, env); err != nil {
		t.Fatalf("writeFramedEnvelope failed: %v", err)
	}

	got, err := readFramedEnvelope(&buf)
	if err != nil {
		t.Fatalf("readFramedEnvelope failed: %v", err)
	}
	if got.GetTraceId() != env.GetTraceId() {
		t.Fatalf("TraceId = %q, want %q", got.GetTraceId(), env.GetTraceId())
	}
}

func TestReadFramedEnvelopeRejectsEmptyFrame(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], 0)
	buf.Write(lenBuf[:])

	_, err := readFramedEnvelope(&buf)
	if !errors.Is(err, errEnvelopeFrameEmpty) {
		t.Fatalf("error = %v, want %v", err, errEnvelopeFrameEmpty)
	}
}

func TestReadFramedEnvelopeRejectsOversizedFrame(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], maxEnvelopeBytes+1)
	buf.Write(lenBuf[:])

	_, err := readFramedEnvelope(&buf)
	if !errors.Is(err, errEnvelopeFrameTooLarge) {
		t.Fatalf("error = %v, want %v", err, errEnvelopeFrameTooLarge)
	}
}

func TestReadFramedEnvelopeRejectsMalformedProto(t *testing.T) {
	payload := []byte{0xff, 0xff, 0xff}
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	buf.Write(lenBuf[:])
	buf.Write(payload)

	_, err := readFramedEnvelope(&buf)
	if err == nil {
		t.Fatal("expected malformed proto error")
	}
}
