module example/simple_chat

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/quic-go/quic-go v0.40.1
	github.com/tradephantom/axcp-spec/sdk/go v0.0.0-00010101000000-000000000000
)

replace github.com/tradephantom/axcp-spec/sdk/go => ../../../sdk/go
