package dp

import (
	"math"
	"testing"
)

func TestLaplaceNoise(t *testing.T) {
	v := LaplaceNoise(1.0)
	if math.IsNaN(v) {
		t.Fatal("noise NaN")
	}
}

func TestBudget(t *testing.T) {
	b := NewBudget(1.0, 1e-5)
	if !b.Consume(0.3, 5e-6) {
		t.Fatal("consume should succeed")
	}
	if b.Consume(0.8, 6e-6) {
		t.Fatal("budget overspent")
	}
	e, d := b.Remaining()
	if e < 0.69-1e-9 || d < 5e-6-1e-9 {
		t.Fatal("remaining incorrect")
	}
}
