module github.com/tradephantom/axcp-spec/sdk/go

go 1.23.4

require (
	github.com/google/uuid v1.6.0
	github.com/tradephantom/axcp-spec/sdk/go/axcp v0.0.0
	google.golang.org/protobuf v1.36.6
)

replace github.com/tradephantom/axcp-spec/sdk/go/axcp => ./axcp
