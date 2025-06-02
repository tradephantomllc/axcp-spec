module github.com/tradephantom/axcp-spec/edge/rpi-agent

go 1.22

require (
	github.com/shirou/gopsutil/v3 v3.23.6
	github.com/tradephantom/axcp-spec/sdk/go/axcp v0.0.0
	github.com/tradephantom/axcp-spec/sdk/go/netquic v0.0.0
	github.com/tradephantom/axcp-spec/sdk/go/pb v0.0.0
	google.golang.org/protobuf v1.33.0
)

replace (
	github.com/tradephantom/axcp-spec => ../../
	github.com/tradephantom/axcp-spec/sdk/go/axcp => ../../sdk/go/axcp
	github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../../sdk/go/internal/pb
	github.com/tradephantom/axcp-spec/sdk/go/netquic => ../../sdk/go/netquic
	github.com/tradephantom/axcp-spec/sdk/go/pb => ../../sdk/go/pb
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/quic-go/quic-go v0.49.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/tradephantom/axcp-spec/sdk/go/internal/pb v0.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
)
