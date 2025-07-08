package internal

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/quic-go/quic-go"
	pb "github.com/tradephantom/axcp-spec/sdk/go/axcp/internal/pb"
	"google.golang.org/protobuf/proto"
)

// EnvelopeHandler gestisce i messaggi AXCP in arrivo
type EnvelopeHandler func(*pb.Envelope)

// TelemetryHandler gestisce i datagrammi di telemetria
type TelemetryHandler func(*pb.TelemetryDatagram)

// RunQuicServer avvia il server QUIC con supporto per stream e datagrammi
func RunQuicServer(addr string, tlsConf *tls.Config, h EnvelopeHandler, dgram TelemetryHandler) error {
	listener, err := quic.ListenAddr(addr, tlsConf, nil)
	if err != nil {
		return err
	}
	log.Printf("[quic] in ascolto su %s", addr)

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}

		// Gestione stream
		go func(c quic.Connection) {
			for {
				stream, err := c.AcceptStream(context.Background())
				if err != nil {
					log.Printf("[quic] errore accettazione stream: %v", err)
					return
				}

				// Gestisci lo stream in una goroutine separata
				go func(s quic.Stream) {
					defer s.Close()
					// TODO: Implementa la gestione del messaggio AXCP
				}(stream)
			}
		}(conn)

		// Gestione datagrammi
		go func(c quic.Connection) {
			for {
				data, err := c.ReceiveDatagram(context.Background())
				if err != nil {
					log.Printf("[quic] errore ricezione datagramma: %v", err)
					return
				}

				// Se il datagramma inizia con 0xA0, è un datagramma di telemetria
				if len(data) > 0 && data[0] == 0xA0 {
					var td pb.TelemetryDatagram
					if err := proto.Unmarshal(data[1:], &td); err == nil {
						// Log per debug con informazioni di base sul datagramma di telemetria
						timestamp := td.GetTimestampMs()
						log.Printf("[quic] ricevuto datagramma telemetria, timestamp: %d", timestamp)
						dgram(&td)
					} else {
						log.Printf("[quic] errore unmarshal telemetria: %v", err)
					}
				}
			}
		}(conn)
	}
}
