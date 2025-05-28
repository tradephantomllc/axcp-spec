package bench

import (
    "testing"
    "github.com/tradephantom/axcp-spec/sdk/go/internal/pb"
)

func BenchmarkEncode(b *testing.B) {
    msg := &pb.AxcpEnvelope{
        Version: 1,
        TraceId: "bench",
        Profile: 0,
    }
    for i := 0; i < b.N; i++ {
        _, _ = pb.Marshal(msg)
    }
}
