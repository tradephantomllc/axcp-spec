module test

go 1.25.6

require (
	github.com/google/uuid v1.6.0
	github.com/tradephantomllc/axcp-spec/sdk/go v0.0.0
)

require google.golang.org/protobuf v1.36.11 // indirect

replace github.com/tradephantomllc/axcp-spec/sdk/go => ../sdk/go
