package netquic

import (
	"fmt"

	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

// SendTelemetry sends legacy 0xA0 telemetry, accepted only by transport-only gateways.
// For Secure Baseline use SendTelemetryEnvelope after signing the envelope.
func (c *Client) SendTelemetry(d *pb.TelemetryDatagram) error {
	if d == nil || d.Payload == nil {
		return fmt.Errorf("telemetry payload is required")
	}
	data, err := proto.Marshal(d)
	if err != nil {
		return err
	}
	return c.SendDatagram(append([]byte{0xA0}, data...))
}

// SendTelemetryEnvelope sends a caller-signed envelope using the 0xA1 encoding.
func (c *Client) SendTelemetryEnvelope(env *axcp.Envelope) error {
	if env == nil || env.GetTelemetry() == nil || env.GetTelemetry().Payload == nil {
		return fmt.Errorf("telemetry envelope is required")
	}
	data, err := axcp.ToBytes(env)
	if err != nil {
		return err
	}
	return c.SendDatagram(append([]byte{0xA1}, data...))
}

// ReceiveTelemetry receives only legacy 0xA0 telemetry (no application authentication).
func (c *Client) ReceiveTelemetry() (*pb.TelemetryDatagram, error) {
	data, err := c.ReceiveDatagram()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || data[0] != 0xA0 {
		return nil, fmt.Errorf("expected legacy 0xA0 telemetry; use ReceiveTelemetryEnvelope for authenticated envelopes")
	}
	td := &pb.TelemetryDatagram{}
	if err := proto.Unmarshal(data[1:], td); err != nil {
		return nil, err
	}
	if td.Payload == nil {
		return nil, fmt.Errorf("telemetry payload is required")
	}
	return td, nil
}

// ReceiveTelemetryEnvelope decodes 0xA1 without authenticating it. Callers MUST
// verify signature, recipient, timestamp and replay state before using its payload.
func (c *Client) ReceiveTelemetryEnvelope() (*axcp.Envelope, error) {
	data, err := c.ReceiveDatagram()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || data[0] != 0xA1 {
		return nil, fmt.Errorf("expected 0xA1 telemetry envelope")
	}
	env, err := axcp.FromBytes(data[1:])
	if err != nil {
		return nil, err
	}
	if env.GetTelemetry() == nil || env.GetTelemetry().Payload == nil {
		return nil, fmt.Errorf("telemetry payload is required")
	}
	return env, nil
}

// WithSystemStats is a helper function to create and send a system stats telemetry datagram.
func (c *Client) WithSystemStats(cpuPercent uint32, memBytes uint64, tempC uint32) error {
	td := axcp.NewTelemetryDatagram()
	td = axcp.WithSystemStats(td, cpuPercent, memBytes, tempC)
	return c.SendTelemetry(td)
}

// WithTokenUsage is a helper function to create and send a token usage telemetry datagram.
func (c *Client) WithTokenUsage(promptTokens, completionTokens uint32) error {
	td := axcp.NewTelemetryDatagram()
	td = axcp.WithTokenUsage(td, promptTokens, completionTokens)
	return c.SendTelemetry(td)
}
