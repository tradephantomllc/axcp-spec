// Package metrics provides Prometheus metrics for the AXCP Gateway
package metrics

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
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
	rpcLatency    *prometheus.HistogramVec
	registry      prometheus.Registerer
}

var (
	once     sync.Once
	registry = prometheus.NewRegistry() // Default registry
	
	// RPCLatency è un istogramma pubblico per la latenza RPC
	RPCLatency *prometheus.HistogramVec
	
	// Mutex per sincronizzare l'accesso alle metriche globali
	metricsMutex sync.Mutex
	
	// Server HTTP per le metriche Prometheus
	promoServer *http.Server
)

// InitPrometheus inizializza le metriche Prometheus con il registry globale
func InitPrometheus(addr string, enabled bool) error {
	if !enabled {
		return nil
	}
	
	// Dato che prometheus.DefaultRegisterer non è un *prometheus.Registry,
	// possiamo usare prometheus.DefaultGatherer per servire le metriche
	
	// Inizializza le metriche con il registry globale se non già fatto
	if RPCLatency == nil {
		RPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "rpc_duration_seconds",
			Help:    "RPC latency distributions in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "status_code", "node_type"})
		
		// Registra nel registry globale
		prometheus.MustRegister(RPCLatency)
	}
	
	// Avvia il server Prometheus con il registry globale
	return ServeWithRegistry(addr, prometheus.DefaultGatherer)
}

// InitPrometheusWithRegistry inizializza le metriche Prometheus con un registry personalizzato
// Il parametro reg deve essere un *prometheus.Registry che implementa sia Registerer che Gatherer
func InitPrometheusWithRegistry(addr string, enabled bool, reg *prometheus.Registry) error {
	if !enabled {
		return nil
	}
	
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	
	// Chiudi eventuali server esistenti
	if promoServer != nil {
		promoServer.Close()
		promoServer = nil
	}
	
	// Inizializza le metriche con il registry fornito
	RPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rpc_duration_seconds",
		Help:    "RPC latency distributions in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "status_code", "node_type"})
	
	// Registra solo se stiamo usando il registry globale
	// altrimenti lasciamo che sia il chiamante a gestire la registrazione
	if reg == prometheus.DefaultRegisterer {
		prometheus.MustRegister(RPCLatency)
	} else {
		reg.MustRegister(RPCLatency)
	}
	
	// Avvia il server Prometheus con il registry specificato
	return ServeWithRegistry(addr, reg)
}

// Serve avvia il server HTTP per le metriche Prometheus con il registry globale
func Serve(addr string) error {
	// Usa il registry globale
	return ServeWithRegistry(addr, registry)
}

// ServeWithRegistry avvia il server HTTP per le metriche Prometheus con un registry personalizzato
func ServeWithRegistry(addr string, reg prometheus.Gatherer) error {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	
	// Chiudi eventuali server esistenti
	if promoServer != nil {
		promoServer.Close()
		promoServer = nil
	}
	
	// Avvia il server HTTP per le metriche con il registry specificato
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	
	promoServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	go func() {
		if err := promoServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	}()
	
	return nil
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance with optional registry
func NewPrometheusMetrics(reg ...prometheus.Registerer) *PrometheusMetrics {
	// Usa il registry fornito o quello di default
	var registry prometheus.Registerer
	if len(reg) > 0 && reg[0] != nil {
		registry = reg[0]
	} else {
		// Utilizziamo il registry globale di Prometheus
		registry = prometheus.DefaultRegisterer
	}
	
	// Inizializza l'istanza di PrometheusMetrics
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
	
	// Registra tutte le metriche al registry specificato
	registry.MustRegister(pm.envelopesIn)
	registry.MustRegister(pm.envelopesOut)
	registry.MustRegister(pm.retryQueueSize)
	registry.MustRegister(pm.retryAttempts)
	registry.MustRegister(pm.retrySuccess)
	registry.MustRegister(pm.retryDropped)
	if pm.rpcLatency != RPCLatency && pm.rpcLatency != nil { // Evita la doppia registrazione se è lo stesso istanza
		registry.MustRegister(pm.rpcLatency)
	}

	// Importante: restituisci l'istanza per consentire l'implementazione dell'interfaccia Metrics
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
	
	// Usa il registry del pm invece di quello globale
	mux.Handle("/metrics", promhttp.HandlerFor(
		pm.registry.(prometheus.Gatherer),
		promhttp.HandlerOpts{},
	))
	
	// Create and start the server
	pm.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	go func() {
		if err := pm.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	}()
	
	return nil
}
