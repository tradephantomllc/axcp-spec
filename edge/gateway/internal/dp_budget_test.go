package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDynamicBudgetCLI(t *testing.T) {
	// Imposta valori personalizzati per il budget
	SetBudget(0.7, 1e-4, 15*time.Minute)
	
	// Ottieni i valori correnti
	e, d, w := GetBudget()
	
	// Verifica che i valori siano stati impostati correttamente
	require.InDelta(t, 0.7, e, 1e-9)
	require.InDelta(t, 1e-4, d, 1e-12)
	require.Equal(t, 15*time.Minute, w)
}
