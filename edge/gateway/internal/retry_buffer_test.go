package internal

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTestLogger è un semplice wrapper per log.Logger che implementa l'interfaccia minima richiesta
// dal RetryBuffer al posto di zap.Logger
type MockTestLogger struct {
	l *log.Logger
}

func NewMockTestLogger(t *testing.T) *MockTestLogger {
	return &MockTestLogger{
		l: log.New(ioutil.Discard, "", 0), // Logger silenzioso per i test
	}
}

// I metodi necessari per implementare l'interfaccia di logging minima
func (m *MockTestLogger) Debug(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Info(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Warn(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Error(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Fatal(msg string, fields ...interface{}) {}
func (m *MockTestLogger) Sync() error { return nil }

func TestRetryBufferBasicOperations(t *testing.T) {
	logger := NewMockTestLogger(t)
	
	// Mock per la funzione di pubblicazione
	publishCalls := 0
	publishMutex := sync.Mutex{}
	mockPublish := func(env *axcp.Envelope) error {
		publishMutex.Lock()
		publishCalls++
		publishMutex.Unlock()
		return nil // Simula sempre successo
	}
	
	// Configurazione con intervalli brevi per i test
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 10 * time.Millisecond
	config.MaxRetryInterval = 100 * time.Millisecond
	
	// Crea il buffer di retry
	rb := NewRetryBuffer(&config, logger, mockPublish)
	defer rb.Close()
	
	// Test aggiunta di un envelope
	env := axcp.NewEnvelope("test-1", 0)
	
	err := rb.AddEnvelope("test-1", env)
	require.NoError(t, err)
	assert.Equal(t, 1, rb.Size(), "Buffer size should be 1")
	
	// Avvia il buffer
	rb.Start()
	
	// Attendi che il buffer processi l'elemento
	time.Sleep(50 * time.Millisecond)
	
	// Verifica che l'elemento sia stato processato
	assert.Eventually(t, func() bool {
		return rb.Size() == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Buffer should be empty after processing")
	
	// Verifica che la funzione di pubblicazione sia stata chiamata
	assert.Eventually(t, func() bool {
		publishMutex.Lock()
		defer publishMutex.Unlock()
		return publishCalls > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Publish function should have been called")
}

func TestRetryBufferFailedRetries(t *testing.T) {
	logger := NewMockTestLogger(t)
	
	// Mock per la funzione di pubblicazione che fallisce sempre
	failingPublish := func(env *axcp.Envelope) error {
		return assert.AnError
	}
	
	// Configurazione con valori ridotti per i test
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 10 * time.Millisecond
	config.MaxRetryInterval = 50 * time.Millisecond
	config.MaxAttempts = 3
	
	// Crea il buffer di retry
	rb := NewRetryBuffer(&config, logger, failingPublish)
	defer rb.Close()
	
	// Aggiungi un envelope
	env := axcp.NewEnvelope("test-retry", 0)
	
	err := rb.AddEnvelope("test-retry", env)
	require.NoError(t, err)
	
	// Avvia il buffer
	rb.Start()
	
	// Dovrebbe fare 3 tentativi e poi rimuovere l'elemento
	// Il tempo totale dovrebbe essere circa:
	// 10ms (primo tentativo immediato) + 20ms (secondo tentativo) + 40ms (terzo tentativo) = ~70ms
	time.Sleep(200 * time.Millisecond)
	
	// Verifica che il buffer sia vuoto dopo il massimo dei tentativi
	assert.Equal(t, 0, rb.Size(), "Buffer should be empty after max attempts")
}

func TestRetryBufferCapacity(t *testing.T) {
	logger := NewMockTestLogger(t)
	
	// Mock per la funzione di pubblicazione
	mockPublish := func(env *axcp.Envelope) error {
		return nil
	}
	
	// Configurazione con capacità limitata
	config := DefaultRetryBufferConfig()
	config.MaxCapacity = 5
	
	// Crea il buffer di retry
	rb := NewRetryBuffer(&config, logger, mockPublish)
	
	// Aggiungi più elementi della capacità massima
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("id-%d", i)
		env := axcp.NewEnvelope(id, 0)
		
		err := rb.AddEnvelope(id, env)
		if i < 5 {
			assert.NoError(t, err, "Adding elements up to capacity should succeed")
		} else {
			assert.Error(t, err, "Adding elements beyond capacity should fail")
		}
	}
	
	assert.Equal(t, 5, rb.Size(), "Buffer size should be limited to max capacity")
}

func TestRetryBufferStop(t *testing.T) {
	logger := NewMockTestLogger(t)
	
	// Mock per la funzione di pubblicazione che si blocca
	blockingCalled := false
	blockingDone := make(chan struct{})
	blockingPublish := func(env *axcp.Envelope) error {
		blockingCalled = true
		<-blockingDone // Blocca finché non viene chiuso il canale
		return nil
	}
	
	// Crea il buffer di retry
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 10 * time.Millisecond
	rb := NewRetryBuffer(&config, logger, blockingPublish)
	
	// Aggiungi un elemento
	env := axcp.NewEnvelope("test-stop", 0)
	
	err := rb.AddEnvelope("test-stop", env)
	require.NoError(t, err)
	
	// Avvia il buffer
	rb.Start()
	
	// Attendi che la funzione di pubblicazione venga chiamata
	time.Sleep(50 * time.Millisecond)
	assert.True(t, blockingCalled, "Publish function should have been called")
	
	// Ferma il buffer
	rb.Stop()
	
	// Sblocca la funzione di pubblicazione
	close(blockingDone)
	
	// Verifica che dopo lo stop non si possano aggiungere nuovi elementi
	env2 := axcp.NewEnvelope("test-after-stop", 0)
	
	err = rb.AddEnvelope("test-after-stop", env2)
	assert.Error(t, err, "Adding elements after stop should fail")
}

func TestRetryBufferBackoff(t *testing.T) {
	logger := NewMockTestLogger(t)
	
	// Traccia i tempi di chiamata per verificare il backoff
	callTimes := []time.Time{}
	callTimesMutex := sync.Mutex{}
	
	failingPublish := func(env *axcp.Envelope) error {
		callTimesMutex.Lock()
		callTimes = append(callTimes, time.Now())
		callTimesMutex.Unlock()
		return assert.AnError
	}
	
	// Configurazione con backoff esponenziale
	config := DefaultRetryBufferConfig()
	config.MinRetryInterval = 20 * time.Millisecond
	config.BackoffFactor = 2.0
	config.MaxAttempts = 4
	
	// Crea il buffer di retry
	rb := NewRetryBuffer(&config, logger, failingPublish)
	
	// Aggiungi un envelope
	env := axcp.NewEnvelope("test-backoff", 0)
	
	err := rb.AddEnvelope("test-backoff", env)
	require.NoError(t, err)
	
	// Avvia il buffer
	rb.Start()
	
	// Attendi che vengano effettuati tutti i tentativi
	// (20ms + 40ms + 80ms = 140ms minimo)
	time.Sleep(300 * time.Millisecond)
	
	// Verifica che il buffer sia vuoto dopo il massimo dei tentativi
	assert.Equal(t, 0, rb.Size(), "Buffer should be empty after max attempts")
	
	// Verifica che ci siano almeno 3 chiamate
	callTimesMutex.Lock()
	defer callTimesMutex.Unlock()
	require.GreaterOrEqual(t, len(callTimes), 3, "Should have at least 3 retry attempts")
	
	// Verifica che gli intervalli siano crescenti (backoff)
	if len(callTimes) >= 3 {
		interval1 := callTimes[1].Sub(callTimes[0])
		interval2 := callTimes[2].Sub(callTimes[1])
		t.Logf("Interval 1: %v, Interval 2: %v", interval1, interval2)
		assert.GreaterOrEqual(t, interval2, interval1, "Second interval should be greater than or equal to first interval")
	}
}
