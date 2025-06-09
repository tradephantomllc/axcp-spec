package internal

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
)

// RetryMetrics contiene tutti i contatori e le metriche per il retry buffer
type RetryMetrics struct {
	retryCounter    metric.Int64Counter
	dropCounter     metric.Int64Counter
	successCounter  metric.Int64Counter
	queueSizeGauge  metric.Int64UpDownCounter
	queueSizeValue  atomic.Int64
	retryDelayGauge metric.Float64Histogram
}

// NewRetryMetrics crea un nuovo gestore di metriche per il retry buffer
func NewRetryMetrics(meter metric.Meter) (*RetryMetrics, error) {
	retryCounter, err := meter.Int64Counter("axcp.retry.attempts",
		metric.WithDescription("Numero di tentativi di retry"))
	if err != nil {
		return nil, err
	}

	dropCounter, err := meter.Int64Counter("axcp.retry.dropped",
		metric.WithDescription("Numero di messaggi persi (non aggiunti al buffer di retry)"))
	if err != nil {
		return nil, err
	}

	successCounter, err := meter.Int64Counter("axcp.retry.success",
		metric.WithDescription("Numero di tentativi di retry riusciti"))
	if err != nil {
		return nil, err
	}

	queueSizeGauge, err := meter.Int64UpDownCounter("axcp.retry.queue_size",
		metric.WithDescription("Dimensione attuale della coda di retry"))
	if err != nil {
		return nil, err
	}

	retryDelayGauge, err := meter.Float64Histogram("axcp.retry.delay_seconds",
		metric.WithDescription("Tempo di attesa prima del retry in secondi"))
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

// RecordRetryAttempt incrementa il contatore dei tentativi di retry
func (r *RetryMetrics) RecordRetryAttempt(ctx context.Context) {
	r.retryCounter.Add(ctx, 1)
}

// RecordRetryDropped incrementa il contatore dei messaggi persi
func (r *RetryMetrics) RecordRetryDropped(ctx context.Context) {
	r.dropCounter.Add(ctx, 1)
}

// RecordRetrySuccess incrementa il contatore dei tentativi di retry riusciti
func (r *RetryMetrics) RecordRetrySuccess(ctx context.Context) {
	r.successCounter.Add(ctx, 1)
}

// RecordRetryDelay registra il tempo di attesa prima del retry
func (r *RetryMetrics) RecordRetryDelay(ctx context.Context, seconds float64) {
	// Non usiamo attributi per ora, poiché attribute.KeyValue non implementa metric.RecordOption
	r.retryDelayGauge.Record(ctx, seconds)
}

// SetRetryQueueSize imposta la dimensione attuale della coda di retry
func (r *RetryMetrics) SetRetryQueueSize(ctx context.Context, size int64) {
	// Calculate delta to update the gauge
	oldSize := r.queueSizeValue.Swap(size)
	delta := size - oldSize
	
	if delta != 0 {
		r.queueSizeGauge.Add(ctx, delta)
	}
}
