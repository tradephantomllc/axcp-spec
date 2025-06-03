module github.com/tradephantom/axcp-spec/edge/gateway

go 1.22

require (
	github.com/eclipse/paho.mqtt.golang v1.4.3
	github.com/quic-go/quic-go v0.49.0
	github.com/stretchr/testify v1.10.0
	github.com/tradephantom/axcp-spec/sdk/go/axcp v0.0.0
	github.com/tradephantom/axcp-spec/sdk/go/netquic v0.0.0
	github.com/tradephantom/axcp-spec/sdk/go/pb v0.0.0
	go.etcd.io/bbolt v1.3.8
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tradephantom/axcp-spec/sdk/go/internal/pb v0.0.0 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Configurazione dei replace per i moduli locali
replace github.com/tradephantom/axcp-spec/sdk/go/axcp => ../../sdk/go/axcp

replace github.com/tradephantom/axcp-spec/sdk/go/netquic => ../../sdk/go/netquic

replace github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../../sdk/go/internal/pb

replace github.com/tradephantom/axcp-spec/sdk/go/pb => ../../sdk/go/pb

replace go.etcd.io/bbolt => go.etcd.io/bbolt v1.3.8
