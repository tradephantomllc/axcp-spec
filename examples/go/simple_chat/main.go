package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

func main() {
	addr := "localhost:61300"
	tlsConf := netquic.InsecureTLSConfig()

	go func() {
		listener, err := quic.ListenAddr(addr, tlsConf, nil)
		if err != nil { log.Fatal(err) }
		sess, _ := listener.Accept(context.Background())
		stream, _ := sess.AcceptStream(context.Background())
		client := &netquic.Client{Stream: stream}
		for {
			env, err := client.RecvEnvelope()
			if err != nil { return }
			client.SendEnvelope(env)
		}
	}()

	client, err := netquic.Dial(addr, tlsConf)
	if err != nil { log.Fatal(err) }
	defer client.Close()

	env := axcp.NewEnvelope(uuid.NewString(), 0)
	env.Payload = &axcp.Envelope_CapabilityMsg{
		CapabilityMsg: &axcp.CapabilityMessage{
			Kind: &axcp.CapabilityMessage_Request{
				Request: &axcp.CapabilityRequest{Ids: []string{"search_query"}},
			},
		},
	}
	if err := client.SendEnvelope(env); err != nil { log.Fatal(err) }

	reply, err := client.RecvEnvelope()
	if err != nil { log.Fatal(err) }
	fmt.Println("echo trace_id:", reply.TraceId)
}
