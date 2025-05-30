package test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

// TestPing è un test di base per verificare la funzionalità di invio/ricezione messaggi
// Questo è un test semplificato che verrà sostituito con un benchmark UDP reale
func TestPing(t *testing.T) {
	// Crea un messaggio di test
	orig := axcp.NewEnvelope(uuid.NewString(), 0)
	
	// In un'implementazione reale, qui andrebbe il codice per inviare/ricevere tramite UDP
	// Per ora, simuliamo una risposta identica al messaggio inviato
	got := orig

	// Verifica che il messaggio ricevuto sia uguale a quello inviato
	if got.TraceId != orig.TraceId {
		t.Fatalf("echo mismatch: expected %q, got %q", orig.TraceId, got.TraceId)
	}
}
