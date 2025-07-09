module github.com/tradephantom/axcp-spec

go 1.23.4

require github.com/quic-go/quic-go v0.53.0

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
)

replace github.com/tradephantom/axcp-spec/sdk/go/axcp => ./sdk/go/axcp

replace github.com/tradephantom/axcp-spec/sdk/go/netquic => ./sdk/go/netquic
