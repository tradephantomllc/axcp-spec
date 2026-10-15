// Package metrics provides Prometheus metrics for the AXCP Gateway
package metrics

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements the Metrics interface using Prometheus
type PrometheusMetrics struct {
	server         *http.Server
	envelopesIn    prometheus.Counter
	envelopesOut   prometheus.Counter
	retryQueueSize prometheus.Gauge
	retryAttempts  prometheus.Counter
	retrySuccess   prometheus.Counter
	retryDropped   prometheus.Counter
	rpcLatency     *prometheus.HistogramVec
	registry       prometheus.Registerer
}

var (
	registry = prometheus.NewRegistry() // Default registry

	// RPCLatency is a public histogram for RPC latency
	RPCLatency *prometheus.HistogramVec

	// Mutex that synchronizes access to the global metrics
	metricsMutex sync.Mutex

	// HTTP server for Prometheus metrics
	promoServer *http.Server
)

// InitPrometheus initializes Prometheus metrics with the global registry
func InitPrometheus(addr string, enabled bool) error {
	if !enabled {
		return nil
	}

	// Since prometheus.DefaultRegisterer is not a *prometheus.Registry,
	// use prometheus.DefaultGatherer to serve the metrics

	// Initialize the metrics with the global registry if not already done
	if RPCLatency == nil {
		RPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "rpc_duration_seconds",
			Help:    "RPC latency distributions in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "status_code", "node_type"})

		// Register with the global registry
		prometheus.MustRegister(RPCLatency)
	}

	// Start the Prometheus server with the global registry
	return ServeWithRegistry(addr, prometheus.DefaultGatherer)
}

// InitPrometheusWithRegistry initializes Prometheus metrics with a custom registry.
// The reg parameter must be a *prometheus.Registry, which implements both Registerer and Gatherer.
func InitPrometheusWithRegistry(addr string, enabled bool, reg *prometheus.Registry) error {
	if !enabled {
		return nil
	}

	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	// Close any existing server
	if promoServer != nil {
		promoServer.Close()
		promoServer = nil
	}

	// Initialize the metrics with the given registry
	RPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rpc_duration_seconds",
		Help:    "RPC latency distributions in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "status_code", "node_type"})

	// Register globally only when using the global registry;
	// otherwise register with the caller's registry
	if reg == prometheus.DefaultRegisterer {
		prometheus.MustRegister(RPCLatency)
	} else {
		reg.MustRegister(RPCLatency)
	}

	// Start the Prometheus server with the given registry
	return ServeWithRegistry(addr, reg)
}

// Serve starts the Prometheus metrics HTTP server with the global registry
func Serve(addr string) error {
	// Use the global registry
	return ServeWithRegistry(addr, registry)
}

// ServeWithRegistry starts the Prometheus metrics HTTP server with a custom registry
func ServeWithRegistry(addr string, reg prometheus.Gatherer) error {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	// Close any existing server
	if promoServer != nil {
		promoServer.Close()
		promoServer = nil
	}

	// Start the metrics HTTP server with the given registry
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	promoServer = newMetricsServer(addr, mux)

	go func() {
		if err := promoServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	}()

	return nil
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance with optional registry
func NewPrometheusMetrics(reg ...prometheus.Registerer) *PrometheusMetrics {
	// Use the given registry or the default one
	var registry prometheus.Registerer
	if len(reg) > 0 && reg[0] != nil {
		registry = reg[0]
	} else {
		// Use the global Prometheus registry
		registry = prometheus.DefaultRegisterer
	}

	// Initialize the PrometheusMetrics instance
	pm := &PrometheusMetrics{
		registry: registry,
		envelopesIn: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "axcp_envelopes_in_total",
			Help: "Total number of envelopes received by the gateway",
		}),
		envelopesOut: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "axcp_envelopes_out_total",
			Help: "Total number of envelopes sent by the gateway",
		}),
		retryQueueSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "axcp_retry_queue_size",
			Help: "Current number of messages in the retry queue",
		}),
		retryAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "axcp_retry_attempts_total",
			Help: "Total number of retry attempts",
		}),
		retrySuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "axcp_retry_success_total",
			Help: "Total number of successful retries",
		}),
		retryDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "axcp_retry_dropped_total",
			Help: "Total number of dropped retries",
		}),
		rpcLatency: RPCLatency,
	}

	// Register all metrics with the given registry
	registry.MustRegister(pm.envelopesIn)
	registry.MustRegister(pm.envelopesOut)
	registry.MustRegister(pm.retryQueueSize)
	registry.MustRegister(pm.retryAttempts)
	registry.MustRegister(pm.retrySuccess)
	registry.MustRegister(pm.retryDropped)
	if pm.rpcLatency != RPCLatency && pm.rpcLatency != nil { // Avoid double registration of the same instance
		registry.MustRegister(pm.rpcLatency)
	}

	// Return the instance, which implements the Metrics interface
	return pm
}

// RecordEnvelopeIn increments the incoming envelope counter
func (pm *PrometheusMetrics) RecordEnvelopeIn() {
	pm.envelopesIn.Inc()
}

// RecordEnvelopeOut increments the outgoing envelope counter
func (pm *PrometheusMetrics) RecordEnvelopeOut() {
	pm.envelopesOut.Inc()
}

// SetRetryQueueSize updates the retry queue size gauge
func (pm *PrometheusMetrics) SetRetryQueueSize(size int) {
	pm.retryQueueSize.Set(float64(size))
}

// RecordRetryAttempt increments the retry attempts counter
func (pm *PrometheusMetrics) RecordRetryAttempt() {
	pm.retryAttempts.Inc()
}

// RecordRetrySuccess increments the successful retries counter
func (pm *PrometheusMetrics) RecordRetrySuccess() {
	pm.retrySuccess.Inc()
}

// RecordRetryDropped increments the dropped retries counter
func (pm *PrometheusMetrics) RecordRetryDropped() {
	pm.retryDropped.Inc()
}

// Shutdown gracefully shuts down the metrics server
func (pm *PrometheusMetrics) Shutdown(ctx context.Context) error {
	if pm.server != nil {
		return pm.server.Shutdown(ctx)
	}
	return nil
}

// StartServer starts the Prometheus metrics server on the specified address
func (pm *PrometheusMetrics) StartServer(addr string) error {
	// Create a new mux for the metrics server
	mux := http.NewServeMux()

	// Use pm's registry instead of the global one
	mux.Handle("/metrics", promhttp.HandlerFor(
		pm.registry.(prometheus.Gatherer),
		promhttp.HandlerOpts{},
	))

	// Create and start the server
	pm.server = newMetricsServer(addr, mux)

	go func() {
		if err := pm.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	}()

	return nil
}

// newMetricsServer returns an HTTP server with timeouts that bound slow clients.
func newMetricsServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}
