package internal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
)

func TestFramedEnvelopeRoundTrip(t *testing.T) {
	env := &pb.AxcpEnvelope{
		Version: 1,
		TraceId: "trace-123",
		Payload: &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{ContextId: "ctx"}},
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

func TestReadFramedEnvelopeRejectsTwoPayloadMembers(t *testing.T) {
	env := &pb.AxcpEnvelope{
		Version: 1,
		Payload: &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{ContextId: "ctx"}},
	}
	raw, _ := proto.Marshal(env)
	errMsg, _ := proto.Marshal(&pb.ErrorMessage{Code: 1})
	raw = protowire.AppendBytes(protowire.AppendTag(raw, 7, protowire.BytesType), errMsg)

	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(raw)))
	buf.Write(lenBuf[:])
	buf.Write(raw)
	if _, err := readFramedEnvelope(&buf); err == nil {
		t.Fatal("envelope with two payload members accepted")
	}
}

func TestReadFramedEnvelopeDoesNotTrustAnnouncedSize(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], maxEnvelopeBytes)
	buf.Write(lenBuf[:])
	buf.WriteString("short")

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := readFramedEnvelope(&buf)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want io.ErrUnexpectedEOF", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > maxEnvelopeBytes/4 {
		t.Fatalf("allocated %d bytes for a short frame", grew)
	}
}

func TestSendErrorResponseDoesNotLeakReason(t *testing.T) {
	var buf bytes.Buffer
	sendErrorResponse(&buf, uint32(pb.ErrorCode_MALFORMED_REQUEST))
	if bytes.Contains(buf.Bytes(), []byte("proto:")) {
		t.Fatal("internal parser error leaked")
	}
	got, err := readFramedEnvelope(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetError().GetReason() != pb.ErrorCode_MALFORMED_REQUEST.String() {
		t.Fatalf("reason = %q", got.GetError().GetReason())
	}
}

func TestQUICConfigLimits(t *testing.T) {
	cfg := ServerConfig{}.quicConfig()
	if cfg.MaxIncomingStreams != DefaultMaxStreamsPerConnection || cfg.MaxIncomingUniStreams >= 0 {
		t.Fatalf("stream limits not applied: %+v", cfg)
	}
	if cfg.HandshakeIdleTimeout == 0 || cfg.MaxIdleTimeout == 0 {
		t.Fatal("timeouts not applied")
	}
	if (ServerConfig{}).maxConnections() != DefaultMaxConnections {
		t.Fatal("connection limit not applied")
	}
}
