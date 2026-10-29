package netquic

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

// headerStallReader returns one header byte, then reports whether a read
// deadline was installed before the rest of the header was requested.
type headerStallReader struct {
	sent          bool
	deadlineSet   bool
	deadlineEarly bool
}

func (r *headerStallReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		p[0] = 0x10
		return 1, nil
	}
	r.deadlineEarly = r.deadlineSet
	return 0, io.EOF
}

func (r *headerStallReader) SetReadDeadline(t time.Time) error {
	if !t.IsZero() {
		r.deadlineSet = true
	}
	return nil
}

func TestReadFrameDeadlineCoversPartialHeader(t *testing.T) {
	r := &headerStallReader{}
	_, err := readFrame(r, binary.LittleEndian, true)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want io.ErrUnexpectedEOF", err)
	}
	if !r.deadlineEarly {
		t.Fatal("no read deadline before the rest of a started header")
	}
}

func TestReadFrameMemoryFollowsReceivedBytes(t *testing.T) {
	var frame bytes.Buffer
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], 64*1024)
	frame.Write(header[:])
	frame.WriteByte(0x0a)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := readFrame(&frame, binary.LittleEndian, true)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want io.ErrUnexpectedEOF", err)
	}
	// The announced 64 KiB must not be reserved up front. The bound leaves room
	// for the race detector's allocation overhead.
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 32*1024 {
		t.Fatalf("allocated %d bytes for a one-byte body", grew)
	}
}

// dialIdle completes the QUIC handshake but never opens a stream.
func dialIdle(t *testing.T, addr string) *quic.Conn {
	t.Helper()
	tlsConf := InsecureTLSConfig()
	tlsConf.NextProtos = []string{"axcp/1"}
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseWithError(0, "test done") })
	return conn
}

func TestListenerIdlePeerDoesNotDelayOthers(t *testing.T) {
	cfg := DefaultListenerConfig("127.0.0.1:0", InsecureTLSConfig())
	cfg.AcceptTimeout = 3 * time.Second
	l, err := Listen(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	addr := l.Addr().String()

	dialIdle(t, addr)
	time.Sleep(50 * time.Millisecond)
	client, err := Dial(addr, InsecureTLSConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.SendMessage([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	conn, err := l.Accept(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("Accept waited %v behind a peer that never opened a stream", waited)
	}
	if got, err := conn.ReceiveMessage(); err != nil || string(got) != "hello" {
		t.Fatalf("message = %q, %v", got, err)
	}
}

func TestListenerPendingLimit(t *testing.T) {
	cfg := DefaultListenerConfig("127.0.0.1:0", InsecureTLSConfig())
	cfg.MaxPendingConnections = 1
	l, err := Listen(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	addr := l.Addr().String()

	dialIdle(t, addr)
	deadline := time.Now().Add(2 * time.Second)
	for l.pending.Load() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("idle connection was not admitted as pending")
		}
		time.Sleep(10 * time.Millisecond)
	}
	second := dialIdle(t, addr)
	select {
	case <-second.Context().Done():
		if cause := context.Cause(second.Context()); cause == nil || !strings.Contains(cause.Error(), "server busy") {
			t.Fatalf("closed with %v, want server busy", cause)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connection beyond the pending limit was not refused")
	}
}

func TestListenerReleasesPendingSlotOnStreamTimeout(t *testing.T) {
	cfg := DefaultListenerConfig("127.0.0.1:0", InsecureTLSConfig())
	cfg.AcceptTimeout = 200 * time.Millisecond
	l, err := Listen(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	idle := dialIdle(t, l.Addr().String())
	select {
	case <-idle.Context().Done():
	case <-time.After(2 * time.Second):
		t.Fatal("connection without a control stream was not closed")
	}
	deadline := time.Now().Add(2 * time.Second)
	for l.pending.Load() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("pending = %d after the stream timeout", l.pending.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
