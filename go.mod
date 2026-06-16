module github.com/tradephantomllc/axcp-spec

go 1.25.6

require (
	github.com/quic-go/quic-go v0.59.1
	github.com/tradephantomllc/axcp-spec/sdk/go v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
)

replace github.com/tradephantomllc/axcp-spec/sdk/go/netquic => ./sdk/go/netquic

replace github.com/tradephantomllc/axcp-spec/sdk/go => ./sdk/go
