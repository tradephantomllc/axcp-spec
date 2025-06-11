package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// RetryQueue tracks the current number of items in the retry queue
	RetryQueue = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_retry_queue_size",
		Help: "Current number of items in the retry queue",
	})

	// RetryDropped counts the number of messages dropped due to retry failures
	RetryDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gateway_retry_messages_dropped_total",
		Help: "Total number of messages dropped after retry attempts",
	})

	// RetryAttempts counts the number of retry attempts
	RetryAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gateway_retry_attempts_total",
		Help: "Total number of retry attempts",
	})

	// RetrySuccess counts the number of successful retries
	RetrySuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gateway_retry_success_total",
		Help: "Total number of successful retries",
	})
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(RetryQueue)
	prometheus.MustRegister(RetryDropped)
	prometheus.MustRegister(RetryAttempts)
	prometheus.MustRegister(RetrySuccess)
}
