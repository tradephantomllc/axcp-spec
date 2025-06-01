module test

go 1.22.7

require (
	github.com/google/uuid v1.6.0
	github.com/tradephantom/axcp-spec/sdk/go v0.0.0
)

require google.golang.org/protobuf v1.31.0 // indirect

replace github.com/tradephantom/axcp-spec/sdk/go => ../sdk/go
