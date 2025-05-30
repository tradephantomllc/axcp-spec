package internal

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

type EnvelopeHandler func(*axcp.Envelope)

func RunQuicServer(addr string, tlsConf *tls.Config, h EnvelopeHandler) error {
	listener, err := quic.ListenAddr(addr, tlsConf, nil)
	if err != nil {
		return err
	}
	log.Printf("[quic] listening on %s", addr)
	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}
		go handleSession(sess, h)
	}
}

func handleSession(sess quic.Connection, h EnvelopeHandler) {
	str, _ := sess.AcceptStream(context.Background())
	client := &axcp.QuicStream{Stream: str}
	for {
		env, err := client.RecvEnvelope()
		if err != nil {
			return
		}
		h(env)
	}
}
