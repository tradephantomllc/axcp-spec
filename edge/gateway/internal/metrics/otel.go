// Package metrics provides metrics collection for the gateway
//
// NOTA: Questo file è DEPRECATO e sarà rimosso in futuro.
// Serve solo come compatibilità temporanea per l'IDE.
// Utilizzare opentelemetry_metrics.go per l'implementazione attuale.
package metrics

// OTELMetricsDeprecated è una struttura deprecata
// Utilizzare invece OpenTelemetryMetrics da opentelemetry_metrics.go
type OTELMetricsDeprecated struct{}

// OTELConfigDeprecated è una struttura deprecata
// Utilizzare invece OpenTelemetryConfig da opentelemetry_metrics.go
type OTELConfigDeprecated struct {
	Endpoint string
}
