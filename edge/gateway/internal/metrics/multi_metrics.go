package metrics

import "context"

// MultiMetrics implements the Metrics interface by delegating to multiple Metrics implementations
type MultiMetrics struct {
	providers []Metrics
}

// NewMultiMetrics creates a new MultiMetrics that delegates to the given providers
func NewMultiMetrics(providers ...Metrics) *MultiMetrics {
	return &MultiMetrics{
		providers: providers,
	}
}

// RecordEnvelopeIn records an incoming envelope across all providers
func (m *MultiMetrics) RecordEnvelopeIn() {
	for _, p := range m.providers {
		p.RecordEnvelopeIn()
	}
}

// RecordEnvelopeOut records an outgoing envelope across all providers
func (m *MultiMetrics) RecordEnvelopeOut() {
	for _, p := range m.providers {
		p.RecordEnvelopeOut()
	}
}

// SetRetryQueueSize updates the retry queue size across all providers
func (m *MultiMetrics) SetRetryQueueSize(size int) {
	for _, p := range m.providers {
		p.SetRetryQueueSize(size)
	}
}

// RecordRetryAttempt records a retry attempt across all providers
func (m *MultiMetrics) RecordRetryAttempt() {
	for _, p := range m.providers {
		p.RecordRetryAttempt()
	}
}

// RecordRetrySuccess records a successful retry across all providers
func (m *MultiMetrics) RecordRetrySuccess() {
	for _, p := range m.providers {
		p.RecordRetrySuccess()
	}
}

// RecordRetryDropped records a dropped retry across all providers
func (m *MultiMetrics) RecordRetryDropped() {
	for _, p := range m.providers {
		p.RecordRetryDropped()
	}
}

// Shutdown shuts down all metrics providers
func (m *MultiMetrics) Shutdown(ctx context.Context) error {
	var lastErr error
	for _, p := range m.providers {
		if err := p.Shutdown(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
