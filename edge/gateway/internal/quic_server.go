package internal

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

type EnvelopeHandler func(*axcp.Envelope)
type TelemetryHandler func(*axcp.TelemetryDatagram)

func RunQuicServer(addr string, tlsConf *tls.Config, h EnvelopeHandler, dgram TelemetryHandler) error {
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
		go handleSession(sess, h, dgram)
	}
}

func handleSession(sess quic.Connection, h EnvelopeHandler, dgram TelemetryHandler) {
	// Handle stream-based envelope communication
	str, err := sess.AcceptStream(context.Background())
	if err == nil {
		go func() {
			client := &axcp.QuicStream{Stream: str}
			for {
				env, err := client.RecvEnvelope()
				if err != nil {
					return
				}
				h(env)
			}
		}()
	}

	// Handle datagram-based telemetry
	go func() {
		for {
			data, err := sess.ReceiveDatagram(context.Background())
			if err != nil {
				log.Printf("[quic] datagram receive error: %v", err)
				return
			}

			// Only process datagrams starting with 0xA0 (telemetry)
			if len(data) > 0 && data[0] == 0xA0 {
				td := &axcp.TelemetryDatagram{}
				if err := td.XXX_Unmarshal(data[1:]); err == nil {
					dgram(td)
				} else {
					log.Printf("[quic] failed to unmarshal telemetry: %v", err)
				}
			}
		}
	}()
}
