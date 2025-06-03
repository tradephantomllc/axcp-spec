package internal

import (
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/tradephantom/axcp-spec/sdk/go/pb"
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

// ApplyNoise applica rumore differenzialmente privato al datagramma di telemetria.
func ApplyNoise(td *pb.TelemetryDatagram) {
	if td == nil {
		return
	}

	sysStats := td.GetSystem()
	if sysStats != nil {
		// Applica rumore a SystemStats
		log.Printf("[dp_noise] Applicazione rumore a SystemStats: CPU %d%%, Mem %d bytes", sysStats.CpuPercent, sysStats.MemBytes)

		// Rumore di Laplace per le percentuali di CPU
		originalCpu := float64(sysStats.CpuPercent)
		noisyCpu := originalCpu + LaplaceNoise(sensitivity/epsilon)
		sysStats.CpuPercent = uint32(math.Max(0, math.Min(100, noisyCpu))) // Assicura che il valore sia tra 0 e 100

		// Rumore Gaussiano per l'utilizzo della memoria
		originalMem := float64(sysStats.MemBytes)
		// Assumiamo una deviazione standard, ad esempio il 5% del valore massimo possibile o un valore fisso
		stdDev := 1024.0 // Esempio: deviazione standard di 1KB per la memoria
		noisyMem := applyGaussianNoise(originalMem, stdDev)
		sysStats.MemBytes = uint64(math.Max(0, noisyMem)) // Assicura che il valore sia non negativo

		log.Printf("[dp_noise] SystemStats con rumore: CPU %d%%, Mem %d bytes", sysStats.CpuPercent, sysStats.MemBytes)
		return
	} // Closes 'if sysStats != nil'

	tokenUsage := td.GetTokens()
	if tokenUsage != nil {
		// TODO: Implementare l'applicazione del rumore per TokenUsage se necessario.
		// Per ora, logghiamo solamente se riceviamo questo tipo di payload.
		log.Printf("[dp_noise] Ricevuto TokenUsage, nessuna applicazione di rumore implementata: Prompt %d, Completion %d", tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
		return
	} // Closes 'if tokenUsage != nil'

	log.Printf("[dp_noise] Tipo di payload TelemetryDatagram non riconosciuto o nil, nessun rumore applicato.")
} // Closes 'func ApplyNoise'

// applyGaussianNoise applica rumore Gaussiano a un valore
func applyGaussianNoise(value float64, stdDev float64) float64 {
	// Genera rumore Gaussiano con media 0 e deviazione standard data
	// e lo aggiunge al valore originale
	return value + (rand.NormFloat64() * stdDev)
}
