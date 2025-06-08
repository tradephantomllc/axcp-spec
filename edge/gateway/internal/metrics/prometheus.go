// Package metrics provides Prometheus metrics for the AXCP Gateway
package metrics

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements the Metrics interface using Prometheus
type PrometheusMetrics struct {
	server        *http.Server
	envelopesIn   prometheus.Counter
	envelopesOut  prometheus.Counter
	retryQueueSize prometheus.Gauge
	retryAttempts prometheus.Counter
	retrySuccess  prometheus.Counter
	retryDropped  prometheus.Counter
}

var (
	once     sync.Once
	registry = prometheus.NewRegistry()
)

// NewPrometheusMetrics creates a new PrometheusMetrics instance
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		envelopesIn: promauto.NewCounter(prometheus.CounterOpts{
			Name: "axcp_envelopes_in_total",
			Help: "Total number of envelopes received by the gateway",
		}),
		envelopesOut: promauto.NewCounter(prometheus.CounterOpts{
			Name: "axcp_envelopes_out_total",
			Help: "Total number of envelopes sent by the gateway",
		}),
		retryQueueSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "axcp_retry_queue_size",
			Help: "Current number of messages in the retry queue",
		}),
		retryAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "axcp_retry_attempts_total",
			Help: "Total number of retry attempts",
		}),
		retrySuccess: promauto.NewCounter(prometheus.CounterOpts{
			Name: "axcp_retry_success_total",
			Help: "Total number of successful retries",
		}),
		retryDropped: promauto.NewCounter(prometheus.CounterOpts{
			Name: "axcp_retry_dropped_total",
			Help: "Total number of dropped retries",
		}),
	}
}

// RecordEnvelopeIn increments the incoming envelope counter
func (p *PrometheusMetrics) RecordEnvelopeIn() {
	p.envelopesIn.Inc()
}

// RecordEnvelopeOut increments the outgoing envelope counter
func (p *PrometheusMetrics) RecordEnvelopeOut() {
	p.envelopesOut.Inc()
}

// SetRetryQueueSize updates the retry queue size gauge
func (p *PrometheusMetrics) SetRetryQueueSize(size int) {
	p.retryQueueSize.Set(float64(size))
}

// RecordRetryAttempt increments the retry attempts counter
func (p *PrometheusMetrics) RecordRetryAttempt() {
	p.retryAttempts.Inc()
}

// RecordRetrySuccess increments the successful retries counter
func (p *PrometheusMetrics) RecordRetrySuccess() {
	p.retrySuccess.Inc()
}

// RecordRetryDropped increments the dropped retries counter
func (p *PrometheusMetrics) RecordRetryDropped() {
	p.retryDropped.Inc()
}

// Shutdown gracefully shuts down the metrics server
func (p *PrometheusMetrics) Shutdown(ctx context.Context) error {
	if p.server != nil {
		return p.server.Shutdown(ctx)
	}
	return nil
}

// StartServer starts the Prometheus metrics server on the specified address
func (p *PrometheusMetrics) StartServer(addr string) error {
	once.Do(func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			},
		))

		server := &http.Server{
			Addr:    addr,
			Handler: mux,
		}
		p.server = server

		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Error starting metrics server: %v", err)
			}
		}()
	})
	return nil
}
