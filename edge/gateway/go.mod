module github.com/tradephantom/axcp-spec/edge/gateway

go 1.21

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/eclipse/paho.mqtt.golang v1.4.3
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/pprof v0.0.0-20230821062121-407c9e7a662f // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/onsi/ginkgo/v2 v2.12.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.19.1
	github.com/prometheus/client_model v0.5.0
	github.com/prometheus/common v0.48.0
	github.com/prometheus/procfs v0.12.0
	github.com/quic-go/quic-go v0.42.0
	github.com/spf13/viper v1.18.2 // indirect
	github.com/stretchr/testify v1.10.0
	go.etcd.io/bbolt v1.3.8
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
	github.com/tradephantom/axcp-spec/sdk/go v0.0.0-00010101000000-000000000000
)

replace github.com/tradephantom/axcp-spec/sdk/go => ../../sdk/go

replace go.etcd.io/bbolt => go.etcd.io/bbolt v1.3.8

replace github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../../sdk/go/axcp/internal/pb

replace github.com/tradephantom/axcp-spec/edge/gateway/internal => ./internal
