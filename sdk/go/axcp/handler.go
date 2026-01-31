// Package axcp provides AXCP protocol types and handlers.
package axcp

import (
	"context"
	"errors"
	"sync"
)

// Handler errors
var (
	// ErrNoHandler indicates no handler was found for the message
	ErrNoHandler = errors.New("axcp: no handler for message")

	// ErrHandlerPanic indicates a handler panicked
	ErrHandlerPanic = errors.New("axcp: handler panic")
)

// Handler processes incoming AXCP envelopes and returns a response.
type Handler interface {
	// HandleEnvelope processes an envelope and returns a response envelope.
	// Returns nil, nil if no response should be sent.
	// Returns nil, error if processing failed.
	HandleEnvelope(ctx context.Context, req *Envelope) (*Envelope, error)
}

// HandlerFunc is an adapter to allow ordinary functions to be used as Handlers.
type HandlerFunc func(ctx context.Context, req *Envelope) (*Envelope, error)

// HandleEnvelope implements Handler interface.
func (f HandlerFunc) HandleEnvelope(ctx context.Context, req *Envelope) (*Envelope, error) {
	return f(ctx, req)
}

// Router routes incoming envelopes to registered handlers.
type Router struct {
	handlers       map[string]Handler
	defaultHandler Handler
	mu             sync.RWMutex
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]Handler),
	}
}

// Register registers a handler for a specific message type or route.
// The key can be a message type name, a route path, or any string identifier.
func (r *Router) Register(key string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[key] = handler
}

// RegisterFunc is a convenience method to register a HandlerFunc.
func (r *Router) RegisterFunc(key string, f func(ctx context.Context, req *Envelope) (*Envelope, error)) {
	r.Register(key, HandlerFunc(f))
}

// SetDefault sets the default handler for unmatched messages.
func (r *Router) SetDefault(handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultHandler = handler
}

// SetDefaultFunc is a convenience method to set a default HandlerFunc.
func (r *Router) SetDefaultFunc(f func(ctx context.Context, req *Envelope) (*Envelope, error)) {
	r.SetDefault(HandlerFunc(f))
}

// Handle routes an envelope to the appropriate handler.
// It first tries to find a handler by the envelope's route/type.
// If no specific handler is found, it uses the default handler.
// Returns ErrNoHandler if no handler is available.
func (r *Router) Handle(ctx context.Context, env *Envelope) (*Envelope, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try to find handler by message payload type
	key := getRouteKey(env)

	if handler, ok := r.handlers[key]; ok {
		return handler.HandleEnvelope(ctx, env)
	}

	// Fall back to default handler
	if r.defaultHandler != nil {
		return r.defaultHandler.HandleEnvelope(ctx, env)
	}

	return nil, ErrNoHandler
}

// HandleEnvelope implements Handler interface, allowing Router to be composed.
func (r *Router) HandleEnvelope(ctx context.Context, req *Envelope) (*Envelope, error) {
	return r.Handle(ctx, req)
}

// getRouteKey extracts the routing key from an envelope.
// This examines the payload type to determine routing.
func getRouteKey(env *Envelope) string {
	if env == nil {
		return ""
	}

	// Check payload type by trying to get each type
	if env.GetContextPatch() != nil {
		return "context_patch"
	}
	if env.GetCapabilityMsg() != nil {
		return "capability"
	}
	if env.GetRouteMsg() != nil {
		return "route"
	}
	if env.GetError() != nil {
		return "error"
	}
	if env.GetProfileNeg() != nil {
		return "profile_negotiate"
	}
	if env.GetProfileAck() != nil {
		return "profile_ack"
	}
	if env.GetRetryEnv() != nil {
		return "retry"
	}
	if env.GetTelemetry() != nil {
		return "telemetry"
	}

	// Default: use trace_id prefix if present for custom routing
	if env.TraceId != "" && len(env.TraceId) >= 8 {
		return env.TraceId[:8]
	}

	return "default"
}

// ConnectionHandler handles a connection's message loop.
type ConnectionHandler struct {
	router  Handler
	onError func(error)
}

// NewConnectionHandler creates a handler for processing connection messages.
func NewConnectionHandler(router Handler) *ConnectionHandler {
	return &ConnectionHandler{
		router: router,
	}
}

// OnError sets an error callback for non-fatal errors.
func (h *ConnectionHandler) OnError(f func(error)) {
	h.onError = f
}

// Connection defines the interface for a bidirectional envelope connection.
type Connection interface {
	// RecvEnvelope receives an envelope from the connection.
	RecvEnvelope() (*Envelope, error)

	// SendEnvelope sends an envelope over the connection.
	SendEnvelope(env *Envelope) error
}

// Serve handles envelopes from the connection until context is canceled or an error occurs.
func (h *ConnectionHandler) Serve(ctx context.Context, conn Connection) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Receive envelope
		env, err := conn.RecvEnvelope()
		if err != nil {
			return err
		}

		// Handle envelope
		resp, err := h.router.HandleEnvelope(ctx, env)
		if err != nil {
			if h.onError != nil {
				h.onError(err)
			}
			// Continue processing - don't break the loop on handler errors
			continue
		}

		// Send response if provided
		if resp != nil {
			if err := conn.SendEnvelope(resp); err != nil {
				return err
			}
		}
	}
}

// NewEchoHandler creates a handler that echoes back the received envelope.
// Useful for testing.
func NewEchoHandler() Handler {
	return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})
}

// NewNoopHandler creates a handler that does nothing and returns no response.
func NewNoopHandler() Handler {
	return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return nil, nil
	})
}
