// Package metrics provides configuration for metrics collection
package metrics

import (
	"context"
	"flag"
	"time"
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
		Enabled     bool
		Collector   string
		ServiceName string
		Timeout     time.Duration
		BatchInterval time.Duration
	}
}

// DefaultConfig returns the default metrics configuration
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Prometheus.Enabled = true
	cfg.Prometheus.ListenAddr = ":9090"
	cfg.OTEL.Enabled = true
	cfg.OTEL.Collector = "localhost:4317"
	cfg.OTEL.ServiceName = "axcp-gateway"
	cfg.OTEL.Timeout = 10 * time.Second
	cfg.OTEL.BatchInterval = 10 * time.Second
	return cfg
}

// AddFlags adds the metrics flags to the flag set
func (c *Config) AddFlags(fs *flag.FlagSet) {
	// Rinomino i flag come richiesto
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
	var cleanupFuncs []func()

	// Setup Prometheus if enabled
	if c.Prometheus.Enabled {
		// Inizializza Prometheus con il nuovo istogramma
		if err := InitPrometheus(c.Prometheus.ListenAddr, true); err != nil {
			return nil, err
		}
		promMetrics := NewPrometheusMetrics()
		metrics = append(metrics, promMetrics)
		cleanupFuncs = append(cleanupFuncs, func() {
			_ = promMetrics.Shutdown(ctx)
		})
	}

	// Setup OpenTelemetry if enabled
	if c.OTEL.Enabled {
		// Inizializza l'esportatore OTEL con supporto per il batching
		if err := InitOTELExporter(c.OTEL.Collector, true, c.OTEL.BatchInterval); err != nil {
			return nil, err
		}
		
		otelMetrics, err := NewOpenTelemetryMetrics(ctx, OpenTelemetryConfig{
			CollectorEndpoint: c.OTEL.Collector,
			ServiceName:      c.OTEL.ServiceName,
			Timeout:          c.OTEL.Timeout,
		})
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, otelMetrics)
		cleanupFuncs = append(cleanupFuncs, func() {
			_ = otelMetrics.Shutdown(ctx)
		})
	}

	// If only one metrics provider is enabled, return it directly
	if len(metrics) == 1 {
		return metrics[0], nil
	}

	// If both are enabled, return a multi-metrics implementation
	return NewMultiMetrics(metrics...), nil
}
