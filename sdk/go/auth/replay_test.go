package auth

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a mock clock for testing time-dependent behavior.
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time {
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.t = f.t.Add(d)
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)}
}

// --- Constructor Tests ---

func TestNewReplayProtector_Valid(t *testing.T) {
	rp, err := NewReplayProtector(time.Minute, 10, nil)
	if err != nil {
		t.Fatalf("NewReplayProtector failed: %v", err)
	}
	if rp == nil {
		t.Fatal("expected non-nil ReplayProtector")
	}
}

func TestNewReplayProtector_WithClock(t *testing.T) {
	clock := newFakeClock()
	rp, err := NewReplayProtector(time.Minute, 10, clock)
	if err != nil {
		t.Fatalf("NewReplayProtector failed: %v", err)
	}
	if rp == nil {
		t.Fatal("expected non-nil ReplayProtector")
	}
}

func TestNewReplayProtector_InvalidTTL(t *testing.T) {
	tests := []time.Duration{
		0,
		-time.Second,
		-time.Minute,
	}

	for _, ttl := range tests {
		_, err := NewReplayProtector(ttl, 10, nil)
		if err != ErrInvalidReplayParams {
			t.Errorf("NewReplayProtector(ttl=%v) error = %v, want ErrInvalidReplayParams", ttl, err)
		}
	}
}

func TestNewReplayProtector_InvalidWindow(t *testing.T) {
	_, err := NewReplayProtector(time.Minute, 0, nil)
	if err != ErrInvalidReplayParams {
		t.Errorf("NewReplayProtector(window=0) error = %v, want ErrInvalidReplayParams", err)
	}
}

// --- Sequence Mode: Basic Tests ---

func TestCheckAndMark_AcceptFirst(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	err := rp.CheckAndMark("peer1", 1)
	if err != nil {
		t.Errorf("first sequence should be accepted, got error: %v", err)
	}
}

func TestCheckAndMark_RejectReplay(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	// First time: accept
	err := rp.CheckAndMark("peer1", 1)
	if err != nil {
		t.Fatalf("first sequence should be accepted: %v", err)
	}

	// Second time: reject as replay
	err = rp.CheckAndMark("peer1", 1)
	if err != ErrReplay {
		t.Errorf("same sequence should be rejected as replay, got: %v", err)
	}
}

func TestCheckAndMark_EmptyPeerID(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	err := rp.CheckAndMark("", 1)
	if err != ErrEmptyPeerID {
		t.Errorf("empty peer ID should return ErrEmptyPeerID, got: %v", err)
	}
}

func TestCheckAndMark_MultiplePeers(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	// Same sequence for different peers should be independent
	err := rp.CheckAndMark("peer1", 100)
	if err != nil {
		t.Errorf("peer1 seq 100 should be accepted: %v", err)
	}

	err = rp.CheckAndMark("peer2", 100)
	if err != nil {
		t.Errorf("peer2 seq 100 should be accepted (independent from peer1): %v", err)
	}

	// Replay for peer1 should still be rejected
	err = rp.CheckAndMark("peer1", 100)
	if err != ErrReplay {
		t.Errorf("peer1 seq 100 replay should be rejected: %v", err)
	}
}

// --- Sequence Mode: Sliding Window Tests ---

func TestCheckAndMark_WindowAccept(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 5, clock) // window=5

	// Set highest to 100
	rp.CheckAndMark("peer1", 100)

	// With window=5 and highest=100, minAcceptable = 100 - 5 + 1 = 96
	// Accept 96, 97, 98, 99 (within window, not seen)
	for seq := uint64(96); seq <= 99; seq++ {
		err := rp.CheckAndMark("peer1", seq)
		if err != nil {
			t.Errorf("seq %d should be accepted (within window), got: %v", seq, err)
		}
	}
}

func TestCheckAndMark_WindowRejectTooOld(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 5, clock) // window=5

	// Set highest to 100
	rp.CheckAndMark("peer1", 100)

	// With window=5 and highest=100, minAcceptable = 96
	// Reject 95, 94, ... (too old)
	for seq := uint64(95); seq >= 90; seq-- {
		err := rp.CheckAndMark("peer1", seq)
		if err != ErrTooOld {
			t.Errorf("seq %d should be rejected as too old, got: %v", seq, err)
		}
	}
}

func TestCheckAndMark_WindowEdgeExact(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 5, clock)

	// Set highest to 100
	rp.CheckAndMark("peer1", 100)

	// minAcceptable = 100 - 5 + 1 = 96
	// seq=96 should be accepted (edge of window)
	err := rp.CheckAndMark("peer1", 96)
	if err != nil {
		t.Errorf("seq 96 (window edge) should be accepted, got: %v", err)
	}

	// seq=95 should be rejected (just outside window)
	err = rp.CheckAndMark("peer1", 95)
	if err != ErrTooOld {
		t.Errorf("seq 95 (outside window) should be rejected as too old, got: %v", err)
	}
}

func TestCheckAndMark_WindowUpdatesWithHigherSeq(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 5, clock)

	// Set highest to 100
	rp.CheckAndMark("peer1", 100)

	// Now 96 is acceptable
	err := rp.CheckAndMark("peer1", 96)
	if err != nil {
		t.Fatalf("seq 96 should be accepted: %v", err)
	}

	// Advance highest to 110
	rp.CheckAndMark("peer1", 110)

	// Now minAcceptable = 110 - 5 + 1 = 106
	// 96 would be too old now, but it's already seen (replay)
	err = rp.CheckAndMark("peer1", 96)
	if err != ErrReplay {
		t.Errorf("seq 96 should be replay (still in seen map), got: %v", err)
	}

	// 105 should be too old (not seen, but outside window)
	err = rp.CheckAndMark("peer1", 105)
	if err != ErrTooOld {
		t.Errorf("seq 105 should be too old, got: %v", err)
	}
}

func TestCheckAndMark_SmallWindow(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 1, clock) // window=1

	// Set highest to 100
	rp.CheckAndMark("peer1", 100)

	// With window=1, minAcceptable = 100 - 1 + 1 = 100
	// Only 100 and higher are acceptable
	err := rp.CheckAndMark("peer1", 99)
	if err != ErrTooOld {
		t.Errorf("seq 99 should be too old with window=1, got: %v", err)
	}

	err = rp.CheckAndMark("peer1", 101)
	if err != nil {
		t.Errorf("seq 101 should be accepted, got: %v", err)
	}
}

func TestCheckAndMark_LowSequenceNumbers(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	// Test with low sequence numbers (edge case for underflow)
	err := rp.CheckAndMark("peer1", 1)
	if err != nil {
		t.Errorf("seq 1 should be accepted: %v", err)
	}

	err = rp.CheckAndMark("peer1", 2)
	if err != nil {
		t.Errorf("seq 2 should be accepted: %v", err)
	}

	// With highest=2 and window=10, minAcceptable would be 0 or 1
	// All positive sequences should be acceptable
	err = rp.CheckAndMark("peer1", 3)
	if err != nil {
		t.Errorf("seq 3 should be accepted: %v", err)
	}
}

// --- Sequence Mode: TTL Expiry Tests ---

func TestCheckAndMark_ExpiryAllowsReuse(t *testing.T) {
	clock := newFakeClock()
	ttl := time.Minute
	rp, _ := NewReplayProtector(ttl, 100, clock) // Large window to avoid too-old rejection

	// Mark seq 50
	err := rp.CheckAndMark("peer1", 50)
	if err != nil {
		t.Fatalf("initial seq 50 should be accepted: %v", err)
	}

	// Immediately replay should fail
	err = rp.CheckAndMark("peer1", 50)
	if err != ErrReplay {
		t.Errorf("immediate replay should be rejected: %v", err)
	}

	// Advance time beyond TTL
	clock.Advance(ttl + time.Second)

	// Now seq 50 should be accepted again (expired from seen map)
	err = rp.CheckAndMark("peer1", 50)
	if err != nil {
		t.Errorf("seq 50 should be accepted after TTL expiry, got: %v", err)
	}
}

func TestCheckAndMark_PartialExpiry(t *testing.T) {
	clock := newFakeClock()
	ttl := time.Minute
	rp, _ := NewReplayProtector(ttl, 100, clock)

	// Mark seq 1 at t=0
	rp.CheckAndMark("peer1", 1)

	// Advance 30 seconds
	clock.Advance(30 * time.Second)

	// Mark seq 2 at t=30s
	rp.CheckAndMark("peer1", 2)

	// Advance 40 more seconds (total 70s from start)
	clock.Advance(40 * time.Second)

	// seq 1 should be expired (70s > 60s TTL), seq 2 should not (40s < 60s)
	// Trigger cleanup by calling CheckAndMark
	err := rp.CheckAndMark("peer1", 1) // Should be accepted (expired)
	if err != nil {
		t.Errorf("seq 1 should be accepted after expiry: %v", err)
	}

	err = rp.CheckAndMark("peer1", 2) // Should still be replay
	if err != ErrReplay {
		t.Errorf("seq 2 should still be replay (not expired): %v", err)
	}
}

// --- Nonce Mode Tests ---

func TestCheckAndMarkNonce_AcceptFirst(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	err := rp.CheckAndMarkNonce("peer1", "nonce-abc-123")
	if err != nil {
		t.Errorf("first nonce should be accepted: %v", err)
	}
}

func TestCheckAndMarkNonce_RejectReplay(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	nonce := "unique-nonce-xyz"

	err := rp.CheckAndMarkNonce("peer1", nonce)
	if err != nil {
		t.Fatalf("first nonce should be accepted: %v", err)
	}

	err = rp.CheckAndMarkNonce("peer1", nonce)
	if err != ErrReplay {
		t.Errorf("same nonce should be rejected as replay: %v", err)
	}
}

func TestCheckAndMarkNonce_EmptyPeerID(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	err := rp.CheckAndMarkNonce("", "nonce")
	if err != ErrEmptyPeerID {
		t.Errorf("empty peer ID should return ErrEmptyPeerID: %v", err)
	}
}

func TestCheckAndMarkNonce_EmptyNonce(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	err := rp.CheckAndMarkNonce("peer1", "")
	if err != ErrEmptyNonce {
		t.Errorf("empty nonce should return ErrEmptyNonce: %v", err)
	}
}

func TestCheckAndMarkNonce_MultiplePeers(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	nonce := "shared-nonce"

	// Same nonce for different peers should be independent
	err := rp.CheckAndMarkNonce("peer1", nonce)
	if err != nil {
		t.Errorf("peer1 nonce should be accepted: %v", err)
	}

	err = rp.CheckAndMarkNonce("peer2", nonce)
	if err != nil {
		t.Errorf("peer2 same nonce should be accepted (independent): %v", err)
	}

	// Replay for peer1 should be rejected
	err = rp.CheckAndMarkNonce("peer1", nonce)
	if err != ErrReplay {
		t.Errorf("peer1 nonce replay should be rejected: %v", err)
	}
}

func TestCheckAndMarkNonce_ExpiryAllowsReuse(t *testing.T) {
	clock := newFakeClock()
	ttl := time.Minute
	rp, _ := NewReplayProtector(ttl, 10, clock)

	nonce := "expiring-nonce"

	err := rp.CheckAndMarkNonce("peer1", nonce)
	if err != nil {
		t.Fatalf("first nonce should be accepted: %v", err)
	}

	// Immediate replay fails
	err = rp.CheckAndMarkNonce("peer1", nonce)
	if err != ErrReplay {
		t.Errorf("immediate replay should fail: %v", err)
	}

	// Advance beyond TTL
	clock.Advance(ttl + time.Second)

	// Now should be accepted
	err = rp.CheckAndMarkNonce("peer1", nonce)
	if err != nil {
		t.Errorf("nonce should be accepted after TTL expiry: %v", err)
	}
}

// --- Mixed Mode Tests ---

func TestMixedMode_IndependentTracking(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	// Sequence and nonce for same peer are tracked independently
	err := rp.CheckAndMark("peer1", 100)
	if err != nil {
		t.Errorf("seq 100 should be accepted: %v", err)
	}

	err = rp.CheckAndMarkNonce("peer1", "100") // String "100", not related to seq 100
	if err != nil {
		t.Errorf("nonce '100' should be accepted: %v", err)
	}

	// Both should reject replays independently
	err = rp.CheckAndMark("peer1", 100)
	if err != ErrReplay {
		t.Errorf("seq 100 replay should fail: %v", err)
	}

	err = rp.CheckAndMarkNonce("peer1", "100")
	if err != ErrReplay {
		t.Errorf("nonce '100' replay should fail: %v", err)
	}
}

// --- Reset Tests ---

func TestReset(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	// Add some state
	rp.CheckAndMark("peer1", 100)
	rp.CheckAndMarkNonce("peer1", "nonce1")

	// Verify replays fail
	if err := rp.CheckAndMark("peer1", 100); err != ErrReplay {
		t.Fatal("expected replay before reset")
	}

	// Reset
	rp.Reset()

	// Now should be accepted
	if err := rp.CheckAndMark("peer1", 100); err != nil {
		t.Errorf("seq should be accepted after reset: %v", err)
	}

	if err := rp.CheckAndMarkNonce("peer1", "nonce1"); err != nil {
		t.Errorf("nonce should be accepted after reset: %v", err)
	}
}

func TestResetPeer(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	// Add state for two peers
	rp.CheckAndMark("peer1", 100)
	rp.CheckAndMark("peer2", 100)

	// Reset only peer1
	rp.ResetPeer("peer1")

	// peer1 should accept, peer2 should reject
	if err := rp.CheckAndMark("peer1", 100); err != nil {
		t.Errorf("peer1 seq should be accepted after reset: %v", err)
	}

	if err := rp.CheckAndMark("peer2", 100); err != ErrReplay {
		t.Errorf("peer2 seq should still be replay: %v", err)
	}
}

// --- Stats Tests ---

func TestGetStats(t *testing.T) {
	clock := newFakeClock()
	rp, _ := NewReplayProtector(time.Minute, 10, clock)

	stats := rp.GetStats()
	if stats.PeerCount != 0 || stats.TotalSeenSeqs != 0 || stats.TotalSeenNonces != 0 {
		t.Errorf("initial stats should be zero: %+v", stats)
	}

	// Add some state
	rp.CheckAndMark("peer1", 1)
	rp.CheckAndMark("peer1", 2)
	rp.CheckAndMark("peer2", 1)
	rp.CheckAndMarkNonce("peer3", "n1")
	rp.CheckAndMarkNonce("peer3", "n2")

	stats = rp.GetStats()
	if stats.TotalSeenSeqs != 3 {
		t.Errorf("expected 3 seen seqs, got %d", stats.TotalSeenSeqs)
	}
	if stats.TotalSeenNonces != 2 {
		t.Errorf("expected 2 seen nonces, got %d", stats.TotalSeenNonces)
	}
}

// --- Concurrency Tests ---

func TestCheckAndMark_Concurrent(t *testing.T) {
	rp, _ := NewReplayProtector(time.Minute, 1000, nil)

	var wg sync.WaitGroup
	numGoroutines := 100
	seqsPerGoroutine := 100

	// Each goroutine uses unique sequences
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			baseSeq := uint64(goroutineID * seqsPerGoroutine)
			for i := 0; i < seqsPerGoroutine; i++ {
				seq := baseSeq + uint64(i)
				err := rp.CheckAndMark("peer1", seq)
				if err != nil {
					t.Errorf("goroutine %d seq %d: unexpected error %v", goroutineID, seq, err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify all sequences are now replays
	for g := 0; g < numGoroutines; g++ {
		baseSeq := uint64(g * seqsPerGoroutine)
		err := rp.CheckAndMark("peer1", baseSeq)
		if err != ErrReplay {
			t.Errorf("seq %d should be replay after concurrent writes", baseSeq)
		}
	}
}

func TestCheckAndMarkNonce_Concurrent(t *testing.T) {
	rp, _ := NewReplayProtector(time.Minute, 10, nil)

	var wg sync.WaitGroup
	numGoroutines := 50
	noncesPerGoroutine := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < noncesPerGoroutine; i++ {
				nonce := string(rune('A'+goroutineID)) + "-" + string(rune('0'+i%10))
				_ = rp.CheckAndMarkNonce("peer1", nonce) // May have collisions, that's ok
			}
		}(g)
	}

	wg.Wait()

	// Just verify no panics occurred - concurrency test passed
}

// --- Benchmark ---

func BenchmarkCheckAndMark(b *testing.B) {
	rp, _ := NewReplayProtector(time.Minute, 10000, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rp.CheckAndMark("peer1", uint64(i))
	}
}

func BenchmarkCheckAndMarkNonce(b *testing.B) {
	rp, _ := NewReplayProtector(time.Minute, 10, nil)
	nonces := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		nonces[i] = "nonce-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rp.CheckAndMarkNonce("peer1", nonces[i%len(nonces)])
	}
}
