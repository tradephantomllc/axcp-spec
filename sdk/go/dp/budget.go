package dp

import "sync"

type Budget struct {
	Epsilon float64
	Delta   float64
	mu      sync.Mutex
	usedE   float64
	usedD   float64
}

func NewBudget(epsilon, delta float64) *Budget {
	return &Budget{Epsilon: epsilon, Delta: delta}
}

func (b *Budget) Consume(e, d float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.usedE+e > b.Epsilon || b.usedD+d > b.Delta {
		return false
	}
	b.usedE += e
	b.usedD += d
	return true
}

func (b *Budget) Remaining() (float64, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Epsilon - b.usedE, b.Delta - b.usedD
}
