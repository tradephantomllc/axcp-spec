module github.com/tradephantom/axcp-spec/sdk/go/pb

go 1.22

require (
	github.com/tradephantom/axcp-spec/sdk/go/internal/pb v0.0.0
	google.golang.org/protobuf v1.33.0
)

replace github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../internal/pb
