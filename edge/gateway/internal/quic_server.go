package internal

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"log"

	"github.com/quic-go/quic-go"
)

// EnvelopeHandler gestisce gli AXCP envelope ricevuti tramite stream
type EnvelopeHandler func([]byte)

// TelemetryHandler gestisce i datagrammi di telemetria con il loro profilo
type TelemetryHandler func([]byte, uint32)

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
			// Use stream to receive envelopes
			for {
				// Ricezione semplificata dallo stream
				buf := make([]byte, 4096)
				n, err := str.Read(buf)
				if err != nil {
					log.Printf("[quic] stream read error: %v", err)
					return
				}
				
				// Passa i dati grezzi al gestore
				h(buf[:n])
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

					// Semplice analisi per determinare se è un datagramma di telemetria e il suo profilo
			// Verifichiamo che ci siano almeno 8 byte (4 per l'header, 4 per il profilo)
			if len(data) < 8 {
				log.Printf("[quic] datagram too short: %d bytes", len(data))
				continue
			}

			// Estrai la versione e il tipo dal primo byte (assumendo un formato specifico del protocollo)
			// Primo byte: 2 bit versione, 6 bit tipo
			// Se il tipo è 11 (binario 001011), è un datagramma di telemetria
			headerByte := data[0]
			msgType := headerByte & 0x3F // Estrai i 6 bit meno significativi

			// Se è un datagramma di telemetria (tipo 11)
			if msgType == 11 {
				// Estrai il profilo (assumiamo sia a offset 4, uint32)
				profile := binary.BigEndian.Uint32(data[4:8])
				
				// Passa i dati grezzi e il profilo al gestore
				dgram(data, profile)
			}
		}
	}()
}
