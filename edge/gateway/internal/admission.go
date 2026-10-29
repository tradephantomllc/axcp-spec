package internal

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

var errPreAuthBudgetExceeded = errors.New("unauthenticated frame budget exceeded")

// admission bounds connections that have not yet delivered an authenticated
// envelope, in total and per remote address, inside the overall connection
// limit. Authenticated connections leave the pre-auth pool, so peers that
// never authenticate cannot take every connection slot.
type admission struct {
	mu              sync.Mutex
	active          int64
	preAuth         int64
	preAuthByIP     map[string]int
	maxActive       int64
	maxPreAuth      int64
	maxPreAuthPerIP int
	budget          *byteBudget
}

func newAdmission(c ServerConfig) *admission {
	return &admission{
		preAuthByIP:     make(map[string]int),
		maxActive:       c.maxConnections(),
		maxPreAuth:      c.maxPreAuthConnections(),
		maxPreAuthPerIP: c.maxPreAuthConnectionsPerIP(),
		budget:          &byteBudget{max: c.maxPreAuthBufferedBytes()},
	}
}

// admit reserves a connection slot and a pre-auth slot for ip.
func (a *admission) admit(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active >= a.maxActive || a.preAuth >= a.maxPreAuth || a.preAuthByIP[ip] >= a.maxPreAuthPerIP {
		return false
	}
	a.active++
	a.preAuth++
	a.preAuthByIP[ip]++
	return true
}

func (a *admission) leavePreAuthLocked(ip string) {
	a.preAuth--
	if a.preAuthByIP[ip]--; a.preAuthByIP[ip] <= 0 {
		delete(a.preAuthByIP, ip)
	}
}

func (a *admission) authenticated(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.leavePreAuthLocked(ip)
}

func (a *admission) release(ip string, preAuth bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active--
	if preAuth {
		a.leavePreAuthLocked(ip)
	}
}

func (a *admission) counts() (active, preAuth int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active, a.preAuth
}

// connState records whether a connection has authenticated and releases its
// admission slots exactly once, whichever stream or datagram finishes last.
type connState struct {
	mu            sync.Mutex
	a             *admission
	ip            string
	authenticated bool
	released      bool
}

func (s *connState) isAuthenticated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authenticated
}

func (s *connState) markAuthenticated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.authenticated || s.released {
		return
	}
	s.authenticated = true
	s.a.authenticated(s.ip)
}

func (s *connState) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.released {
		return
	}
	s.released = true
	s.a.release(s.ip, !s.authenticated)
}

// byteBudget bounds frame bytes buffered for unauthenticated connections. It is
// charged with bytes actually received, never with a peer's announced size.
type byteBudget struct {
	used atomic.Int64
	max  int64
}

func (b *byteBudget) tryAcquire(n int64) bool {
	if b.used.Add(n) > b.max {
		b.used.Add(-n)
		return false
	}
	return true
}

func (b *byteBudget) release(n int64) {
	if b != nil && n > 0 {
		b.used.Add(-n)
	}
}

func remoteIP(addr net.Addr) string {
	if u, ok := addr.(*net.UDPAddr); ok {
		return u.IP.String()
	}
	if host, _, err := net.SplitHostPort(addr.String()); err == nil {
		return host
	}
	return addr.String()
}
