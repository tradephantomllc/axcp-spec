module telemetry_example

go 1.22

require (
	github.com/quic-go/quic-go v0.40.1
	github.com/tradephantom/axcp-spec/sdk/go v0.0.0
)

// Use local version of the SDK
replace github.com/tradephantom/axcp-spec/sdk/go => ../../..

require (
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20230821062121-407c9e7a662f // indirect
	github.com/onsi/ginkgo/v2 v2.12.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.4.1 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.12.1-0.20230815132531-74c255bcf846 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
