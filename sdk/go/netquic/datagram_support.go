package netquic

import "github.com/quic-go/quic-go"

func supportsOutboundDatagrams(state quic.ConnectionState) bool {
	return state.SupportsDatagrams.Remote
}

func supportsInboundDatagrams(state quic.ConnectionState) bool {
	return state.SupportsDatagrams.Local
}
