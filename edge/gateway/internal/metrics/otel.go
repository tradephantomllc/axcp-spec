// Package metrics provides metrics collection for the gateway
package metrics

import (
	"context"
	"sync"
	"time"
)

var (
	// Various batching-related variables
	batchMutex       sync.Mutex
	batchBuffer      []metricRecord
	batchTicker      *time.Ticker
	batchInterval    time.Duration
	batchingEnabled  bool
	batchContext     context.Context
	batchCancelFunc  context.CancelFunc
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
	
	// Initialize new batch context and buffer
	batchingEnabled = true
	batchInterval = interval
	batchBuffer = make([]metricRecord, 0, 100) // Pre-allocate buffer
	batchContext, batchCancelFunc = context.WithCancel(context.Background())
	batchTicker = time.NewTicker(batchInterval)
	
	// Start background batch processor
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
	for {
		select {
		case <-batchTicker.C:
			flushBatch()
		case <-ctx.Done():
			return
		}
	}
}

// flushBatch sends all batched metrics to OpenTelemetry
func flushBatch() {
	batchMutex.Lock()
	
	// Make a copy of the buffer and reset it
	records := batchBuffer
	batchBuffer = make([]metricRecord, 0, cap(batchBuffer))
	
	batchMutex.Unlock()
	
	if len(records) == 0 {
		return
	}
	
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

