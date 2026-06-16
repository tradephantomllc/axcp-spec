package netquic

import (
	"testing"

	"github.com/quic-go/quic-go"
)

func TestDatagramSupportDirection(t *testing.T) {
	var state quic.ConnectionState
	state.SupportsDatagrams.Remote = true

	if !supportsOutboundDatagrams(state) {
		t.Fatal("outbound datagrams should depend on peer-advertised support")
	}
	if supportsInboundDatagrams(state) {
		t.Fatal("inbound datagrams should not be enabled by remote support alone")
	}

	state.SupportsDatagrams.Remote = false
	state.SupportsDatagrams.Local = true

	if supportsOutboundDatagrams(state) {
		t.Fatal("outbound datagrams should not be enabled by local support alone")
	}
	if !supportsInboundDatagrams(state) {
		t.Fatal("inbound datagrams should depend on local datagram support")
	}
}
