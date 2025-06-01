module gateway

go 1.22.7

require (
	github.com/eclipse/paho.mqtt.golang v1.4.3
	github.com/quic-go/quic-go v0.42.0
	github.com/tradephantom/axcp-spec/sdk/go/axcp v0.0.0-00010101000000-000000000000
)

replace (
	github.com/tradephantom/axcp-spec => ../../..
	github.com/tradephantom/axcp-spec/sdk/go/axcp => ../../sdk/go/axcp
	github.com/tradephantom/axcp-spec/sdk/go/netquic => ../../sdk/go/netquic
)

require (
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/quic-go/qtls-go1-20 v0.4.1 // indirect
	go.etcd.io/bbolt v1.3.8 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/exp v0.0.0-20221205204356-47842c84f3db // indirect
	golang.org/x/mod v0.11.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.10.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace go.etcd.io/bbolt => go.etcd.io/bbolt v1.3.8

replace github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../../sdk/go/axcp/internal/pb

replace github.com/tradephantom/axcp-spec/edge/gateway/internal => ./internal
