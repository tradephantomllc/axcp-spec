package dp

import (
	"math"
	"math/rand"
	"time"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func LaplaceNoise(scale float64) float64 {
	u := rand.Float64() - 0.5
	return -scale * math.Copysign(1, u) * math.Log(1-2*math.Abs(u))
}

// GaussianNoise generates Gaussian noise with mean 0 and standard deviation sigma
// using the Box-Muller transform
func GaussianNoise(sigma float64) float64 {
	u1 := rand.Float64()
	u2 := rand.Float64()
	// Box-Muller transform
	z0 := math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return z0 * sigma
}
