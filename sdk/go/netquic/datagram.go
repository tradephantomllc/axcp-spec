package netquic

import (
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"google.golang.org/protobuf/proto"
)

func (c *Client) SendTelemetry(d *axcp.TelemetryDatagram) error {
	payload, _ := proto.Marshal(d)
	frame := append([]byte{0xA0, 0, 0}, payload...)
	return c.conn.SendDatagram(frame)
}
