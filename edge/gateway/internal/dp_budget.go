package internal

import (
	"math"
	"sync/atomic"
	"time"
)

// Variabili atomiche per il budget di differential privacy
var (
	// Epsilon è il parametro principale per il budget di DP
	// Viene memorizzato come bits di float64 per supportare operazioni atomiche
	Epsilon atomic.Uint64

	// Delta è il parametro secondario per il budget di DP
	// Viene memorizzato come bits di float64 per supportare operazioni atomiche
	Delta atomic.Uint64

	// Window è la finestra temporale per il calcolo del budget
	Window atomic.Value // time.Duration
)

// Inizializza i valori di default
func init() {
	// Inizializza con valori di default
	SetBudget(1.0, 1e-5, 1*time.Hour)
}

// SetBudget imposta i parametri del budget DP in modo thread-safe
func SetBudget(e, d float64, w time.Duration) {
	Epsilon.Store(math.Float64bits(e))
	Delta.Store(math.Float64bits(d))
	Window.Store(w)
}

// GetBudget restituisce i parametri correnti del budget DP
func GetBudget() (float64, float64, time.Duration) {
	return math.Float64frombits(Epsilon.Load()),
		math.Float64frombits(Delta.Load()),
		Window.Load().(time.Duration)
}
