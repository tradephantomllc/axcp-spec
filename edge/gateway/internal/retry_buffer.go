package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
)

var errRetryPublisherNotConfigured = errors.New("retry buffer publisher is not configured")

// RetryBufferConfig holds the configuration for the retry buffer
type RetryBufferConfig struct {
	// Maximum in-memory buffer capacity
	MaxCapacity int
	// Minimum interval between retransmission attempts
	MinRetryInterval time.Duration
	// Maximum interval between retransmission attempts
	MaxRetryInterval time.Duration
	// Backoff factor for retransmission attempts (e.g. 2.0 = double the delay)
	BackoffFactor float64
	// Maximum number of attempts before a message is dropped
	MaxAttempts int
}

// RetryItem represents an entry in the retry buffer
type RetryItem struct {
	inFlight   bool
	generation uint64
	// Unique message ID
	ID string
	// AXCP envelope contents
	Envelope *axcp.Envelope
	// Creation timestamp
	CreatedAt time.Time
	// Timestamp of the last send attempt
	LastAttempt time.Time
	// Number of attempts made
	Attempts int
	// Next retry interval
	NextRetryInterval time.Duration
}

// Logger defines the minimal interface required for a compatible logger
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	Sync() error
}

// StandardLogger wraps log.Logger and implements the Logger interface
type StandardLogger struct {
	l *log.Logger
}

// NewStandardLogger creates a new standard logger
func NewStandardLogger(prefix string) *StandardLogger {
	return &StandardLogger{
		l: log.New(log.Writer(), prefix, log.LstdFlags|log.Lshortfile),
	}
}

// Debug and the methods below implement the Logger interface
func (s *StandardLogger) Debug(msg string, fields ...interface{}) {
	s.l.Printf("DEBUG: %s %v", msg, fields)
}

func (s *StandardLogger) Info(msg string, fields ...interface{}) {
	s.l.Printf("INFO: %s %v", msg, fields)
}

func (s *StandardLogger) Warn(msg string, fields ...interface{}) {
	s.l.Printf("WARN: %s %v", msg, fields)
}

func (s *StandardLogger) Error(msg string, fields ...interface{}) {
	s.l.Printf("ERROR: %s %v", msg, fields)
}

func (s *StandardLogger) Fatal(msg string, fields ...interface{}) {
	s.l.Fatalf("FATAL: %s %v", msg, fields)
}

func (s *StandardLogger) Sync() error {
	return nil
}

// RetryBuffer implements an in-memory buffer for AXCP messages awaiting retransmission
type RetryBuffer struct {
	config      RetryBufferConfig
	items       map[string]*RetryItem
	queue       []string
	mutex       sync.RWMutex
	logger      Logger
	metrics     *RetryMetrics
	publishFn   func(*axcp.Envelope) error
	ctx         context.Context
	cancelFunc  context.CancelFunc
	stopped     bool
	started     bool
	configError error
	wg          sync.WaitGroup
}

// DefaultRetryBufferConfig returns the default retry buffer configuration
func DefaultRetryBufferConfig() RetryBufferConfig {
	return RetryBufferConfig{
		MaxCapacity:      1000,
		MinRetryInterval: 1 * time.Second,
		MaxRetryInterval: 5 * time.Minute,
		BackoffFactor:    2.0,
		MaxAttempts:      5,
	}
}

// NewRetryBuffer creates a new retry buffer
func NewRetryBuffer(config *RetryBufferConfig, logger Logger, publishFn func(*axcp.Envelope) error) *RetryBuffer {
	if config == nil {
		defaultConfig := DefaultRetryBufferConfig()
		config = &defaultConfig
	}
	if logger == nil {
		logger = NewStandardLogger("[retry-buffer] ")
	}
	if publishFn == nil {
		publishFn = func(*axcp.Envelope) error {
			return errRetryPublisherNotConfigured
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	rb := &RetryBuffer{
		config:      *config,
		items:       make(map[string]*RetryItem),
		queue:       make([]string, 0),
		configError: config.Validate(),
		logger:      logger,
		publishFn:   publishFn,
		ctx:         ctx,
		cancelFunc:  cancel,
	}

	return rb
}

// SetMetrics attaches metrics to the retry buffer
func (rb *RetryBuffer) SetMetrics(metrics *RetryMetrics) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	rb.metrics = metrics
}

// AddEnvelope adds an envelope to the retry buffer
func (rb *RetryBuffer) AddEnvelope(id string, env *axcp.Envelope) error {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	if rb.configError != nil {
		return rb.configError
	}
	if id == "" {
		return errors.New("retry item ID is required")
	}
	if env == nil {
		return errors.New("retry envelope is required")
	}
	if rb.stopped {
		return fmt.Errorf("retry buffer is stopped")
	}

	clone := axcp.CloneEnvelope(&env.AxcpEnvelope)
	if item, exists := rb.items[id]; exists {
		item.Envelope = clone
		item.generation++
		item.Attempts = 0
		item.NextRetryInterval = rb.config.MinRetryInterval
		return nil
	}
	if len(rb.queue) >= rb.config.MaxCapacity {
		if rb.metrics != nil {
			rb.metrics.RecordRetryDropped(rb.ctx)
		}
		return fmt.Errorf("retry buffer is full (capacity: %d)", rb.config.MaxCapacity)
	}

	// Create a new entry
	item := &RetryItem{
		ID:                id,
		Envelope:          clone,
		CreatedAt:         time.Now(),
		LastAttempt:       time.Time{}, // Zero time
		Attempts:          0,
		NextRetryInterval: rb.config.MinRetryInterval,
	}

	// Add the entry to the buffer and the queue
	rb.items[id] = item
	rb.queue = append(rb.queue, id)

	if rb.metrics != nil {
		rb.metrics.SetRetryQueueSize(rb.ctx, int64(len(rb.queue)))
	}

	rb.logger.Debug("Added envelope to retry buffer", "id", id, "queue_size", len(rb.queue))

	return nil
}

// Start launches the retry worker
func (rb *RetryBuffer) Start() {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	if rb.stopped || rb.started {
		return
	}
	if rb.configError != nil {
		rb.logger.Error("Retry worker configuration invalid", "error", rb.configError)
		return
	}
	rb.started = true
	rb.wg.Add(1)
	go rb.processRetries()
}

// Stop halts the retry worker
func (rb *RetryBuffer) Stop() {
	// Mark the buffer as stopped
	rb.mutex.Lock()
	rb.stopped = true
	rb.mutex.Unlock()

	// Cancel the context for all pending operations
	rb.cancelFunc()

	// Wait for all goroutines to finish
	rb.wg.Wait()

	// Debug log
	if rb.logger != nil {
		rb.logger.Debug("Retry buffer stopped successfully")
	}
}

// Size returns the number of entries in the buffer
func (rb *RetryBuffer) Size() int {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()
	return len(rb.queue)
}

// processRetries is the worker that processes entries in the retry queue
func (rb *RetryBuffer) processRetries() {
	defer rb.wg.Done()

	// Use a short polling interval (10ms) so tests run quickly
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	if rb.logger != nil {
		rb.logger.Debug("Retry worker started with interval", "interval_ms", 10)
	}

	for {
		select {
		case <-rb.ctx.Done():
			return
		case <-ticker.C:
			rb.retryPendingItems()
		}
	}
}

// retryPendingItems processes entries that are ready to be retried
func (rb *RetryBuffer) retryPendingItems() {
	now := time.Now()
	var toRetry []*RetryItem

	// Phase one: find entries ready to be retried
	rb.mutex.RLock()
	for _, id := range rb.queue {
		item := rb.items[id]
		// First attempt, or enough time has passed since the last attempt
		if !item.inFlight && (item.Attempts == 0 || now.Sub(item.LastAttempt) >= item.NextRetryInterval) {
			toRetry = append(toRetry, item)
		}
	}
	rb.mutex.RUnlock()

	// Phase two: process the ready entries
	for _, item := range toRetry {
		rb.processRetryItem(item)
	}
}

// processRetryItem handles the retry of a single entry
func (rb *RetryBuffer) processRetryItem(item *RetryItem) {
	// Take the lock before modifying the item and hold it until the
	// function returns to avoid race conditions
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	// Check that the item is still in the buffer;
	// another goroutine may have removed it
	if current, exists := rb.items[item.ID]; !exists || current != item || item.inFlight || rb.stopped {
		return
	}

	// Increment the attempt counter and record the last attempt
	item.inFlight = true
	generation := item.generation
	item.Attempts++
	item.LastAttempt = time.Now()

	// Copy the envelope so the network call runs without holding the mutex.
	envelopeCopy := axcp.CloneEnvelope(&item.Envelope.AxcpEnvelope)

	// Release the mutex during publication.
	rb.mutex.Unlock()

	// Try to publish the envelope
	err := rb.publishFn(envelopeCopy)

	// Reacquire the mutex for the rest of the function
	rb.mutex.Lock()

	item.inFlight = false
	if item.generation != generation {
		return
	}

	// On successful publication, remove the entry from the buffer
	if err == nil {
		rb.removeItemLocked(item.ID)
		rb.logger.Debug("Retry successful, removed from buffer", "id", item.ID, "attempts", item.Attempts)

		if rb.metrics != nil {
			rb.metrics.RecordRetrySuccess(rb.ctx)
			rb.metrics.SetRetryQueueSize(rb.ctx, int64(len(rb.queue)))
		}
		return
	}

	// Publication failed
	if rb.metrics != nil {
		rb.metrics.RecordRetryAttempt(rb.ctx)
	}

	// Drop the entry once the maximum number of attempts is reached
	if item.Attempts >= rb.config.MaxAttempts {
		rb.removeItemLocked(item.ID)
		rb.logger.Warn("Max retry attempts reached, dropped from buffer",
			"id", item.ID,
			"max_attempts", rb.config.MaxAttempts,
			"error", err)

		if rb.metrics != nil {
			rb.metrics.RecordRetryDropped(rb.ctx)
			rb.metrics.SetRetryQueueSize(rb.ctx, int64(len(rb.queue)))
		}
		return
	}

	// Compute the next retry interval with exponential backoff
	delay := float64(rb.config.MinRetryInterval) * math.Pow(rb.config.BackoffFactor, float64(item.Attempts-1))
	nextInterval := rb.config.MaxRetryInterval
	if delay < float64(rb.config.MaxRetryInterval) {
		nextInterval = time.Duration(delay)
	}

	item.NextRetryInterval = nextInterval

	if rb.metrics != nil {
		rb.metrics.RecordRetryDelay(rb.ctx, nextInterval.Seconds())
	}

	rb.logger.Debug("Retry failed, scheduled next attempt",
		"id", item.ID,
		"attempts", item.Attempts,
		"next_interval", nextInterval,
		"error", err)
}

// removeItemLocked removes an entry from the buffer (the caller must hold the mutex)
func (rb *RetryBuffer) removeItemLocked(id string) {
	// Remove the entry from the map
	delete(rb.items, id)

	// Remove the entry from the queue
	for i, queueID := range rb.queue {
		if queueID == id {
			// Efficient removal without preserving order
			rb.queue[i] = rb.queue[len(rb.queue)-1]
			rb.queue = rb.queue[:len(rb.queue)-1]
			break
		}
	}
}

// Close implements the io.Closer interface
func (rb *RetryBuffer) Close() error {
	rb.Stop()
	return nil
}

// Validate rejects configurations that cannot provide bounded retry behavior.
func (c RetryBufferConfig) Validate() error {
	if c.MaxCapacity <= 0 {
		return errors.New("retry capacity must be greater than zero")
	}
	if c.MaxAttempts <= 0 {
		return errors.New("retry attempts must be greater than zero")
	}
	if c.MinRetryInterval <= 0 {
		return errors.New("retry minimum interval must be positive")
	}
	if c.MaxRetryInterval < c.MinRetryInterval {
		return errors.New("retry maximum interval must be at least the minimum interval")
	}
	if math.IsNaN(c.BackoffFactor) || math.IsInf(c.BackoffFactor, 0) || c.BackoffFactor < 1 {
		return errors.New("retry backoff factor must be finite and at least one")
	}
	return nil
}
