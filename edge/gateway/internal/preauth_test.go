package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
	"google.golang.org/protobuf/proto"
)

type testGateway struct {
	addr      string
	limits    *admission
	delivered chan *pb.AxcpEnvelope
}

// startTestGateway runs a Secure Baseline gateway on localhost with cfg's limits.
func startTestGateway(t *testing.T, tweak func(*ServerConfig)) (*testGateway, func(uint64) *pb.AxcpEnvelope) {
	t.Helper()
	cfg, makeEnv := secureFixture(t)
	cfg.TLSConf = netquic.InsecureTLSConfig()
	if tweak != nil {
		tweak(&cfg)
	}
	listener, err := quic.ListenAddr("127.0.0.1:0", cfg.TLSConf, cfg.quicConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	gw := &testGateway{addr: listener.Addr().String(), limits: newAdmission(cfg), delivered: make(chan *pb.AxcpEnvelope, 16)}
	done := make(chan error, 1)
	go func() {
		done <- serveAdmitted(ctx, listener, cfg, gw.limits, func(env *pb.AxcpEnvelope, _ *AuthResult) { gw.delivered <- env }, nil)
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("gateway did not stop")
		}
	})
	return gw, makeEnv
}

func dialTestGateway(t *testing.T, gw *testGateway) *quic.Conn {
	t.Helper()
	conn, err := quic.DialAddr(context.Background(), gw.addr, netquic.InsecureTLSConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseWithError(0, "test done") })
	return conn
}

func sendEnvelope(t *testing.T, conn *quic.Conn, env *pb.AxcpEnvelope) {
	t.Helper()
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFramedEnvelope(stream, env); err != nil {
		t.Fatal(err)
	}
}

func expectDelivered(t *testing.T, gw *testGateway) {
	t.Helper()
	select {
	case <-gw.delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("authenticated envelope was not delivered")
	}
}

func expectClosed(t *testing.T, conn *quic.Conn, reason string) {
	t.Helper()
	select {
	case <-conn.Context().Done():
		if cause := context.Cause(conn.Context()); cause == nil || !strings.Contains(cause.Error(), reason) {
			t.Fatalf("connection closed with %v, want %q", cause, reason)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("connection was not closed (%s)", reason)
	}
}

// waitCounts polls the admission counters until they match or time runs out.
func waitCounts(t *testing.T, a *admission, active, preAuth int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		gotActive, gotPreAuth := a.counts()
		if gotActive == active && gotPreAuth == preAuth {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("counts = active %d pre-auth %d, want %d %d", gotActive, gotPreAuth, active, preAuth)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUnauthenticatedConnectionClosedAfterPreAuthTimeout(t *testing.T) {
	gw, _ := startTestGateway(t, func(c *ServerConfig) { c.PreAuthTimeout = 200 * time.Millisecond })
	idle := dialTestGateway(t, gw)
	waitCounts(t, gw.limits, 1, 1)
	expectClosed(t, idle, "authentication timeout")
	waitCounts(t, gw.limits, 0, 0)
}

func TestPreAuthPeersCannotTakeAuthenticatedCapacity(t *testing.T) {
	gw, makeEnv := startTestGateway(t, func(c *ServerConfig) {
		c.MaxConnections = 4
		c.MaxPreAuthConnections = 2
		c.MaxPreAuthConnectionsPerIP = 4
		c.PreAuthTimeout = 5 * time.Second
	})
	legit := dialTestGateway(t, gw)
	sendEnvelope(t, legit, makeEnv(1))
	expectDelivered(t, gw)
	waitCounts(t, gw.limits, 1, 0)

	// One idle peer holds a pre-auth slot; a new client still gets the other
	// one and leaves the pre-auth pool as soon as it authenticates.
	dialTestGateway(t, gw)
	waitCounts(t, gw.limits, 2, 1)
	second := dialTestGateway(t, gw)
	sendEnvelope(t, second, makeEnv(2))
	expectDelivered(t, gw)
	waitCounts(t, gw.limits, 3, 1)

	// Idle peers fill the pre-auth pool; further unauthenticated peers are
	// refused, but authenticated connections keep working.
	dialTestGateway(t, gw)
	waitCounts(t, gw.limits, 4, 2)
	refused, err := quic.DialAddr(context.Background(), gw.addr, netquic.InsecureTLSConfig(), nil)
	if err == nil {
		expectClosed(t, refused, "server busy")
	} else if !strings.Contains(err.Error(), "server busy") {
		t.Fatal(err)
	}
	sendEnvelope(t, legit, makeEnv(3))
	expectDelivered(t, gw)
}

func TestPreAuthLimitPerRemoteIP(t *testing.T) {
	gw, _ := startTestGateway(t, func(c *ServerConfig) {
		c.MaxPreAuthConnections = 8
		c.MaxPreAuthConnectionsPerIP = 1
		c.PreAuthTimeout = 5 * time.Second
	})
	dialTestGateway(t, gw)
	waitCounts(t, gw.limits, 1, 1)
	second, err := quic.DialAddr(context.Background(), gw.addr, netquic.InsecureTLSConfig(), nil)
	if err == nil {
		expectClosed(t, second, "server busy")
	} else if !strings.Contains(err.Error(), "server busy") {
		t.Fatal(err)
	}
}

// stallReader returns its data, then blocks until its read deadline passes.
type stallReader struct {
	data     []byte
	deadline time.Time
}

func (r *stallReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if r.deadline.IsZero() {
		return 0, errors.New("read blocked without a deadline")
	}
	time.Sleep(time.Until(r.deadline))
	return 0, os.ErrDeadlineExceeded
}

func (r *stallReader) SetReadDeadline(t time.Time) error {
	r.deadline = t
	return nil
}

func TestPartialFrameHeaderTimesOut(t *testing.T) {
	old := frameBodyTimeout
	frameBodyTimeout = 50 * time.Millisecond
	defer func() { frameBodyTimeout = old }()

	// Two of four header bytes, then silence.
	_, _, err := readFramedEnvelopeBudgeted(&stallReader{data: []byte{0x10, 0x00}}, nil)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestPreAuthBudgetChargesReceivedBytes(t *testing.T) {
	budget := &byteBudget{max: 1000}
	var frame bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], 2000)
	frame.Write(lenBuf[:])
	frame.Write(make([]byte, 2000))

	_, charged, err := readFramedEnvelopeBudgeted(&frame, budget)
	if !errors.Is(err, errPreAuthBudgetExceeded) {
		t.Fatalf("error = %v, want budget exceeded", err)
	}
	if charged > budget.max || budget.used.Load() != charged {
		t.Fatalf("charged %d, used %d", charged, budget.used.Load())
	}
	budget.release(charged)
	if budget.used.Load() != 0 {
		t.Fatalf("budget not released: %d", budget.used.Load())
	}
}

func TestFrameMemoryFollowsReceivedBytes(t *testing.T) {
	var frame bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], 64*1024)
	frame.Write(lenBuf[:])
	frame.WriteByte(0x0a)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, _, err := readFramedEnvelopeBudgeted(&frame, nil)
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

func TestRetryBufferByteBudget(t *testing.T) {
	sized := func(id string, n int) *axcp.Envelope {
		env := axcp.NewEnvelope(id, 0)
		env.TraceId = strings.Repeat("x", n)
		return env
	}
	unit := int64(proto.Size(&sized("a", 1000).AxcpEnvelope))
	rb := NewRetryBuffer(&RetryBufferConfig{MaxCapacity: 100, MaxBytes: 2*unit + unit/2, MinRetryInterval: time.Second, MaxRetryInterval: time.Second, BackoffFactor: 1, MaxAttempts: 1}, NewMockTestLogger(t), nil)
	defer rb.Stop()

	if err := rb.AddEnvelope("a", sized("a", 1000)); err != nil {
		t.Fatal(err)
	}
	if err := rb.AddEnvelope("b", sized("b", 1000)); err != nil {
		t.Fatal(err)
	}
	if err := rb.AddEnvelope("c", sized("c", 1000)); err == nil {
		t.Fatal("envelope beyond the byte budget was accepted")
	}
	// Replacing an entry is charged by its new size, not added on top.
	if err := rb.AddEnvelope("b", sized("b", 1000)); err != nil {
		t.Fatalf("same-size replacement rejected: %v", err)
	}
	if err := rb.AddEnvelope("b", sized("b", 3000)); err == nil {
		t.Fatal("oversized replacement was accepted")
	}
	rb.mutex.Lock()
	rb.removeItemLocked("a")
	held := rb.bytes
	rb.mutex.Unlock()
	if held != unit {
		t.Fatalf("held %d bytes after removal, want %d", held, unit)
	}
	if err := rb.AddEnvelope("c", sized("c", 1000)); err != nil {
		t.Fatalf("budget not freed by removal: %v", err)
	}
}
