//go:build ignore
package main

import (
  "encoding/json"
  "fmt"
  "math/rand"
  "github.com/tradephantom/axcp-spec/sdk/go/dp"
)

func main() {
  rand.Seed(42)
  const N = 1000000
  type Out struct{ Mean, Var float64 }
  sum, sum2 := 0.0, 0.0
  for i := 0; i < N; i++ {
    v := dp.LaplaceNoise(1.0)
    sum += v; sum2 += v*v
  }
  lap := Out{Mean: sum / N, Var: sum2 / N}
  sum, sum2 = 0, 0
  for i := 0; i < N; i++ {
    v := dp.GaussianNoise(1.0)
    sum += v; sum2 += v*v
  }
  gau := Out{Mean: sum / N, Var: sum2 / N}
  fmt.Println("LAPLACE")
  b, _ := json.MarshalIndent(lap, "", " ")
  fmt.Println(string(b))
  fmt.Println("GAUSS")
  b, _ = json.MarshalIndent(gau, "", " ")
  fmt.Println(string(b))
}
