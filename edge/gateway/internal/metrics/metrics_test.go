package metrics

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetTestState è una funzione helper per ripulire lo stato tra i test
func resetTestState() {
	// Usa ShutdownOTEL per terminare in modo pulito il goroutine del batch processor
	ShutdownOTEL()
	
	// Cleanup extra per test precedenti
	batchMutex.Lock()
	defer batchMutex.Unlock()
	
	// Assicurati che lo stato sia completamente pulito
	batchingEnabled = false
	batchBuffer = nil
}

func TestHistogramObserve(t *testing.T) {
	// Reset test state
	resetTestState()

	// Trova una porta libera per il test
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() // Chiudi l'ascoltatore ma mantieni la porta per il test

	// Inizializza Prometheus con l'istogramma sulla porta casuale
	endpoint := fmt.Sprintf(":%d", port)
	err = InitPrometheus(endpoint, true)
	require.NoError(t, err)
	
	// Verifica che l'istogramma sia stato creato
	require.NotNil(t, RPCLatency)
	
	// Osserva un valore di latenza
	RPCLatency.WithLabelValues("Sync", "200", "edge").Observe(0.12)
	
	// Aspetta un po' per assicurarci che il server sia avviato
	time.Sleep(100 * time.Millisecond)
	
	// Esegui una richiesta HTTP per ottenere le metriche
	url := fmt.Sprintf("http://localhost:%d/metrics", port)
	res, err := http.Get(url)
	require.NoError(t, err)
	defer res.Body.Close()
	
	// Leggi il corpo della risposta
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	
	// Verifica che l'istogramma sia presente nelle metriche
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "rpc_duration_seconds_bucket")
	assert.Contains(t, bodyStr, "method=\"Sync\"")
	assert.Contains(t, bodyStr, "status_code=\"200\"")
	assert.Contains(t, bodyStr, "node_type=\"edge\"")
}

func TestOTELBatching(t *testing.T) {
	// Reset test state
	resetTestState()
	
	// Inizializza OTEL con batching
	err := InitOTELExporter("localhost:4317", true, 100*time.Millisecond)
	require.NoError(t, err)
	
	// Verifica che il batching sia abilitato
	assert.True(t, batchingEnabled)
	
	// Aggiungi un'osservazione al batch
	BatchObserve("Sync", 150*time.Millisecond)
	
	// Verifica che l'osservazione sia stata aggiunta al batch
	assert.Equal(t, 1, BatchSize())
	
	// Attendi il flush del batch
	time.Sleep(200 * time.Millisecond)
	
	// Verifica che il batch sia stato svuotato
	assert.Zero(t, BatchSize())
}

func TestBatchObserveWithDetails(t *testing.T) {
	// Reset test state
	resetTestState()
	
	// Inizializza OTEL con batching
	err := InitOTELExporter("localhost:4317", true, 500*time.Millisecond)
	require.NoError(t, err)
	
	// Aggiungi osservazioni con dettagli diversi
	BatchObserveWithDetails("Async", 50*time.Millisecond, "core", "404")
	BatchObserveWithDetails("Query", 75*time.Millisecond, "edge", "500")
	
	// Verifica che entrambe le osservazioni siano state aggiunte al batch
	assert.Equal(t, 2, BatchSize())
	
	// Forza manualmente il flush per il test
	flushBatch()
	
	// Verifica che il batch sia stato svuotato
	assert.Zero(t, BatchSize())
}
