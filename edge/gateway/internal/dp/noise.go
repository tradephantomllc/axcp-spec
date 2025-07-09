package dp

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

// ApplyNoise applies differential privacy noise to telemetry data based on the topic
func ApplyNoise(td *axcp.TelemetryDatagram, topic string, lookup *BudgetLookup) error {
	if td == nil {
		return fmt.Errorf("telemetry datagram is nil")
	}

	// Get the budget for this topic
	budget, err := lookup.ForTopic(topic)
	if err != nil {
		// If no specific budget is found, use the default
		budget, _ = lookup.ForTopic("*")
	}

	// Skip if no budget is available
	if budget == nil {
		return nil
	}

	payload := td.GetPayload()
	if payload == nil {
		return fmt.Errorf("telemetry payload is nil")
	}

	switch p := payload.(type) {
	case *axcp.TelemetryDatagram_System:
		// Apply noise to system stats
		sys := p.System
		if sys == nil {
			return fmt.Errorf("system stats is nil")
		}

		// Apply noise to CPU percentage
		if budget.Epsilon > 0 && budget.Delta > 0 {
			// Generate noise once for this datagram
			rand.Seed(time.Now().UnixNano())
			noise := laplaceNoise(budget.Epsilon, budget.Delta, budget.ClipNorm)
			
			// Apply noise to CPU percentage (0-100 range)
			cpuWithNoise := float64(sys.CpuPercent) + noise
			// Clip to [0, 100] for CPU percentage
			clipped := applyClip(cpuWithNoise, 100)
			if clipped < 0 {
				clipped = 0
			} else if clipped > 100 {
				clipped = 100
			}
			sys.CpuPercent = uint32(clipped) // Clip to 0-100%
		}

	case *axcp.TelemetryDatagram_Tokens:
		// Apply noise to token usage
		tokens := p.Tokens
		if tokens == nil {
			return fmt.Errorf("token usage is nil")
		}

		if budget.Epsilon > 0 && budget.Delta > 0 {
			// Generate noise once for this datagram
			rand.Seed(time.Now().UnixNano())
			noise := laplaceNoise(budget.Epsilon, budget.Delta, budget.ClipNorm)
			
			// Apply noise to token counts
			if tokens.PromptTokens > 0 {
				promptWithNoise := float64(tokens.PromptTokens) + noise
				// Clip to [0, clipNorm] since token counts can't be negative
				clipped := applyClip(promptWithNoise, budget.ClipNorm)
				if clipped < 0 {
					clipped = 0
				}
				tokens.PromptTokens = uint32(clipped)
			}
			
			if tokens.CompletionTokens > 0 {
				completionWithNoise := float64(tokens.CompletionTokens) + noise
				// Clip to [0, clipNorm] since token counts can't be negative
				clipped := applyClip(completionWithNoise, budget.ClipNorm)
				if clipped < 0 {
					clipped = 0
				}
				tokens.CompletionTokens = uint32(clipped)
			}
		}

	default:
		return fmt.Errorf("unsupported telemetry payload type: %T", p)
	}

	return nil
}

// laplaceNoise generates Laplace noise with the given parameters
func laplaceNoise(epsilon, delta, sensitivity float64) float64 {
	if epsilon <= 0 || delta <= 0 || sensitivity <= 0 {
		return 0
	}

	// Generate random value in [0,1)
	u := rand.Float64() - 0.5
	// Scale by sensitivity / epsilon
	scale := sensitivity / epsilon

	// Apply the inverse CDF of the Laplace distribution
	if u < 0 {
		return scale * (1.0 / delta) * (1.0 - 2.0*u)
	}
	return -scale * (1.0 / delta) * (1.0 - 2.0*u)
}

// applyClip ensures the value is within [-clip, clip] range
func applyClip(value, clip float64) float64 {
	if value > clip {
		return clip
	}
	if value < -clip {
		return -clip
	}
	return value
}
