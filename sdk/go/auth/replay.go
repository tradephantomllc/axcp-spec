package auth

import (
	"errors"
	"sync"
	"time"
)

// Replay protection errors
var (
	// ErrReplay indicates the sequence/nonce was already seen within the TTL window
	ErrReplay = errors.New("auth: replay detected")

	// ErrTooOld indicates the sequence is below the sliding window threshold
	ErrTooOld = errors.New("auth: sequence too old (outside sliding window)")

	// ErrInvalidReplayParams indicates invalid parameters for replay protection
	ErrInvalidReplayParams = errors.New("auth: invalid replay parameters")

	// ErrEmptyPeerID indicates an empty peer identifier was provided
	ErrEmptyPeerID = errors.New("auth: peer ID cannot be empty")

	// ErrEmptyNonce indicates an empty nonce was provided
	ErrEmptyNonce = errors.New("auth: nonce cannot be empty")
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
type peerWindow struct {
	highest uint64
	seen    map[uint64]time.Time
}

// nonceEntry tracks when a nonce was first seen.
type nonceEntry struct {
	seenAt time.Time
}

// ReplayProtector provides thread-safe replay attack protection.
//
// It supports two modes:
//  1. Sequence mode: per-peer sliding window with monotonic sequence numbers
//  2. Nonce mode: per-peer nonce tracking with TTL expiration
//
// Both modes enforce TTL-based expiration of seen entries.
type ReplayProtector struct {
	mu     sync.Mutex
	clock  Clock
	ttl    time.Duration
	window uint64

	// peers tracks sequence-based replay state per peer
	peers map[string]*peerWindow

	// nonces tracks nonce-based replay state per peer
	nonces map[string]map[string]*nonceEntry
}

// NewReplayProtector creates a new replay protector.
//
// Parameters:
//   - ttl: Time-to-live for seen sequences/nonces. After TTL expires, the same
//     sequence/nonce can be accepted again. Must be > 0.
//   - window: Sliding window size for sequence mode. Sequences more than
//     `window` behind the highest seen are rejected as too old. Must be >= 1.
//   - clock: Time source for TTL calculations. If nil, uses real system clock.
//
// Returns error if ttl <= 0 or window < 1.
func NewReplayProtector(ttl time.Duration, window uint64, clock Clock) (*ReplayProtector, error) {
	if ttl <= 0 {
		return nil, ErrInvalidReplayParams
	}
	if window < 1 {
		return nil, ErrInvalidReplayParams
	}
	if clock == nil {
		clock = RealClock{}
	}
	return &ReplayProtector{
		clock:  clock,
		ttl:    ttl,
		window: window,
		peers:  make(map[string]*peerWindow),
		nonces: make(map[string]map[string]*nonceEntry),
	}, nil
}

// CheckAndMark checks if a sequence number is a replay and marks it as seen.
//
// For a given peerID and sequence number:
//  1. Rejects if peerID is empty (ErrEmptyPeerID)
//  2. Cleans up expired entries based on TTL
//  3. Rejects if seq was already seen within TTL (ErrReplay)
//  4. Rejects if seq is too old: seq + window < highest (ErrTooOld)
//  5. Accepts and marks seq as seen, updating highest if seq > highest
//
// The sliding window policy:
//   - Let H = highest seen sequence for this peer
//   - Accept if: seq > H - window (i.e., within the last `window` sequences)
//   - Reject as too old if: seq <= H - window
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) CheckAndMark(peerID string, seq uint64) error {
	if peerID == "" {
		return ErrEmptyPeerID
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	now := rp.clock.Now()

	// Get or create peer window
	pw, exists := rp.peers[peerID]
	if !exists {
		pw = &peerWindow{
			highest: 0,
			seen:    make(map[uint64]time.Time),
		}
		rp.peers[peerID] = pw
	}

	// Cleanup expired entries
	rp.cleanupPeerExpired(pw, now)

	// Check if already seen (replay)
	if _, seen := pw.seen[seq]; seen {
		return ErrReplay
	}

	// Check sliding window (only if we have seen sequences before)
	if pw.highest > 0 {
		// Calculate the minimum acceptable sequence
		// If highest=100 and window=5, accept seq >= 96 (i.e., 96,97,98,99,100,101,...)
		// Reject seq < 96 (i.e., 95,94,... are too old)
		var minAcceptable uint64
		if pw.highest >= rp.window {
			minAcceptable = pw.highest - rp.window + 1
		} else {
			minAcceptable = 0 // Allow seq=0 when highest < window
		}

		if seq < minAcceptable {
			return ErrTooOld
		}
	}

	// Mark as seen
	pw.seen[seq] = now

	// Update highest if needed
	if seq > pw.highest {
		pw.highest = seq
	}

	return nil
}

// cleanupPeerExpired removes expired entries from a peer's seen map.
func (rp *ReplayProtector) cleanupPeerExpired(pw *peerWindow, now time.Time) {
	cutoff := now.Add(-rp.ttl)
	for seq, seenAt := range pw.seen {
		if seenAt.Before(cutoff) {
			delete(pw.seen, seq)
		}
	}
}

// CheckAndMarkNonce checks if a nonce is a replay and marks it as seen.
//
// For a given peerID and nonce:
//  1. Rejects if peerID is empty (ErrEmptyPeerID)
//  2. Rejects if nonce is empty (ErrEmptyNonce)
//  3. Cleans up expired nonces based on TTL
//  4. Rejects if nonce was already seen within TTL (ErrReplay)
//  5. Accepts and marks nonce as seen
//
// Unlike sequence mode, nonce mode has no ordering constraints.
// Each nonce is simply tracked for TTL duration.
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

	// Get or create nonce map for peer
	peerNonces, exists := rp.nonces[peerID]
	if !exists {
		peerNonces = make(map[string]*nonceEntry)
		rp.nonces[peerID] = peerNonces
	}

	// Cleanup expired nonces
	rp.cleanupNoncesExpired(peerNonces, now)

	// Check if already seen (replay)
	if _, seen := peerNonces[nonce]; seen {
		return ErrReplay
	}

	// Mark as seen
	peerNonces[nonce] = &nonceEntry{seenAt: now}

	return nil
}

// cleanupNoncesExpired removes expired nonces from a peer's nonce map.
func (rp *ReplayProtector) cleanupNoncesExpired(nonces map[string]*nonceEntry, now time.Time) {
	cutoff := now.Add(-rp.ttl)
	for nonce, entry := range nonces {
		if entry.seenAt.Before(cutoff) {
			delete(nonces, nonce)
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
	rp.nonces = make(map[string]map[string]*nonceEntry)
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
// Thread-safe: can be called concurrently from multiple goroutines.
func (rp *ReplayProtector) GetStats() Stats {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	stats := Stats{
		PeerCount: len(rp.peers) + len(rp.nonces),
	}

	for _, pw := range rp.peers {
		stats.TotalSeenSeqs += len(pw.seen)
	}

	for _, peerNonces := range rp.nonces {
		stats.TotalSeenNonces += len(peerNonces)
	}

	return stats
}
