package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

var errRetryPublisherNotConfigured = errors.New("retry buffer publisher is not configured")

// RetryBufferConfig contiene le configurazioni per il buffer di retry
type RetryBufferConfig struct {
	// Capacità massima del buffer in memoria
	MaxCapacity int
	// Intervallo minimo tra i tentativi di ritrasmissione
	MinRetryInterval time.Duration
	// Intervallo massimo tra i tentativi di ritrasmissione
	MaxRetryInterval time.Duration
	// Fattore di backoff per i tentativi di ritrasmissione (es. 2.0 = raddoppia il tempo)
	BackoffFactor float64
	// Numero massimo di tentativi prima di eliminare un messaggio
	MaxAttempts int
}

// RetryItem rappresenta un elemento nel buffer di retry
type RetryItem struct {
	inFlight   bool
	generation uint64
	// ID univoco del messaggio
	ID string
	// Contenuto dell'envelope AXCP
	Envelope *axcp.Envelope
	// Timestamp di creazione
	CreatedAt time.Time
	// Timestamp dell'ultimo tentativo di invio
	LastAttempt time.Time
	// Numero di tentativi effettuati
	Attempts int
	// Prossimo intervallo di retry
	NextRetryInterval time.Duration
}

// Logger definisce l'interfaccia minima richiesta per un logger compatibile
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	Sync() error
}

// StandardLogger è un wrapper attorno a log.Logger che implementa l'interfaccia Logger
type StandardLogger struct {
	l *log.Logger
}

// NewStandardLogger crea un nuovo logger standard
func NewStandardLogger(prefix string) *StandardLogger {
	return &StandardLogger{
		l: log.New(log.Writer(), prefix, log.LstdFlags|log.Lshortfile),
	}
}

// Implementazione dei metodi dell'interfaccia Logger
func (s *StandardLogger) Debug(msg string, fields ...interface{}) {
	s.l.Printf("DEBUG: %s %v", msg, fields)
}

func (s *StandardLogger) Info(msg string, fields ...interface{}) {
	s.l.Printf("INFO: %s %v", msg, fields)
}

func (s *StandardLogger) Warn(msg string, fields ...interface{}) {
	s.l.Printf("WARN: %s %v", msg, fields)
}

func (s *StandardLogger) Error(msg string, fields ...interface{}) {
	s.l.Printf("ERROR: %s %v", msg, fields)
}

func (s *StandardLogger) Fatal(msg string, fields ...interface{}) {
	s.l.Fatalf("FATAL: %s %v", msg, fields)
}

func (s *StandardLogger) Sync() error {
	return nil
}

// RetryBuffer implementa un buffer in memoria per i messaggi AXCP da ritrasmettere
type RetryBuffer struct {
	config      RetryBufferConfig
	items       map[string]*RetryItem
	queue       []string
	mutex       sync.RWMutex
	logger      Logger
	metrics     *RetryMetrics
	publishFn   func(*axcp.Envelope) error
	ctx         context.Context
	cancelFunc  context.CancelFunc
	stopped     bool
	started     bool
	configError error
	wg          sync.WaitGroup
}

// DefaultRetryBufferConfig ritorna una configurazione predefinita per il retry buffer
func DefaultRetryBufferConfig() RetryBufferConfig {
	return RetryBufferConfig{
		MaxCapacity:      1000,
		MinRetryInterval: 1 * time.Second,
		MaxRetryInterval: 5 * time.Minute,
		BackoffFactor:    2.0,
		MaxAttempts:      5,
	}
}

// NewRetryBuffer crea un nuovo buffer di retry
func NewRetryBuffer(config *RetryBufferConfig, logger Logger, publishFn func(*axcp.Envelope) error) *RetryBuffer {
	if config == nil {
		defaultConfig := DefaultRetryBufferConfig()
		config = &defaultConfig
	}
	if logger == nil {
		logger = NewStandardLogger("[retry-buffer] ")
	}
	if publishFn == nil {
		publishFn = func(*axcp.Envelope) error {
			return errRetryPublisherNotConfigured
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	rb := &RetryBuffer{
		config:      *config,
		items:       make(map[string]*RetryItem),
		queue:       make([]string, 0),
		configError: config.Validate(),
		logger:      logger,
		publishFn:   publishFn,
		ctx:         ctx,
		cancelFunc:  cancel,
	}

	return rb
}

// SetMetrics associa le metriche al buffer di retry
func (rb *RetryBuffer) SetMetrics(metrics *RetryMetrics) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	rb.metrics = metrics
}

// AddEnvelope aggiunge un envelope al buffer di retry
func (rb *RetryBuffer) AddEnvelope(id string, env *axcp.Envelope) error {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	if rb.configError != nil {
		return rb.configError
	}
	if id == "" {
		return errors.New("retry item ID is required")
	}
	if env == nil {
		return errors.New("retry envelope is required")
	}
	if rb.stopped {
		return fmt.Errorf("retry buffer is stopped")
	}

	clone := &axcp.Envelope{AxcpEnvelope: *proto.Clone(&env.AxcpEnvelope).(*pb.AxcpEnvelope)}
	if item, exists := rb.items[id]; exists {
		item.Envelope = clone
		item.generation++
		item.Attempts = 0
		item.NextRetryInterval = rb.config.MinRetryInterval
		return nil
	}
	if len(rb.queue) >= rb.config.MaxCapacity {
		if rb.metrics != nil {
			rb.metrics.RecordRetryDropped(rb.ctx)
		}
		return fmt.Errorf("retry buffer is full (capacity: %d)", rb.config.MaxCapacity)
	}

	// Crea un nuovo elemento
	item := &RetryItem{
		ID:                id,
		Envelope:          clone,
		CreatedAt:         time.Now(),
		LastAttempt:       time.Time{}, // Zero time
		Attempts:          0,
		NextRetryInterval: rb.config.MinRetryInterval,
	}

	// Aggiungi l'elemento al buffer e alla coda
	rb.items[id] = item
	rb.queue = append(rb.queue, id)

	if rb.metrics != nil {
		rb.metrics.SetRetryQueueSize(rb.ctx, int64(len(rb.queue)))
	}

	rb.logger.Debug("Added envelope to retry buffer", "id", id, "queue_size", len(rb.queue))

	return nil
}

// Start avvia il processo di gestione dei retry
func (rb *RetryBuffer) Start() {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	if rb.stopped || rb.started {
		return
	}
	if rb.configError != nil {
		rb.logger.Error("Retry worker configuration invalid", "error", rb.configError)
		return
	}
	rb.started = true
	rb.wg.Add(1)
	go rb.processRetries()
}

// Stop ferma il processo di gestione dei retry
func (rb *RetryBuffer) Stop() {
	// Impostiamo lo stato stopped
	rb.mutex.Lock()
	rb.stopped = true
	rb.mutex.Unlock()

	// Annulliamo il contesto per tutte le operazioni pendenti
	rb.cancelFunc()

	// Attendiamo che tutte le goroutine completino
	rb.wg.Wait()

	// Log di debug
	if rb.logger != nil {
		rb.logger.Debug("Retry buffer stopped successfully")
	}
}

// Size ritorna il numero di elementi nel buffer
func (rb *RetryBuffer) Size() int {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()
	return len(rb.queue)
}

// processRetries è il worker che processa gli elementi nella coda di retry
func (rb *RetryBuffer) processRetries() {
	defer rb.wg.Done()

	// Usa un intervallo molto più breve (10ms) per i test
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	if rb.logger != nil {
		rb.logger.Debug("Retry worker started with interval", "interval_ms", 10)
	}

	for {
		select {
		case <-rb.ctx.Done():
			return
		case <-ticker.C:
			rb.retryPendingItems()
		}
	}
}

// retryPendingItems processa gli elementi pronti per il retry
func (rb *RetryBuffer) retryPendingItems() {
	now := time.Now()
	var toRetry []*RetryItem

	// Prima fase: trova elementi pronti per il retry
	rb.mutex.RLock()
	for _, id := range rb.queue {
		item := rb.items[id]
		// Se è il primo tentativo o è passato abbastanza tempo dall'ultimo tentativo
		if !item.inFlight && (item.Attempts == 0 || now.Sub(item.LastAttempt) >= item.NextRetryInterval) {
			toRetry = append(toRetry, item)
		}
	}
	rb.mutex.RUnlock()

	// Seconda fase: processa gli elementi pronti
	for _, item := range toRetry {
		rb.processRetryItem(item)
	}
}

// processRetryItem gestisce il retry di un singolo elemento
func (rb *RetryBuffer) processRetryItem(item *RetryItem) {
	// Acquisisce il lock prima di qualsiasi modifica all'item
	// e lo mantiene fino alla fine della funzione per evitare race condition
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	// Verifichiamo che l'item esista ancora nel buffer
	// potrebbe essere stato rimosso da un'altra goroutine
	if current, exists := rb.items[item.ID]; !exists || current != item || item.inFlight || rb.stopped {
		return
	}

	// Incrementa il contatore dei tentativi e registra l'ultimo tentativo
	item.inFlight = true
	generation := item.generation
	item.Attempts++
	item.LastAttempt = time.Now()

	// Creiamo una copia dell'envelope per processarla
	// senza bloccare il mutex durante la chiamata di rete
	envelopeCopy := &axcp.Envelope{AxcpEnvelope: *proto.Clone(&item.Envelope.AxcpEnvelope).(*pb.AxcpEnvelope)}

	// Rilascia il mutex durante l'operazione di pubblicazione
	rb.mutex.Unlock()

	// Prova a pubblicare l'envelope
	err := rb.publishFn(envelopeCopy)

	// Riacquisisce il mutex per il resto della funzione
	rb.mutex.Lock()

	item.inFlight = false
	if item.generation != generation {
		return
	}

	// Se la pubblicazione è riuscita, rimuovi l'elemento dal buffer
	if err == nil {
		rb.removeItemLocked(item.ID)
		rb.logger.Debug("Retry successful, removed from buffer", "id", item.ID, "attempts", item.Attempts)

		if rb.metrics != nil {
			rb.metrics.RecordRetrySuccess(rb.ctx)
			rb.metrics.SetRetryQueueSize(rb.ctx, int64(len(rb.queue)))
		}
		return
	}

	// Pubblicazione fallita
	if rb.metrics != nil {
		rb.metrics.RecordRetryAttempt(rb.ctx)
	}

	// Se abbiamo raggiunto il numero massimo di tentativi, rimuovi l'elemento
	if item.Attempts >= rb.config.MaxAttempts {
		rb.removeItemLocked(item.ID)
		rb.logger.Warn("Max retry attempts reached, dropped from buffer",
			"id", item.ID,
			"max_attempts", rb.config.MaxAttempts,
			"error", err)

		if rb.metrics != nil {
			rb.metrics.RecordRetryDropped(rb.ctx)
			rb.metrics.SetRetryQueueSize(rb.ctx, int64(len(rb.queue)))
		}
		return
	}

	// Calcola il prossimo intervallo di retry con backoff esponenziale
	delay := float64(rb.config.MinRetryInterval) * math.Pow(rb.config.BackoffFactor, float64(item.Attempts-1))
	nextInterval := rb.config.MaxRetryInterval
	if delay < float64(rb.config.MaxRetryInterval) {
		nextInterval = time.Duration(delay)
	}

	item.NextRetryInterval = nextInterval

	if rb.metrics != nil {
		rb.metrics.RecordRetryDelay(rb.ctx, nextInterval.Seconds())
	}

	rb.logger.Debug("Retry failed, scheduled next attempt",
		"id", item.ID,
		"attempts", item.Attempts,
		"next_interval", nextInterval,
		"error", err)
}

// removeItemLocked rimuove un elemento dal buffer (deve essere chiamato con il mutex già acquisito)
func (rb *RetryBuffer) removeItemLocked(id string) {
	// Rimuovi l'elemento dalla mappa
	delete(rb.items, id)

	// Rimuovi l'elemento dalla coda
	for i, queueID := range rb.queue {
		if queueID == id {
			// Rimozione efficiente senza preservare l'ordine
			rb.queue[i] = rb.queue[len(rb.queue)-1]
			rb.queue = rb.queue[:len(rb.queue)-1]
			break
		}
	}
}

// Close implementa l'interfaccia io.Closer
func (rb *RetryBuffer) Close() error {
	rb.Stop()
	return nil
}

// Validate rejects configurations that cannot provide bounded retry behavior.
func (c RetryBufferConfig) Validate() error {
	if c.MaxCapacity <= 0 {
		return errors.New("retry capacity must be greater than zero")
	}
	if c.MaxAttempts <= 0 {
		return errors.New("retry attempts must be greater than zero")
	}
	if c.MinRetryInterval <= 0 {
		return errors.New("retry minimum interval must be positive")
	}
	if c.MaxRetryInterval < c.MinRetryInterval {
		return errors.New("retry maximum interval must be at least the minimum interval")
	}
	if math.IsNaN(c.BackoffFactor) || math.IsInf(c.BackoffFactor, 0) || c.BackoffFactor < 1 {
		return errors.New("retry backoff factor must be finite and at least one")
	}
	return nil
}
