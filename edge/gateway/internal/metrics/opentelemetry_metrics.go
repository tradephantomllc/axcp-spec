// Package metrics provides unified metrics collection for the gateway
package metrics

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// OpenTelemetryMetrics implements the Metrics interface using OpenTelemetry
type OpenTelemetryMetrics struct {
	meter           metric.Meter
	envelopesIn     metric.Int64Counter
	envelopesOut    metric.Int64Counter
	retryQueueSize  metric.Int64ObservableGauge
	retryAttempts   metric.Int64Counter
	retrySuccess    metric.Int64Counter
	retryDropped    metric.Int64Counter
	meterProvider   *sdkmetric.MeterProvider
	queueSize       int64 // Current queue size for the observable gauge
}

// OpenTelemetryConfig holds the configuration for OpenTelemetry metrics
type OpenTelemetryConfig struct {
	// CollectorEndpoint is the address of the OTLP collector
	CollectorEndpoint string
	// ServiceName is the name of the service for resource attributes
	ServiceName string
	// Timeout is the timeout for the OTLP exporter
	Timeout time.Duration
}

// NewOpenTelemetryMetrics creates a new OpenTelemetryMetrics instance
func NewOpenTelemetryMetrics(ctx context.Context, cfg OpenTelemetryConfig) (*OpenTelemetryMetrics, error) {
	exp, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(cfg.CollectorEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service name
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
	)

	// Create meter
	meter := meterProvider.Meter(
		"github.com/tradephantom/axcp-spec/gateway",
	)

	// Create counters and gauges
	envelopesIn, err := meter.Int64Counter(
		"envelopes_in_total",
		metric.WithDescription("Total number of incoming envelopes"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create envelopes_in_total counter: %w", err)
	}

	envelopesOut, err := meter.Int64Counter(
		"envelopes_out_total",
		metric.WithDescription("Total number of outgoing envelopes"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create envelopes_out_total counter: %w", err)
	}

	retryQueueSize, err := meter.Int64ObservableGauge(
		"retry_queue_size",
		metric.WithDescription("Current size of the retry queue"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry_queue_size gauge: %w", err)
	}

	retryAttempts, err := meter.Int64Counter(
		"retry_attempts_total",
		metric.WithDescription("Total number of retry attempts"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry_attempts_total counter: %w", err)
	}

	retrySuccess, err := meter.Int64Counter(
		"retry_success_total",
		metric.WithDescription("Total number of successful retries"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry_success_total counter: %w", err)
	}

	retryDropped, err := meter.Int64Counter(
		"retry_dropped_total",
		metric.WithDescription("Total number of dropped retries"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry_dropped_total counter: %w", err)
	}

	metrics := &OpenTelemetryMetrics{
		meter:          meter,
		envelopesIn:    envelopesIn,
		envelopesOut:   envelopesOut,
		retryQueueSize: retryQueueSize,
		retryAttempts:  retryAttempts,
		retrySuccess:   retrySuccess,
		retryDropped:   retryDropped,
		meterProvider:  meterProvider,
		queueSize:      0,
	}

	// Register the queue size callback
	if err := metrics.registerQueueSizeCallback(); err != nil {
		return nil, fmt.Errorf("failed to register queue size callback: %w", err)
	}

	return metrics, nil
}

// RecordEnvelopeIn increments the incoming envelope counter
func (o *OpenTelemetryMetrics) RecordEnvelopeIn() {
	o.envelopesIn.Add(context.Background(), 1)
}

// RecordEnvelopeOut increments the outgoing envelope counter
func (o *OpenTelemetryMetrics) RecordEnvelopeOut() {
	o.envelopesOut.Add(context.Background(), 1)
}

// SetRetryQueueSize updates the retry queue size gauge
func (o *OpenTelemetryMetrics) SetRetryQueueSize(size int) {
	// Update the current queue size
	atomic.StoreInt64(&o.queueSize, int64(size))
}

// registerQueueSizeCallback registers a callback to report the current queue size
func (o *OpenTelemetryMetrics) registerQueueSizeCallback() error {
	_, err := o.meter.RegisterCallback(
		func(ctx context.Context, obsrv metric.Observer) error {
			obsrv.ObserveInt64(o.retryQueueSize, atomic.LoadInt64(&o.queueSize))
			return nil
		},
		o.retryQueueSize,
	)
	return err
}

// RecordRetryAttempt increments the retry attempts counter
func (o *OpenTelemetryMetrics) RecordRetryAttempt() {
	o.retryAttempts.Add(context.Background(), 1)
}

// RecordRetrySuccess increments the successful retries counter
func (o *OpenTelemetryMetrics) RecordRetrySuccess() {
	o.retrySuccess.Add(context.Background(), 1)
}

// RecordRetryDropped increments the dropped retries counter
func (o *OpenTelemetryMetrics) RecordRetryDropped() {
	o.retryDropped.Add(context.Background(), 1)
}

// Shutdown gracefully shuts down the OpenTelemetry metrics provider
func (o *OpenTelemetryMetrics) Shutdown(ctx context.Context) error {
	return o.meterProvider.Shutdown(ctx)
}
