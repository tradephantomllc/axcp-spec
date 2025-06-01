package internal

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

// EnvelopeHandler gestisce i messaggi AXCP in arrivo
type EnvelopeHandler func(*axcp.Envelope)

// TelemetryHandler gestisce i datagrammi di telemetria
type TelemetryHandler func(*axcp.TelemetryDatagram)

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
					var td axcp.TelemetryDatagram
					if err := td.Unmarshal(data[1:]); err == nil {
						// Log per debug
						log.Printf("[quic] ricevuto datagramma telemetria, profilo: %d", td.Profile)
						dgram(&td)
					} else {
						log.Printf("[quic] errore unmarshal telemetria: %v", err)
					}
				}
			}
		}(conn)
	}
}
