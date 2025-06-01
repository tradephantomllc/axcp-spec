package internal

import (
	"math"
	"math/rand"
	"time"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

const (
	sensitivity = 1.0
	epsilon     = 1.0
)

func init() {
	// Inizializza il generatore di numeri casuali
	rand.Seed(time.Now().UnixNano())
}

// LaplaceNoise genera rumore Laplace con parametro b = sensitivity/epsilon
func LaplaceNoise(scale float64) float64 {
	// Genera un numero casuale tra 0 e 1
	u := rand.Float64() - 0.5
	// Trasforma in distribuzione Laplace
	return -scale * math.Copysign(math.Log(1-2*math.Abs(u)), u)
}

// GaussianNoise genera rumore Gaussiano (normale) con deviazione standard data
func GaussianNoise(stdDev float64) float64 {
	// Utilizziamo l'algoritmo Box-Muller per generare valori casuali con distribuzione normale
	x1, x2 := rand.Float64(), rand.Float64()
	z := math.Sqrt(-2*math.Log(x1)) * math.Cos(2*math.Pi*x2)
	return z * stdDev
}

// ApplyNoise applica rumore differenzialmente privato al datagramma di telemetria
// secondo il profilo specificato
func ApplyNoise(td *axcp.TelemetryDatagram) {
	if td == nil {
		return
	}

	// Ottieni i valori attuali
	metric := td.GetMetric()
	value := td.GetValue()

	// Applica rumore in base al tipo di metrica
	switch metric {
	case "cpu.usage":
		// Rumore di Laplace per le percentuali di CPU
		td.Value = math.Max(0, math.Min(100,
			value+LaplaceNoise(sensitivity/epsilon)*100.0))

	case "memory.used", "memory.total":
		// Rumore Gaussiano per i byte di memoria (5% di deviazione standard)
		td.Value = math.Max(0,
			value+GaussianNoise(value*0.05))

	// Aggiungi altri casi per altre metriche se necessario
	default:
		// Per le metriche non specificate, applica un rumore ridotto
		td.Value = value + GaussianNoise(1.0)
	}
}

func applySystemNoise(sys *axcp.SystemTelemetry) {
	if sys == nil {
		return
	}

}

// applyGaussianNoise applica rumore Gaussiano a un valore
func applyGaussianNoise(value float64, stdDev float64) float64 {
	// Genera rumore Gaussiano con media 0 e deviazione standard data
	return rand.NormFloat64() * stdDev
}
