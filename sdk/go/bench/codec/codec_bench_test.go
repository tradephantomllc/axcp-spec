package codec

import (
	"os"
	"path/filepath"
	"testing"
	"github.com/google/uuid"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

var sample = axcp.NewEnvelope(uuid.NewString(), 1)

func BenchmarkEncode(b *testing.B) {
	if !protobufAvailable() {
		b.Skip("Skipping benchmark: protobuf artifacts not available")
		return
	}

	for i := 0; i < b.N; i++ {
		if _, err := axcp.ToBytes(sample); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode(b *testing.B) {
	if !protobufAvailable() {
		b.Skip("Skipping benchmark: protobuf artifacts not available")
		return
	}

	raw, _ := axcp.ToBytes(sample)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := axcp.FromBytes(raw); err != nil {
			b.Fatal(err)
		}
	}
}

// protobufAvailable checks if the required protobuf files are present
func protobufAvailable() bool {
	// Check if the internal/pb directory exists with generated files
	pbDir := filepath.Join("..", "..", "internal", "pb")
	if _, err := os.Stat(pbDir); os.IsNotExist(err) {
		return false
	}
	
	// Check for specific protobuf generated file
	pbFile := filepath.Join(pbDir, "axcp.pb.go")
	if _, err := os.Stat(pbFile); os.IsNotExist(err) {
		return false
	}
	
	return true
}
