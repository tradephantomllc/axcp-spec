package codec

import (
	"testing"
	"github.com/google/uuid"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

var sample = pb.NewAxcpEnvelope()

func init() {
	sample.TraceId = uuid.NewString()
	sample.Profile = 1
}

func BenchmarkEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := axcp.ToBytes(sample); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode(b *testing.B) {
	raw, _ := axcp.ToBytes(sample)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := axcp.FromBytes(raw); err != nil {
			b.Fatal(err)
		}
	}
}
