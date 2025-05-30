// SPDX-License-Identifier: Apache-2.0
// Placeholder – v0.3 structure only
package dp

import (
	"math/rand"
)

// Config holds differential privacy parameters
type Config struct {
	Epsilon    float64 `yaml:"epsilon"`
	Delta      float64 `yaml:"delta"`
	Mechanism  string  `yaml:"mechanism"`
	ClipNorm   float64 `yaml:"clip_norm"`
}

// DPNoiseGenerator interface for different noise mechanisms
type DPNoiseGenerator interface {
	AddNoise(value float64) float64
}

// NewNoiseGenerator creates a new noise generator based on config
func NewNoiseGenerator(cfg Config) (DPNoiseGenerator, error) {
	switch cfg.Mechanism {
	case "laplace":
		return &LaplaceNoise{Scale: 1.0 / cfg.Epsilon}, nil
	case "gaussian":
		return &GaussianNoise{
			StdDev: calcGaussianNoiseScale(cfg.Epsilon, cfg.Delta, cfg.ClipNorm),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported noise mechanism: %s", cfg.Mechanism)
	}
}

// LaplaceNoise implements DP with Laplace mechanism
type LaplaceNoise struct {
	Scale float64
}

func (l *LaplaceNoise) AddNoise(value float64) float64 {
	// TODO: Implement actual Laplace noise
	return value + (rand.Float64() - 0.5) * l.Scale
}

// GaussianNoise implements DP with Gaussian mechanism
type GaussianNoise struct {
	StdDev float64
}

func (g *GaussianNoise) AddNoise(value float64) float64 {
	// TODO: Implement actual Gaussian noise
	return value + rand.NormFloat64() * g.StdDev
}

func calcGaussianNoiseScale(epsilon, delta, clipNorm float64) float64 {
	// Calculate noise scale for Gaussian mechanism
	// See: https://arxiv.org/abs/1805.06530
	return (2 * math.Log(1.25/delta) * clipNorm) / (epsilon * epsilon)
}
