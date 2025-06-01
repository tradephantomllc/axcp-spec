module github.com/tradephantom/axcp-spec/sdk/go/bench

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/tradephantom/axcp-spec/sdk/go/axcp v0.0.0
	github.com/tradephantom/axcp-spec/sdk/go/internal/pb v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.6
)

replace github.com/tradephantom/axcp-spec/sdk/go/axcp => ../axcp

replace github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../internal/pb
