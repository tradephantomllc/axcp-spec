package dp

import (
	"math"
	"math/rand"
	"time"
	"gonum.org/v1/gonum/stat/distuv"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func LaplaceNoise(scale float64) float64 {
	u := rand.Float64() - 0.5
	return -scale * math.Copysign(1, u) * math.Log(1-2*math.Abs(u))
}

func GaussianNoise(sigma float64) float64 {
	return distuv.UnitNormal.Rand() * sigma
}
