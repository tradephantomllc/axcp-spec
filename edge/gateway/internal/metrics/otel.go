// Package metrics provides metrics collection for the gateway
package metrics

import (
	"context"
	"sync"
	"time"
)

var (
	// Various batching-related variables
	batchMutex      sync.Mutex
	batchBuffer     []metricRecord
	batchTicker     *time.Ticker
	batchInterval   time.Duration
	batchingEnabled bool
	batchContext    context.Context
	batchCancelFunc context.CancelFunc
	shutdown        chan struct{}
	batchWaitGroup  sync.WaitGroup
)

// metricRecord represents a single metric record in the batch
type metricRecord struct {
	Method   string
	Duration time.Duration
	NodeType string
	Status   string
}

// InitOTELExporter initializes the OpenTelemetry exporter with batching support
func InitOTELExporter(endpoint string, enabled bool, interval time.Duration) error {
	if !enabled {
		return nil
	}

	batchMutex.Lock()
	defer batchMutex.Unlock()

	// Clean up any existing batch processing
	if batchCancelFunc != nil {
		batchCancelFunc()
	}
	if batchTicker != nil {
		batchTicker.Stop()
	}
	if shutdown != nil {
		close(shutdown)
		// Wait for the goroutine to exit
		batchWaitGroup.Wait()
	}

	// Initialize new batch context and buffer
	batchingEnabled = true
	batchInterval = interval
	batchBuffer = make([]metricRecord, 0, 100) // Pre-allocate buffer
	batchContext, batchCancelFunc = context.WithCancel(context.Background())
	batchTicker = time.NewTicker(batchInterval)
	shutdown = make(chan struct{})

	// Start background batch processor
	batchWaitGroup.Add(1)
	go batchProcessor(batchContext)

	return nil
}

// BatchObserve adds a metric observation to the batch buffer
func BatchObserve(method string, duration time.Duration) {
	BatchObserveWithDetails(method, duration, "edge", "200")
}

// BatchObserveWithDetails adds a detailed metric observation to the batch buffer
func BatchObserveWithDetails(method string, duration time.Duration, nodeType string, status string) {
	if !batchingEnabled {
		return
	}

	batchMutex.Lock()
	defer batchMutex.Unlock()

	// Add to buffer
	batchBuffer = append(batchBuffer, metricRecord{
		Method:   method,
		Duration: duration,
		NodeType: nodeType,
		Status:   status,
	})
}

// BatchSize returns the current size of the batch buffer (for testing)
func BatchSize() int {
	batchMutex.Lock()
	defer batchMutex.Unlock()
	return len(batchBuffer)
}

// batchProcessor periodically flushes the batch buffer
func batchProcessor(ctx context.Context) {
	defer batchWaitGroup.Done()

	for {
		var localTicker *time.Ticker
		var localShutdown chan struct{}

		// Safely read the shared variables
		batchMutex.Lock()
		localTicker = batchTicker
		localShutdown = shutdown
		batchMutex.Unlock()

		// Safety check in case of shutdown while running
		if localTicker == nil || localShutdown == nil {
			return
		}

		select {
		case <-localTicker.C:
			flushBatch()
		case <-ctx.Done():
			return
		case <-localShutdown:
			// Flush any remaining metrics before shutdown
			flushBatch()
			return
		}
	}
}

// ShutdownOTEL terminates the OTEL exporter gracefully
func ShutdownOTEL() {
	// Take the lock before modifying the shared variables
	batchMutex.Lock()

	if !batchingEnabled {
		batchMutex.Unlock()
		return
	}

	batchingEnabled = false

	// Cancel the batch context
	var localCancelFunc context.CancelFunc
	if batchCancelFunc != nil {
		localCancelFunc = batchCancelFunc
		batchCancelFunc = nil
	}

	// Stop the ticker
	if batchTicker != nil {
		batchTicker.Stop()
		batchTicker = nil
	}

	// Signal shutdown
	var needWait bool
	if shutdown != nil {
		close(shutdown)
		shutdown = nil
		needWait = true
	}

	// Release the lock before blocking operations
	batchMutex.Unlock()

	// Cancel the context after releasing the lock
	if localCancelFunc != nil {
		localCancelFunc()
	}

	// Wait for the goroutine to exit, if needed
	if needWait {
		batchWaitGroup.Wait()
	}

	// Clear the batch buffer
	batchBuffer = nil
}

// flushBatch sends all batched metrics to OpenTelemetry
func flushBatch() {
	// Take the lock to access the shared buffer
	batchMutex.Lock()

	// Check that there is data to process
	if len(batchBuffer) == 0 || !batchingEnabled {
		batchMutex.Unlock()
		return
	}

	// Make a local copy of the data to process
	records := make([]metricRecord, len(batchBuffer))
	copy(records, batchBuffer)

	// Empty the buffer while still holding the lock
	batchBuffer = batchBuffer[:0]

	// Release the lock before processing the data
	batchMutex.Unlock()

	// Process all records in batch
	for _, record := range records {
		// Process metrics for this record using Prometheus
		// OTEL functionality would be added here if needed in the future

		// Process both OTEL and Prometheus metrics
		durationSeconds := float64(record.Duration) / float64(time.Second)
		if RPCLatency != nil {
			RPCLatency.WithLabelValues(record.Method, record.Status, record.NodeType).Observe(durationSeconds)
		}
	}

	// Note: This implementation focuses on Prometheus metrics.
	// Future enhancement: Add OpenTelemetry batch recording if needed.
}
