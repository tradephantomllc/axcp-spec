package internal

import (
	"fmt"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
)

// MockTestLogger is a simple log.Logger wrapper that implements the minimal interface required
// by RetryBuffer in place of zap.Logger
type MockTestLogger struct {
	l *log.Logger
}

func NewMockTestLogger(t *testing.T) *MockTestLogger {
	return &MockTestLogger{
		l: log.New(io.Discard, "", 0), // Silent logger for tests
	}
}

// Debug and the methods below implement the minimal logging interface
func (m *MockTestLogger) Debug(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Info(msg string, fields ...interface{})  {}
func (m *MockTestLogger) Warn(msg string, fields ...interface{})  {}
func (m *MockTestLogger) Error(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Fatal(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Sync() error                             { return nil }

func TestRetryBufferBasicOperations(t *testing.T) {
	logger := NewMockTestLogger(t)

	// Mock publish function
	publishCalls := 0
	publishMutex := sync.Mutex{}
	mockPublish := func(env *axcp.Envelope) error {
		publishMutex.Lock()
		publishCalls++
		publishMutex.Unlock()
		return nil // Always simulate success
	}

	// Short intervals for tests
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 10 * time.Millisecond
	config.MaxRetryInterval = 100 * time.Millisecond

	// Create the retry buffer
	rb := NewRetryBuffer(&config, logger, mockPublish)
	defer rb.Close()

	// Test adding an envelope
	env := axcp.NewEnvelope("test-1", 0)

	err := rb.AddEnvelope("test-1", env)
	require.NoError(t, err)
	assert.Equal(t, 1, rb.Size(), "Buffer size should be 1")

	// Start the buffer
	rb.Start()

	// Wait for the buffer to process the entry
	time.Sleep(50 * time.Millisecond)

	// Verify the entry was processed
	assert.Eventually(t, func() bool {
		return rb.Size() == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Buffer should be empty after processing")

	// Verify the publish function was called
	assert.Eventually(t, func() bool {
		publishMutex.Lock()
		defer publishMutex.Unlock()
		return publishCalls > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Publish function should have been called")
}

func TestRetryBufferDefaultsLoggerAndPublisher(t *testing.T) {
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 5 * time.Millisecond
	config.MaxRetryInterval = 10 * time.Millisecond
	config.MaxAttempts = 1

	rb := NewRetryBuffer(&config, nil, nil)
	defer rb.Close()

	err := rb.AddEnvelope("test-defaults", axcp.NewEnvelope("test-defaults", 0))
	require.NoError(t, err)
	require.NotNil(t, rb.logger)

	rb.Start()
	require.Eventually(t, func() bool {
		return rb.Size() == 0
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func TestRetryBufferFailedRetries(t *testing.T) {
	logger := NewMockTestLogger(t)

	// Mock publish function that always fails
	failingPublish := func(env *axcp.Envelope) error {
		return assert.AnError
	}

	// Reduced values for tests
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 10 * time.Millisecond
	config.MaxRetryInterval = 50 * time.Millisecond
	config.MaxAttempts = 3

	// Create the retry buffer
	rb := NewRetryBuffer(&config, logger, failingPublish)
	defer rb.Close()

	// Add an envelope
	env := axcp.NewEnvelope("test-retry", 0)

	err := rb.AddEnvelope("test-retry", env)
	require.NoError(t, err)

	// Start the buffer
	rb.Start()

	// Should make 3 attempts and then remove the entry
	// Total time should be roughly:
	// 10ms (immediate first attempt) + 20ms (second attempt) + 40ms (third attempt) = ~70ms
	time.Sleep(200 * time.Millisecond)

	// Verify the buffer is empty after max attempts
	assert.Equal(t, 0, rb.Size(), "Buffer should be empty after max attempts")
}

func TestRetryBufferCapacity(t *testing.T) {
	logger := NewMockTestLogger(t)

	// Mock publish function
	mockPublish := func(env *axcp.Envelope) error {
		return nil
	}

	// Limited capacity configuration
	config := DefaultRetryBufferConfig()
	config.MaxCapacity = 5

	// Create the retry buffer
	rb := NewRetryBuffer(&config, logger, mockPublish)

	// Add more entries than the maximum capacity
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("id-%d", i)
		env := axcp.NewEnvelope(id, 0)

		err := rb.AddEnvelope(id, env)
		if i < 5 {
			assert.NoError(t, err, "Adding elements up to capacity should succeed")
		} else {
			assert.Error(t, err, "Adding elements beyond capacity should fail")
		}
	}

	assert.Equal(t, 5, rb.Size(), "Buffer size should be limited to max capacity")
}

func TestRetryBufferStop(t *testing.T) {
	logger := NewMockTestLogger(t)

	// Synchronous mock publish function that records calls
	callMutex := sync.Mutex{}
	publishCalled := false
	mockPublish := func(env *axcp.Envelope) error {
		callMutex.Lock()
		publishCalled = true
		callMutex.Unlock()
		return nil
	}

	// Create the retry buffer with a fast configuration for tests
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 5 * time.Millisecond // Reduced for faster tests
	config.MaxRetryInterval = 20 * time.Millisecond
	rb := NewRetryBuffer(&config, logger, mockPublish)

	// Add a few entries to the buffer
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("test-stop-%d", i)
		env := axcp.NewEnvelope(id, 0)
		err := rb.AddEnvelope(id, env)
		require.NoError(t, err)
	}

	// Verify initial size
	assert.Equal(t, 3, rb.Size(), "Buffer should contain 3 items")

	// Start the buffer
	rb.Start()

	// Wait for publish to be called
	require.Eventually(t, func() bool {
		callMutex.Lock()
		defer callMutex.Unlock()
		return publishCalled
	}, 100*time.Millisecond, 5*time.Millisecond, "Publish function should be called")

	// Wait for the queue to drain
	require.Eventually(t, func() bool {
		return rb.Size() == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Queue should be empty after processing")

	// Stop the buffer
	rb.Stop()

	// Verify no new entries can be added after stop
	env2 := axcp.NewEnvelope("test-after-stop", 0)
	err := rb.AddEnvelope("test-after-stop", env2)
	assert.Error(t, err, "Adding elements after stop should fail")
}

func TestRetryBufferBackoff(t *testing.T) {
	logger := NewMockTestLogger(t)

	// Track call times to verify backoff
	callTimes := []time.Time{}
	callTimesMutex := sync.Mutex{}

	failingPublish := func(env *axcp.Envelope) error {
		callTimesMutex.Lock()
		callTimes = append(callTimes, time.Now())
		callTimesMutex.Unlock()
		return assert.AnError
	}

	// Exponential backoff configuration
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 20 * time.Millisecond
	config.BackoffFactor = 2.0
	config.MaxAttempts = 4

	// Create the retry buffer
	rb := NewRetryBuffer(&config, logger, failingPublish)

	// Add an envelope
	env := axcp.NewEnvelope("test-backoff", 0)

	err := rb.AddEnvelope("test-backoff", env)
	require.NoError(t, err)

	// Start the buffer
	rb.Start()
	t.Cleanup(rb.Stop)

	// Wait until the item is dropped after MaxAttempts (20ms + 40ms + 80ms of
	// backoff at minimum); allow generous time for loaded CI runners.
	require.Eventually(t, func() bool { return rb.Size() == 0 }, 5*time.Second, 5*time.Millisecond,
		"Buffer should be empty after max attempts")

	callTimesMutex.Lock()
	defer callTimesMutex.Unlock()
	require.GreaterOrEqual(t, len(callTimes), 3, "Should have at least 3 retry attempts")

	// Each retry must wait at least the exponential backoff interval. Only the
	// lower bound is deterministic: scheduling delays can lengthen any interval.
	const slack = 2 * time.Millisecond
	expected := config.MinRetryInterval
	for i := 1; i < len(callTimes); i++ {
		gap := callTimes[i].Sub(callTimes[i-1])
		t.Logf("Interval %d: %v (backoff %v)", i, gap, expected)
		assert.GreaterOrEqual(t, gap, expected-slack, "retry %d came %v after the previous attempt, before its %v backoff", i+1, gap, expected)
		expected = time.Duration(float64(expected) * config.BackoffFactor)
	}
}
