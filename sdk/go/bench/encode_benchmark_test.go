package bench

import (
    "testing"
    "google.golang.org/protobuf/proto"
    pb "github.com/tradephantom/axcp-spec/sdk/go/axcp/pb"
)

func BenchmarkEncode(b *testing.B) {
    msg := &pb.AxcpEnvelope{
        Version: 1,
        TraceId: "bench",
        Profile: 0,
    }
    for i := 0; i < b.N; i++ {
        _, err := proto.Marshal(msg)
        if err != nil {
            b.Fatal(err)
        }
    }
}
