package metrics

import "context"

// Metrics defines the interface for all metrics operations
type Metrics interface {
	// RecordEnvelopeIn records an incoming envelope
	RecordEnvelopeIn()
	// RecordEnvelopeOut records an outgoing envelope
	RecordEnvelopeOut()
	// SetRetryQueueSize updates the retry queue size
	SetRetryQueueSize(size int)
	// RecordRetryAttempt records a retry attempt
	RecordRetryAttempt()
	// RecordRetrySuccess records a successful retry
	RecordRetrySuccess()
	// RecordRetryDropped records a dropped retry
	RecordRetryDropped()
	// Shutdown gracefully shuts down the metrics client
	Shutdown(ctx context.Context) error
}

// NopMetrics is a no-op implementation of the Metrics interface
type NopMetrics struct{}

func (n NopMetrics) RecordEnvelopeIn()                {}
func (n NopMetrics) RecordEnvelopeOut()               {}
func (n NopMetrics) SetRetryQueueSize(int)            {}
func (n NopMetrics) RecordRetryAttempt()              {}
func (n NopMetrics) RecordRetrySuccess()              {}
func (n NopMetrics) RecordRetryDropped()              {}
func (n NopMetrics) Shutdown(ctx context.Context) error { return nil }
