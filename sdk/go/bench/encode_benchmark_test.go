package bench

import (
    "os"
    "path/filepath"
    "testing"
    "google.golang.org/protobuf/proto"
    pb "github.com/tradephantom/axcp-spec/sdk/go/axcp/pb"
)

func BenchmarkEncode(b *testing.B) {
    // Check if protobuf artifacts are available
    if !protobufAvailable() {
        b.Skip("Skipping benchmark: protobuf artifacts not available")
        return
    }

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

// protobufAvailable checks if the required protobuf files are present
func protobufAvailable() bool {
    // Check if the internal/pb directory exists with generated files
    pbDir := filepath.Join("..", "internal", "pb")
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
