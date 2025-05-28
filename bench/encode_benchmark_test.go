package bench

import (
    "testing"
    "github.com/tradephantom/axcp-spec/proto"
)

func BenchmarkEncode(b *testing.B) {
    msg := &proto.AxcpEnvelope{
        Version: 1,
        TraceId: "bench",
        Profile: 0,
    }
    for i := 0; i < b.N; i++ {
        _, _ = proto.Marshal(msg)
    }
}
