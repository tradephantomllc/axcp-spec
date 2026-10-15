// Package metrics provides configuration for metrics collection
package metrics

import (
	"context"
	"flag"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Config holds the configuration for metrics collection
type Config struct {
	// Prometheus configuration
	Prometheus struct {
		Enabled    bool
		ListenAddr string
	}

	// OpenTelemetry configuration
	OTEL struct {
		Enabled       bool
		Collector     string
		ServiceName   string
		Timeout       time.Duration
		BatchInterval time.Duration
	}
}

// DefaultConfig returns the default metrics configuration
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Prometheus.Enabled = true
	// Bind to loopback by default: the endpoint has no authentication.
	cfg.Prometheus.ListenAddr = "127.0.0.1:9090"
	cfg.OTEL.Enabled = false
	cfg.OTEL.Collector = "localhost:4317"
	cfg.OTEL.ServiceName = "axcp-gateway"
	cfg.OTEL.Timeout = 10 * time.Second
	cfg.OTEL.BatchInterval = 10 * time.Second
	return cfg
}

// AddFlags adds the metrics flags to the flag set
func (c *Config) AddFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.Prometheus.Enabled, "enable-prom", c.Prometheus.Enabled, "Enable Prometheus metrics")
	fs.StringVar(&c.Prometheus.ListenAddr, "prom-addr", c.Prometheus.ListenAddr, "Prometheus metrics listen address")
	fs.BoolVar(&c.OTEL.Enabled, "enable-otel", c.OTEL.Enabled, "Enable OpenTelemetry metrics")
	fs.StringVar(&c.OTEL.Collector, "otel-collector", c.OTEL.Collector, "OpenTelemetry collector address")
	fs.DurationVar(&c.OTEL.BatchInterval, "otel-batch-interval", c.OTEL.BatchInterval, "OpenTelemetry batch interval")
}

// Setup initializes the metrics based on the configuration
func (c *Config) Setup(ctx context.Context) (Metrics, error) {
	// If no metrics are enabled, return a no-op implementation
	if !c.Prometheus.Enabled && !c.OTEL.Enabled {
		return &NopMetrics{}, nil
	}

	var metrics []Metrics

	// Setup Prometheus if enabled
	if c.Prometheus.Enabled {
		// Use a dedicated registry so repeated setups (and tests) do not conflict
		registry := prometheus.NewRegistry()

		// Start Prometheus with the dedicated registry
		if err := InitPrometheusWithRegistry(c.Prometheus.ListenAddr, true, registry); err != nil {
			return nil, err
		}

		// Create PrometheusMetrics on the dedicated registry
		promMetrics := NewPrometheusMetrics(registry)
		metrics = append(metrics, promMetrics)
	}

	// Setup OpenTelemetry if enabled
	if c.OTEL.Enabled {
		// Initialize the OTEL exporter with batching
		if err := InitOTELExporter(c.OTEL.Collector, true, c.OTEL.BatchInterval); err != nil {
			return nil, err
		}

		otelMetrics, err := NewOpenTelemetryMetrics(ctx, OpenTelemetryConfig{
			CollectorEndpoint: c.OTEL.Collector,
			ServiceName:       c.OTEL.ServiceName,
			Timeout:           c.OTEL.Timeout,
		})
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, otelMetrics)
	}

	// If only one metrics provider is enabled, return it directly
	if len(metrics) == 1 {
		return metrics[0], nil
	}

	// If both are enabled, return a multi-metrics implementation
	return NewMultiMetrics(metrics...), nil
}
