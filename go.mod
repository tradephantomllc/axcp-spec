module github.com/tradephantomllc/axcp-spec

go 1.26.10

require (
	github.com/quic-go/quic-go v0.63.0
	github.com/tradephantomllc/axcp-spec/sdk/go v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/tradephantomllc/axcp-spec/sdk/go => ./sdk/go
