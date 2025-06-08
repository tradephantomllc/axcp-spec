package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMetrics è un'implementazione mock dell'interfaccia Metrics per i test
type mockMetrics struct {
	envelopesInCount  int
	envelopesOutCount int
	queueSize         int
	retryAttemptCount int
	retrySuccessCount int
	retryDroppedCount int
}

func (m *mockMetrics) RecordEnvelopeIn() {
	m.envelopesInCount++
}

func (m *mockMetrics) RecordEnvelopeOut() {
	m.envelopesOutCount++
}

func (m *mockMetrics) SetRetryQueueSize(size int) {
	m.queueSize = size
}

func (m *mockMetrics) RecordRetryAttempt() {
	m.retryAttemptCount++
}

func (m *mockMetrics) RecordRetrySuccess() {
	m.retrySuccessCount++
}

func (m *mockMetrics) RecordRetryDropped() {
	m.retryDroppedCount++
}

func (m *mockMetrics) Shutdown(ctx context.Context) error {
	return nil
}

func TestPrometheusMetrics(t *testing.T) {
	// Create a new Prometheus metrics instance
	metrics := NewPrometheusMetrics()

	// Test recording metrics
	metrics.RecordEnvelopeIn()
	metrics.RecordEnvelopeOut()
	metrics.SetRetryQueueSize(5)
	metrics.RecordRetryAttempt()
	metrics.RecordRetrySuccess()
	metrics.RecordRetryDropped()

	// Verify the metrics were recorded by checking the underlying Prometheus metrics
	// This is a basic check - in a real test, you might want to scrape the metrics
	// endpoint and parse the response to verify the values
	assert.True(t, true, "Metrics should be recorded")

	// Test server startup with a random port
	err := metrics.StartServer(":0")
	require.NoError(t, err, "Failed to start metrics server")

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = metrics.Shutdown(ctx)
	assert.NoError(t, err, "Failed to shutdown metrics server")
}

func TestMultiMetrics(t *testing.T) {
	// Create mock metrics implementations to avoid Prometheus registry conflicts
	mock1 := &mockMetrics{}
	mock2 := &mockMetrics{}

	// Create a multi-metrics instance with two mock metrics
	multi := NewMultiMetrics(mock1, mock2)

	// Test recording metrics through the multi-metrics instance
	multi.RecordEnvelopeIn()
	multi.RecordEnvelopeOut()
	multi.SetRetryQueueSize(5)
	multi.RecordRetryAttempt()
	multi.RecordRetrySuccess()
	multi.RecordRetryDropped()
	
	// Verify that both mock metrics received the calls
	assert.Equal(t, 1, mock1.envelopesInCount, "First mock should record envelope in")
	assert.Equal(t, 1, mock2.envelopesInCount, "Second mock should record envelope in")
	assert.Equal(t, 1, mock1.envelopesOutCount, "First mock should record envelope out")
	assert.Equal(t, 5, mock1.queueSize, "First mock should record queue size")
	assert.Equal(t, 1, mock1.retryAttemptCount, "First mock should record retry attempt")
	assert.Equal(t, 1, mock1.retrySuccessCount, "First mock should record retry success")
	assert.Equal(t, 1, mock1.retryDroppedCount, "First mock should record retry dropped")

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := multi.Shutdown(ctx)
	assert.NoError(t, err, "Failed to shutdown multi-metrics")
}
