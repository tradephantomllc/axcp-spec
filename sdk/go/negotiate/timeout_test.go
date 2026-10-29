package negotiate

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// closableStream blocks every receive until Close is called.
type closableStream struct {
	once    sync.Once
	closed  chan struct{}
	pending *atomic.Int64
}

func (s *closableStream) ReceiveMessage() ([]byte, error) {
	s.pending.Add(1)
	defer s.pending.Add(-1)
	<-s.closed
	return nil, errors.New("stream closed")
}

func (s *closableStream) SendMessage([]byte) error { return nil }

func (s *closableStream) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func TestHandshakeTimeoutClosesStream(t *testing.T) {
	var pending atomic.Int64
	cfg := DefaultHandshakerConfig("did:example:server", nil, nil)
	cfg.Timeout = 5 * time.Millisecond
	h := NewHandshaker(cfg)
	for i := 0; i < 10; i++ {
		stream := &closableStream{closed: make(chan struct{}), pending: &pending}
		if _, err := h.ServerHandshake(context.Background(), stream); !errors.Is(err, ErrHandshakeTimeout) {
			t.Fatalf("error = %v, want ErrHandshakeTimeout", err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for pending.Load() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d receives still blocked after handshake timeouts", pending.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
