package internal

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
)

// RetryMetrics holds all counters and metrics for the retry buffer
type RetryMetrics struct {
	retryCounter    metric.Int64Counter
	dropCounter     metric.Int64Counter
	successCounter  metric.Int64Counter
	queueSizeGauge  metric.Int64UpDownCounter
	queueSizeValue  atomic.Int64
	retryDelayGauge metric.Float64Histogram
}

// NewRetryMetrics creates a new metrics recorder for the retry buffer
func NewRetryMetrics(meter metric.Meter) (*RetryMetrics, error) {
	retryCounter, err := meter.Int64Counter("axcp.retry.attempts",
		metric.WithDescription("Number of retry attempts"))
	if err != nil {
		return nil, err
	}

	dropCounter, err := meter.Int64Counter("axcp.retry.dropped",
		metric.WithDescription("Number of messages dropped by the retry buffer"))
	if err != nil {
		return nil, err
	}

	successCounter, err := meter.Int64Counter("axcp.retry.success",
		metric.WithDescription("Number of successful retry attempts"))
	if err != nil {
		return nil, err
	}

	queueSizeGauge, err := meter.Int64UpDownCounter("axcp.retry.queue_size",
		metric.WithDescription("Current size of the retry queue"))
	if err != nil {
		return nil, err
	}

	retryDelayGauge, err := meter.Float64Histogram("axcp.retry.delay_seconds",
		metric.WithDescription("Wait time before a retry, in seconds"))
	if err != nil {
		return nil, err
	}

	return &RetryMetrics{
		retryCounter:    retryCounter,
		dropCounter:     dropCounter,
		successCounter:  successCounter,
		queueSizeGauge:  queueSizeGauge,
		retryDelayGauge: retryDelayGauge,
	}, nil
}

// RecordRetryAttempt increments the retry attempt counter
func (r *RetryMetrics) RecordRetryAttempt(ctx context.Context) {
	r.retryCounter.Add(ctx, 1)
}

// RecordRetryDropped increments the dropped message counter
func (r *RetryMetrics) RecordRetryDropped(ctx context.Context) {
	r.dropCounter.Add(ctx, 1)
}

// RecordRetrySuccess increments the successful retry counter
func (r *RetryMetrics) RecordRetrySuccess(ctx context.Context) {
	r.successCounter.Add(ctx, 1)
}

// RecordRetryDelay records the wait time before a retry
func (r *RetryMetrics) RecordRetryDelay(ctx context.Context, seconds float64) {
	// No attributes for now, since attribute.KeyValue does not implement metric.RecordOption
	r.retryDelayGauge.Record(ctx, seconds)
}

// SetRetryQueueSize sets the current size of the retry queue
func (r *RetryMetrics) SetRetryQueueSize(ctx context.Context, size int64) {
	// Calculate delta to update the gauge
	oldSize := r.queueSizeValue.Swap(size)
	delta := size - oldSize

	if delta != 0 {
		r.queueSizeGauge.Add(ctx, delta)
	}
}
