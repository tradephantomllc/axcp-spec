package internal

import (
	"math"
	"math/rand"
	"time"
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

// ApplyNoiseToTelemetryData applies differential privacy noise to telemetry data
// based on the profile level. For profile >= 3, it applies both
// Laplace and Gaussian noise to the telemetry metrics.
func ApplyNoiseToTelemetryData(td *TelemetryData) {
	// Verifica che la privacy differenziale sia abilitata
	if !td.DifferentialDP {
		return
	}

	// Applica rumore ai dati di sistema se presenti
	if td.SystemStats != nil {
		// Applica rumore Laplace alla percentuale CPU (valore discreto)
		noisyCPU := float64(td.SystemStats.CPUPercent) + LaplaceNoise(sensitivity/epsilon)
		// Limita i valori tra 0 e 100
		td.SystemStats.CPUPercent = uint32(math.Max(0, math.Min(100, noisyCPU)))
		
		// Applica rumore Gaussiano all'utilizzo memoria (valore continuo)
		noisyMem := float64(td.SystemStats.MemBytes) + GaussianNoise(0.01*float64(td.SystemStats.MemBytes))
		if noisyMem < 0 {
			noisyMem = 0
		}
		td.SystemStats.MemBytes = uint64(noisyMem)
		
		// Applica rumore Laplace alla temperatura (valore continuo con limiti)
		noisyTemp := float64(td.SystemStats.TemperatureC) + LaplaceNoise(sensitivity/epsilon*0.2)
		// Limita i valori a un range ragionevole (ad es. 0-100°C)
		td.SystemStats.TemperatureC = uint32(math.Max(0, math.Min(100, noisyTemp)))
	}

	// Applica rumore ai dati di utilizzo token se presenti
	if td.TokenUsage != nil {
		// Applica rumore Laplace ai conteggi token (valori discreti)
		noisyPrompt := float64(td.TokenUsage.PromptTokens) + LaplaceNoise(sensitivity/epsilon*0.5)
		noisyCompletion := float64(td.TokenUsage.CompletionTokens) + LaplaceNoise(sensitivity/epsilon*0.5)
		
		// Assicurati che i valori rimangano positivi
		if noisyPrompt < 0 {
			noisyPrompt = 0
		}
		if noisyCompletion < 0 {
			noisyCompletion = 0
		}
		
		td.TokenUsage.PromptTokens = uint32(noisyPrompt)
		td.TokenUsage.CompletionTokens = uint32(noisyCompletion)
	}

	// Applica rumore ai dati di latenza se presenti
	if td.LatencyStats != nil {
		// Applica rumore Gaussiano alle latenze (valori continui)
		noisyRequest := float64(td.LatencyStats.RequestLatencyMs) + 
			GaussianNoise(math.Max(1.0, float64(td.LatencyStats.RequestLatencyMs)*0.05))
		noisyResponse := float64(td.LatencyStats.ResponseLatencyMs) + 
			GaussianNoise(math.Max(1.0, float64(td.LatencyStats.ResponseLatencyMs)*0.05))
		
		// Assicurati che i valori rimangano positivi
		if noisyRequest < 0 {
			noisyRequest = 0
		}
		if noisyResponse < 0 {
			noisyResponse = 0
		}
		
		td.LatencyStats.RequestLatencyMs = uint32(noisyRequest)
		td.LatencyStats.ResponseLatencyMs = uint32(noisyResponse)
	}
}
