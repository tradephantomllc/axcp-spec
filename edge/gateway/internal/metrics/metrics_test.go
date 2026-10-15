package metrics

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetTestState is a helper that clears state between tests
func resetTestState() {
	// Use ShutdownOTEL to cleanly stop the batch processor goroutine
	ShutdownOTEL()

	// Extra cleanup for previous tests
	batchMutex.Lock()
	defer batchMutex.Unlock()

	// Make sure the state is fully cleared
	batchingEnabled = false
	batchBuffer = nil
}

func TestHistogramObserve(t *testing.T) {
	// Reset test state
	resetTestState()

	// Find a free port for the test
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() // Close the listener but keep the port for the test

	// Initialize Prometheus with the histogram on the random port
	endpoint := fmt.Sprintf(":%d", port)
	err = InitPrometheus(endpoint, true)
	require.NoError(t, err)

	// Verify the histogram was created
	require.NotNil(t, RPCLatency)

	// Observe a latency value
	RPCLatency.WithLabelValues("Sync", "200", "edge").Observe(0.12)

	// Wait briefly to make sure the server has started
	time.Sleep(100 * time.Millisecond)

	// Make an HTTP request to fetch the metrics
	url := fmt.Sprintf("http://localhost:%d/metrics", port)
	res, err := http.Get(url)
	require.NoError(t, err)
	defer res.Body.Close()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	// Verify the histogram is present in the metrics
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "rpc_duration_seconds_bucket")
	assert.Contains(t, bodyStr, "method=\"Sync\"")
	assert.Contains(t, bodyStr, "status_code=\"200\"")
	assert.Contains(t, bodyStr, "node_type=\"edge\"")
}

func TestOTELBatching(t *testing.T) {
	// Reset test state
	resetTestState()

	// Initialize OTEL with batching
	err := InitOTELExporter("localhost:4317", true, 100*time.Millisecond)
	require.NoError(t, err)

	// Verify batching is enabled
	assert.True(t, batchingEnabled)

	// Add an observation to the batch
	BatchObserve("Sync", 150*time.Millisecond)

	// Verify the observation was added to the batch
	assert.Equal(t, 1, BatchSize())

	// Wait for the batch flush
	time.Sleep(200 * time.Millisecond)

	// Verify the batch was emptied
	assert.Zero(t, BatchSize())
}

func TestBatchObserveWithDetails(t *testing.T) {
	// Reset test state
	resetTestState()

	// Initialize OTEL with batching
	err := InitOTELExporter("localhost:4317", true, 500*time.Millisecond)
	require.NoError(t, err)

	// Add observations with different details
	BatchObserveWithDetails("Async", 50*time.Millisecond, "core", "404")
	BatchObserveWithDetails("Query", 75*time.Millisecond, "edge", "500")

	// Verify both observations were added to the batch
	assert.Equal(t, 2, BatchSize())

	// Manually force a flush for the test
	flushBatch()

	// Verify the batch was emptied
	assert.Zero(t, BatchSize())
}
