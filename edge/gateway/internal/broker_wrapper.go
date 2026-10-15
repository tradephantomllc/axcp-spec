package internal

import (
	"fmt"
	"log"

	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
)

// BrokerWrapper adapts the existing broker's publish function
// so it can be used by the retry buffer.
// It bridges the pb.AxcpEnvelope type used by the broker
// and the axcp.Envelope type used by the retry buffer.
type BrokerWrapper struct {
	broker        *Broker
	convertLogger *log.Logger
}

// NewBrokerWrapper creates a new wrapper for the broker.
func NewBrokerWrapper(broker *Broker) *BrokerWrapper {
	return &BrokerWrapper{
		broker:        broker,
		convertLogger: log.New(log.Writer(), "[broker-wrapper] ", log.LstdFlags|log.Lshortfile),
	}
}

// PublishEnvelope publishes an axcp.Envelope, converting it to the format required by the broker.
func (w *BrokerWrapper) PublishEnvelope(env *axcp.Envelope) error {
	if env == nil {
		return fmt.Errorf("envelope is nil")
	}

	return w.broker.Publish(&env.AxcpEnvelope)
}

// PublishTelemetryWithEnvelope publishes a telemetry datagram carried in an envelope.
func (w *BrokerWrapper) PublishTelemetryWithEnvelope(env *axcp.Envelope) error {
	if env == nil {
		return fmt.Errorf("envelope is nil")
	}

	// Extract telemetry from the envelope
	telemetry := GetTelemetryFromEnvelope(env)
	if telemetry == nil {
		return fmt.Errorf("no telemetry data in envelope ID %s", env.GetTraceId())
	}

	w.convertLogger.Printf("Publishing telemetry from envelope with ID %s", env.GetTraceId())

	// Use the broker to publish the telemetry
	return w.broker.PublishTelemetry(telemetry, env.GetTraceId())
}

// GetTelemetryFromEnvelope extracts the telemetry datagram from an AXCP envelope
func GetTelemetryFromEnvelope(env *axcp.Envelope) *pb.TelemetryDatagram {
	if env == nil {
		return nil
	}

	return env.GetTelemetry()
}
