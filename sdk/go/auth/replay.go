package auth

import (
	"errors"
	"math/bits"
	"sync"
	"time"
)

// Replay protection errors
var (
	// ErrReplay indicates the sequence/nonce was already seen
	ErrReplay = errors.New("auth: replay detected")

	// ErrTooOld indicates the sequence is below the sliding window threshold
	ErrTooOld = errors.New("auth: sequence too old (outside sliding window)")

	// ErrInvalidReplayParams indicates invalid parameters for replay protection
	ErrInvalidReplayParams = errors.New("auth: invalid replay parameters")

	// ErrEmptyPeerID indicates an empty peer identifier was provided
	ErrEmptyPeerID = errors.New("auth: peer ID cannot be empty")

	// ErrEmptyNonce indicates an empty nonce was provided
	ErrEmptyNonce = errors.New("auth: nonce cannot be empty")

	// ErrReplayCapacity indicates the protector already tracks the maximum
	// number of peers. New peers are rejected (fail closed) until idle peers expire.
	ErrReplayCapacity = errors.New("auth: replay protection capacity exceeded")
)

const (
	// DefaultReplayPeerTTL is the default idle time after which a peer's replay
	// state is discarded. It must be at least the timestamp max age plus the
	// allowed clock skew, so a message whose state was discarded is already
	// rejected by timestamp validation.
	DefaultReplayPeerTTL = 10 * time.Minute

	// DefaultReplayWindow is the default sliding window size in sequences.
	DefaultReplayWindow = 1000

	// DefaultMaxReplayPeers is the default maximum number of tracked peers.
	DefaultMaxReplayPeers = 100000

	// maxReplayWindow bounds the per-peer bitmap (128 KiB per peer at most).
	maxReplayWindow = 1 << 20
)

// Clock abstracts time for testing purposes.
type Clock interface {
	Now() time.Time
}

// RealClock uses the system clock.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// peerWindow tracks replay state for a single peer using sequence numbers.
//
// Bit i of seen records whether sequence highest-i was accepted.
type peerWindow struct {
	highest  uint64
	seen     []uint64
	lastSeen time.Time
}

// peerNonces tracks nonce-based replay state for a single peer.
type peerNonces struct {
	nonces   map[string]time.Time
	lastSeen time.Time
}

// ReplayProtector provides thread-safe replay attack protection.
//
// It supports two modes:
//  1. Sequence mode: per-peer sliding window over monotonic sequence numbers.
//     Accepted sequences are remembered in a bitmap for as long as the peer is
//     active; they never expire while still inside the window.
//  2. Nonce mode: per-peer nonce tracking; each nonce is remembered for the TTL.
//
// Peer state that has been idle for longer than the TTL is discarded. The TTL
// must therefore be at least the timestamp max age plus the allowed clock skew
// (see EnsureMinTTL); otherwise a discarded message could be accepted again.
// The number of tracked peers is bounded: when the limit is reached and no idle
// peer can be discarded, new peers are rejected with ErrReplayCapacity.
type ReplayProtector struct {
	mu       sync.Mutex
	clock    Clock
	ttl      time.Duration
	window   uint64
	maxPeers int

	lastSweep time.Time

	// peers tracks sequence-based replay state per peer
	peers map[string]*peerWindow

	// nonces tracks nonce-based replay state per peer
	nonces map[string]*peerNonces
}

// NewReplayProtector creates a new replay protector.
//
// Parameters:
//   - ttl: idle time after which a peer's replay state (and each nonce) is
//     discarded. Must be > 0 and should be at least the timestamp max age plus
//     clock skew; validators raise it automatically with EnsureMinTTL.
//   - window: sliding window size for sequence mode. Sequences `window` or more
//     behind the highest accepted sequence are rejected as too old. Must be
//     between 1 and 1048576.
//   - clock: time source. If nil, uses the system clock.
//
// Returns ErrInvalidReplayParams for invalid parameters.
func NewReplayProtector(ttl time.Duration, window uint64, clock Clock) (*ReplayProtector, error) {
	if ttl <= 0 {
		return nil, ErrInvalidReplayParams
	}
	if window < 1 || window > maxReplayWindow {
		return nil, ErrInvalidReplayParams
	}
	if clock == nil {
		clock = RealClock{}
	}
	return &ReplayProtector{
		clock:     clock,
		ttl:       ttl,
		window:    window,
		maxPeers:  DefaultMaxReplayPeers,
		lastSweep: clock.Now(),
		peers:     make(map[string]*peerWindow),
		nonces:    make(map[string]*peerNonces),
	}, nil
}

// NewDefaultReplayProtector returns a protector with DefaultReplayPeerTTL,
// DefaultReplayWindow and the system clock.
func NewDefaultReplayProtector() *ReplayProtector {
	rp, err := NewReplayProtector(DefaultReplayPeerTTL, DefaultReplayWindow, nil)
	if err != nil {
		panic(err) // unreachable: defaults are valid
	}
	return rp
}

// TTL returns the configured peer/nonce TTL.
func (rp *ReplayProtector) TTL() time.Duration {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.ttl
}

// EnsureMinTTL raises the TTL to at least min. Validators call it with the
// timestamp max age plus clock skew so that discarding idle state can never
// re-open a replay window.
func (rp *ReplayProtector) EnsureMinTTL(min time.Duration) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if min > rp.ttl {
		rp.ttl = min
	}
}

// SetMaxPeers sets the maximum number of tracked peers (per mode).
// A value <= 0 restores DefaultMaxReplayPeers.
func (rp *ReplayProtector) SetMaxPeers(n int) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if n <= 0 {
		n = DefaultMaxReplayPeers
	}
	rp.maxPeers = n
}

// CheckAndMark checks if a sequence number is a replay and marks it as seen.
//
// For a given peerID and sequence number, with H the highest accepted
// sequence and W the window size:
//   - the first sequence from a peer is accepted;
//   - seq > H advances the window and is accepted;
//   - H-W < seq <= H is rejected with ErrReplay if already accepted, otherwise accepted;
//   - seq <= H-W is rejected with ErrTooOld.
//
// Callers must invoke it only after the message signature has been verified.
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) CheckAndMark(peerID string, seq uint64) error {
	if peerID == "" {
		return ErrEmptyPeerID
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	now := rp.clock.Now()
	rp.maybeSweep(now)

	pw, exists := rp.peers[peerID]
	if exists && now.Sub(pw.lastSeen) > rp.ttl {
		delete(rp.peers, peerID)
		exists = false
	}
	if !exists {
		if len(rp.peers) >= rp.maxPeers {
			rp.sweep(now)
			if len(rp.peers) >= rp.maxPeers {
				return ErrReplayCapacity
			}
		}
		pw = &peerWindow{highest: seq, seen: make([]uint64, (rp.window+63)/64), lastSeen: now}
		pw.seen[0] = 1
		rp.peers[peerID] = pw
		return nil
	}

	if seq > pw.highest {
		pw.shift(seq-pw.highest, rp.window)
		pw.highest = seq
		pw.seen[0] |= 1
		pw.lastSeen = now
		return nil
	}

	diff := pw.highest - seq
	if diff >= rp.window {
		return ErrTooOld
	}
	word, bit := diff/64, diff%64
	if pw.seen[word]&(1<<bit) != 0 {
		return ErrReplay
	}
	pw.seen[word] |= 1 << bit
	pw.lastSeen = now
	return nil
}

// shift moves the bitmap so that bit 0 represents a sequence n higher than before.
func (pw *peerWindow) shift(n, window uint64) {
	if n >= window {
		clear(pw.seen)
		return
	}
	words, offset := int(n/64), uint(n%64)
	for i := len(pw.seen) - 1; i >= 0; i-- {
		var v uint64
		if src := i - words; src >= 0 {
			v = pw.seen[src] << offset
			if offset != 0 && src-1 >= 0 {
				v |= pw.seen[src-1] >> (64 - offset)
			}
		}
		pw.seen[i] = v
	}
	// Clear bits beyond the window in the last word.
	if rem := window % 64; rem != 0 {
		pw.seen[len(pw.seen)-1] &= (uint64(1) << rem) - 1
	}
}

// CheckAndMarkNonce checks if a nonce is a replay and marks it as seen.
//
// Each nonce is remembered for the TTL. Unlike sequence mode, nonce mode has
// no ordering constraints.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) CheckAndMarkNonce(peerID string, nonce string) error {
	if peerID == "" {
		return ErrEmptyPeerID
	}
	if nonce == "" {
		return ErrEmptyNonce
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	now := rp.clock.Now()
	rp.maybeSweep(now)

	pn, exists := rp.nonces[peerID]
	if !exists {
		if len(rp.nonces) >= rp.maxPeers {
			rp.sweep(now)
			if len(rp.nonces) >= rp.maxPeers {
				return ErrReplayCapacity
			}
		}
		pn = &peerNonces{nonces: make(map[string]time.Time)}
		rp.nonces[peerID] = pn
	}

	if seenAt, seen := pn.nonces[nonce]; seen && now.Sub(seenAt) <= rp.ttl {
		return ErrReplay
	}

	pn.nonces[nonce] = now
	pn.lastSeen = now
	return nil
}

// maybeSweep discards expired state at most once per quarter TTL, keeping the
// amortized cost per call constant.
func (rp *ReplayProtector) maybeSweep(now time.Time) {
	if now.Sub(rp.lastSweep) >= rp.ttl/4 {
		rp.sweep(now)
	}
}

// sweep discards idle peers and expired nonces.
func (rp *ReplayProtector) sweep(now time.Time) {
	rp.lastSweep = now
	for id, pw := range rp.peers {
		if now.Sub(pw.lastSeen) > rp.ttl {
			delete(rp.peers, id)
		}
	}
	for id, pn := range rp.nonces {
		for nonce, seenAt := range pn.nonces {
			if now.Sub(seenAt) > rp.ttl {
				delete(pn.nonces, nonce)
			}
		}
		if len(pn.nonces) == 0 {
			delete(rp.nonces, id)
		}
	}
}

// Reset clears all replay state for all peers.
// Useful for testing or when restarting protection with fresh state.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) Reset() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.peers = make(map[string]*peerWindow)
	rp.nonces = make(map[string]*peerNonces)
}

// ResetPeer clears replay state for a specific peer.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) ResetPeer(peerID string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	delete(rp.peers, peerID)
	delete(rp.nonces, peerID)
}

// Stats returns current statistics for monitoring/debugging.
type Stats struct {
	PeerCount       int
	TotalSeenSeqs   int
	TotalSeenNonces int
}

// GetStats returns current replay protector statistics.
//
// PeerCount counts distinct peers across sequence and nonce mode.
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) GetStats() Stats {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	var stats Stats
	for _, pw := range rp.peers {
		stats.PeerCount++
		for _, w := range pw.seen {
			stats.TotalSeenSeqs += bits.OnesCount64(w)
		}
	}

	for id, pn := range rp.nonces {
		if _, counted := rp.peers[id]; !counted {
			stats.PeerCount++
		}
		stats.TotalSeenNonces += len(pn.nonces)
	}

	return stats
}
