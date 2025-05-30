package main

import (
	"log"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

func main() {
	addr := ":7143"
	tlsConf := netquic.InsecureTLSConfig()

	broker := internal.NewBroker("tcp://mosquitto:1883")
	handler := func(env *axcp.Envelope) {
		if err := broker.Publish(env); err != nil {
			log.Printf("[mqtt] pub failed: %v", err)
		}
	}

	if err := internal.RunQuicServer(addr, tlsConf, handler); err != nil {
		log.Fatal(err)
	}
}
