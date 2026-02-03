module github.com/tradephantomllc/axcp-spec

go 1.24

require (
	github.com/quic-go/quic-go v0.59.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace github.com/tradephantomllc/axcp-spec/sdk/go/axcp => ./sdk/go/axcp

replace github.com/tradephantomllc/axcp-spec/sdk/go/netquic => ./sdk/go/netquic
