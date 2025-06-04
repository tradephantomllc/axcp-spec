package buffer

import (
	"fmt"
	"math"
	"time"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal/metrics"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
	"google.golang.org/protobuf/proto"
)

// Retry configuration
const (
	initialBackoff = 250 * time.Millisecond
	maxBackoff    = 4 * time.Second
)

// Broker defines the interface required for publishing telemetry data
type Broker interface {
	PublishTelemetry(td *pb.TelemetryDatagram, traceID string) error
}

// StartRetryLoop starts a goroutine that processes items from the queue with exponential backoff
func StartRetryLoop(q *Queue, b Broker, stop <-chan struct{}) {
	backoff := initialBackoff
	ticker := time.NewTicker(initialBackoff)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Try to process items from the queue
			success, err := processBatch(q, b, 10) // Process up to 10 items per batch
			if err != nil {
				// Log error and continue with next tick
				continue
			}

			// Adjust backoff based on success/failure
			if success {
				// Success, reset backoff to minimum
				backoff = initialBackoff
			} else {
				// Failure, increase backoff exponentially
				backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			}

			// Reset ticker with new backoff
			ticker.Reset(backoff)
		}
	}
}

// processBatch processes a batch of items from the queue
// Returns true if all items were processed successfully
func processBatch(q *Queue, b Broker, batchSize int) (bool, error) {
	items, err := q.Pop(batchSize)
	if err != nil {
		return false, err
	}

	if len(items) == 0 {
		// No items to process
		return true, nil
	}

	// Track if all items were processed successfully
	allSuccessful := true

	for _, item := range items {
		// Unmarshal the telemetry data
		var td pb.TelemetryDatagram
		if err := proto.Unmarshal(item, &td); err != nil {
			// If we can't unmarshal, skip this item
			metrics.RetryDropped.Inc()
			continue
		}

		// Try to publish the item
		err = b.PublishTelemetry(&td, fmt.Sprintf("retry-%d", time.Now().UnixNano()))
		if err != nil {
			// If publish fails, put the item back in the queue
			allSuccessful = false
			// Use a new key to avoid overwriting other items
			if err := q.Push([]byte(time.Now().String()), item); err != nil {
				// If we can't requeue, we'll lose this message
				metrics.RetryDropped.Inc()
			}
		} else {
			// Update success metrics
			metrics.RetrySuccess.Inc()
		}

		// Update metrics
		metrics.RetryAttempts.Inc()
		if count, err := q.Len(); err == nil {
			metrics.RetryQueue.Set(float64(count))
		}
	}

	return allSuccessful, nil
}
