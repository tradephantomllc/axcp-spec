module github.com/tradephantomllc/axcp-spec/sdk/go

go 1.25.6

require (
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.11.1
	github.com/tradephantomllc/axcp-spec/sdk/go/netquic v0.0.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/quic-go v0.59.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/tradephantomllc/axcp-spec/sdk/go/netquic => ./netquic
