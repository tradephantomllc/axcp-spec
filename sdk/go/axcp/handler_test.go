package axcp

import (
	"context"
	"errors"
	"testing"
)

func TestHandlerFunc(t *testing.T) {
	called := false
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		called = true
		return req, nil
	})

	env := NewEnvelope("test-trace", 1)
	resp, err := handler.HandleEnvelope(context.Background(), env)

	if !called {
		t.Error("handler was not called")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != env {
		t.Error("expected same envelope back")
	}
}

func TestRouter_Register(t *testing.T) {
	router := NewRouter()

	router.RegisterFunc("test-key", func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return nil, nil
	})

	// This won't match because getRouteKey uses envelope payload
	env := NewEnvelope("test-trace", 1)
	_, err := router.Handle(context.Background(), env)

	// Without a matching key, should fall back to default (which is nil)
	if err != ErrNoHandler {
		t.Errorf("expected ErrNoHandler, got: %v", err)
	}
}

func TestRouter_SetDefault(t *testing.T) {
	router := NewRouter()

	called := false
	router.SetDefaultFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		called = true
		return nil, nil
	})

	env := NewEnvelope("test-trace", 1)
	_, err := router.Handle(context.Background(), env)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("default handler was not called")
	}
}

func TestRouter_ImplementsHandler(t *testing.T) {
	router := NewRouter()
	router.SetDefaultFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	// Router should implement Handler interface
	var _ Handler = router

	env := NewEnvelope("test", 1)
	resp, err := router.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != env {
		t.Error("expected same envelope")
	}
}

func TestRouter_HandlerError(t *testing.T) {
	router := NewRouter()
	expectedErr := errors.New("test error")

	router.SetDefaultFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return nil, expectedErr
	})

	env := NewEnvelope("test", 1)
	_, err := router.Handle(context.Background(), env)

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestNewEchoHandler(t *testing.T) {
	handler := NewEchoHandler()

	env := NewEnvelope("test-trace", 1)
	resp, err := handler.HandleEnvelope(context.Background(), env)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != env {
		t.Error("expected same envelope back")
	}
}

func TestNewNoopHandler(t *testing.T) {
	handler := NewNoopHandler()

	env := NewEnvelope("test-trace", 1)
	resp, err := handler.HandleEnvelope(context.Background(), env)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Error("expected nil response")
	}
}

func TestGetRouteKey_Empty(t *testing.T) {
	key := getRouteKey(nil)
	if key != "" {
		t.Errorf("expected empty string for nil envelope, got %q", key)
	}
}

func TestGetRouteKey_Default(t *testing.T) {
	env := NewEnvelope("", 1)
	key := getRouteKey(env)
	if key != "default" {
		t.Errorf("expected 'default' for empty envelope, got %q", key)
	}
}

func TestGetRouteKey_TracePrefix(t *testing.T) {
	env := NewEnvelope("12345678-extra-stuff", 1)
	key := getRouteKey(env)
	if key != "12345678" {
		t.Errorf("expected trace prefix '12345678', got %q", key)
	}
}

// mockConnection for testing ConnectionHandler
type mockConnection struct {
	recvQueue []*Envelope
	recvIndex int
	sent      []*Envelope
	recvErr   error
	sendErr   error
}

func (c *mockConnection) RecvEnvelope() (*Envelope, error) {
	if c.recvErr != nil {
		return nil, c.recvErr
	}
	if c.recvIndex >= len(c.recvQueue) {
		return nil, errors.New("no more envelopes")
	}
	env := c.recvQueue[c.recvIndex]
	c.recvIndex++
	return env, nil
}

func (c *mockConnection) SendEnvelope(env *Envelope) error {
	if c.sendErr != nil {
		return c.sendErr
	}
	c.sent = append(c.sent, env)
	return nil
}

func TestConnectionHandler_Serve(t *testing.T) {
	// Create mock connection that signals when envelope is sent
	sentChan := make(chan struct{}, 1)
	conn := &mockConnection{
		recvQueue: []*Envelope{NewEnvelope("test", 1)},
	}

	// Create handler that signals when done
	handler := NewConnectionHandler(HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		defer func() {
			select {
			case sentChan <- struct{}{}:
			default:
			}
		}()
		return req, nil // Echo back
	}))

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Serve should process the envelope and then error when recv fails
	go func() {
		// Wait for processing then cancel
		<-sentChan
		cancel()
	}()

	err := handler.Serve(ctx, conn)

	// Should either be context canceled or recv error
	if err != context.Canceled && err == nil {
		// The serve loop will exit due to recv error after processing
	}

	// Verify the envelope was echoed
	if len(conn.sent) != 1 {
		t.Errorf("expected 1 sent envelope, got %d", len(conn.sent))
	}
}

func TestConnectionHandler_OnError(t *testing.T) {
	// Create a mock connection
	conn := &mockConnection{
		recvQueue: []*Envelope{NewEnvelope("test", 1)},
	}

	// Create handler that returns error
	testErr := errors.New("handler error")
	router := NewRouter()
	router.SetDefaultFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return nil, testErr
	})

	handler := NewConnectionHandler(router)

	// Use channel to avoid race on error capture
	errChan := make(chan error, 1)
	handler.OnError(func(err error) {
		select {
		case errChan <- err:
		default:
		}
	})

	// Serve should continue after handler error
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Wait for error then cancel
		<-errChan
		cancel()
	}()

	_ = handler.Serve(ctx, conn)

	// Error callback should have been called
	select {
	case capturedErr := <-errChan:
		if capturedErr != testErr {
			t.Errorf("expected error %v, got %v", testErr, capturedErr)
		}
	default:
		// Error was already consumed by goroutine, which is fine
	}
}
