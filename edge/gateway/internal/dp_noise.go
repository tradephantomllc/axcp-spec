package internal

import (
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/dp"
)

const (
	sensitivity = 1.0
	epsilon     = 1.0
)

// ApplyNoise applies differential privacy noise to telemetry data
// based on the profile level. For profile >= 3, it applies both
// Laplace and Gaussian noise to the telemetry metrics.
func ApplyNoise(td *axcp.TelemetryDatagram) {
	switch p := td.Payload.(type) {
	case *axcp.TelemetryDatagram_System:
		s := p.System
		// Apply Laplace noise to CPU percentage (discrete value)
		s.CpuPercent = uint32(float64(s.CpuPercent) +
			dp.LaplaceNoise(sensitivity/epsilon))
		// Apply Gaussian noise to memory usage (continuous value)
		s.MemBytes = uint64(float64(s.MemBytes) +
			dp.GaussianNoise(0.01*float64(s.MemBytes)))
	}
}
