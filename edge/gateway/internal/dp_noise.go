package internal

import (
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

// ApplyNoise applica rumore differenzialmente privato al datagramma di telemetria
// secondo il profilo specificato
func ApplyNoise(td *pb.TelemetryDatagram) {
	if td == nil {
		return
	}

	// Non eseguiamo il controllo del profilo qui, assumiamo che il chiamante
	// abbia già verificato se è necessario applicare il rumore

	// Accediamo al campo Payload che è un oneof in pb.TelemetryDatagram
	switch payload := td.GetPayload().(type) {
	case *pb.TelemetryDatagram_System:
		systemStats := payload.System
		if systemStats == nil {
			return
		}
		
		// Applica rumore a CPU usage (con rumore Laplace)
		if systemStats.GetCpuPercent() > 0 {
			// Converte CPU percent da uint32 a float per calcoli e poi torna a uint32
			cpuWithNoise := math.Max(0, math.Min(100, 
				float64(systemStats.GetCpuPercent()) + LaplaceNoise(sensitivity/epsilon)*10.0))
			systemStats.CpuPercent = uint32(cpuWithNoise) // Valore impostato tramite campo generato
		}

		// Applica rumore a Memory usage (con rumore Gaussiano)
		if systemStats.GetMemBytes() > 0 {
			// Converte la memoria con rumore in uint64
			memWithNoise := math.Max(0,
				float64(systemStats.GetMemBytes()) + GaussianNoise(sensitivity)*float64(systemStats.GetMemBytes()))
			systemStats.MemBytes = uint64(memWithNoise) // Valore impostato tramite campo generato
		}

	case *pb.TelemetryDatagram_Tokens:
		// Per i token non applichiamo rumore per ora
		return
	}
}
