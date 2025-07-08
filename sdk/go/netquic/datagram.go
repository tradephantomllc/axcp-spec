package netquic

import (
	"fmt"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp/internal/pb"
	"google.golang.org/protobuf/proto"
)

// SendTelemetry sends a telemetry datagram over the QUIC connection.
// It wraps the TelemetryDatagram in an AxcpEnvelope with the appropriate profile
// and sends it as a datagram.
func (c *Client) SendTelemetry(d *pb.TelemetryDatagram) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("client is not connected")
	}

	// Create an envelope for the telemetry data
	envelope := &pb.AxcpEnvelope{
		Version: 1, // Current protocol version
		Profile: 0, // Basic profile for telemetry
		Payload: &pb.AxcpEnvelope_Telemetry{
			Telemetry: d,
		},
	}

	// Marshal the envelope to protobuf
	payload, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry envelope: %w", err)
	}

	// Send as a datagram (unreliable but faster)
	return c.SendDatagram(payload)
}

// ReceiveTelemetry waits for and receives a telemetry datagram from the QUIC connection.
// It returns the received TelemetryDatagram or an error if the operation fails.
func (c *Client) ReceiveTelemetry() (*pb.TelemetryDatagram, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("client is not connected")
	}

	// Receive a datagram
	data, err := c.ReceiveDatagram()
	if err != nil {
		return nil, fmt.Errorf("failed to receive datagram: %w", err)
	}

	// Unmarshal the envelope
	envelope := &pb.AxcpEnvelope{}
	if err := proto.Unmarshal(data, envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	// Extract the telemetry data from the envelope
	telemetry, ok := envelope.Payload.(*pb.AxcpEnvelope_Telemetry)
	if !ok {
		return nil, fmt.Errorf("received message is not a telemetry datagram")
	}

	return telemetry.Telemetry, nil
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
