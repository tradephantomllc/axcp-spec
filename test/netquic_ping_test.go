package test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

func TestPing(t *testing.T) {
	addr := "127.0.0.1:61234"
	tlsConf := netquic.InsecureTLSConfig()

	go func() {
		listener, _ := quic.ListenAddr(addr, tlsConf, nil)
		sess, _ := listener.Accept(context.Background())
		str, _ := sess.AcceptStream(context.Background())
		for {
			env, err := (&netquic.Client{stream: str}).RecvEnvelope()
			if err != nil {
				return
			}
			(&netquic.Client{stream: str}).SendEnvelope(env)
		}
	}()

	client, err := netquic.Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	orig := axcp.NewEnvelope(uuid.NewString(), 0)
	if err := client.SendEnvelope(orig); err != nil {
		t.Fatalf("send: %v", err)
	}
	got, err := client.RecvEnvelope()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if got.TraceId != orig.TraceId {
		t.Fatalf("echo mismatch")
	}
}
