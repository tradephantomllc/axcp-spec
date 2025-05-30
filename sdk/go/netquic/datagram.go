package netquic

import (
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"google.golang.org/protobuf/proto"
)

// SendTelemetry invia un datagramma di telemetria attraverso la connessione QUIC
func (c *Client) SendTelemetry(d *axcp.TelemetryDatagram) error {
	// Marshal del payload protobuf
	payload, err := proto.Marshal(d)
	if err != nil {
		return err
	}
	
	// Crea il frame con l'header (0xA0, 0, 0) seguito dal payload
	frame := append([]byte{0xA0, 0, 0}, payload...)
	
	// Invia il messaggio attraverso la connessione QUIC
	// Nota: in quic-go v0.38.1, il metodo corretto è SendMessage
	return c.conn.SendMessage(frame)
}
