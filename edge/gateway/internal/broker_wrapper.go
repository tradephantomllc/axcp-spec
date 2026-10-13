package internal

import (
	"fmt"
	"log"

	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
)

// BrokerWrapper è un wrapper che adatta la funzione di pubblicazione
// del broker esistente per funzionare con il retry buffer.
// Risolve la discrepanza tra il tipo pb.AxcpEnvelope usato dal broker
// e il tipo axcp.Envelope usato dal retry buffer.
type BrokerWrapper struct {
	broker        *Broker
	convertLogger *log.Logger
}

// NewBrokerWrapper crea un nuovo wrapper per il broker.
func NewBrokerWrapper(broker *Broker) *BrokerWrapper {
	return &BrokerWrapper{
		broker:        broker,
		convertLogger: log.New(log.Writer(), "[broker-wrapper] ", log.LstdFlags|log.Lshortfile),
	}
}

// PublishEnvelope pubblica un envelope axcp.Envelope convertendolo nel formato richiesto dal broker.
func (w *BrokerWrapper) PublishEnvelope(env *axcp.Envelope) error {
	if env == nil {
		return fmt.Errorf("envelope is nil")
	}

	return w.broker.Publish(&env.AxcpEnvelope)
}

// PublishTelemetryWithEnvelope pubblica un datagramma di telemetria usando un envelope.
func (w *BrokerWrapper) PublishTelemetryWithEnvelope(env *axcp.Envelope) error {
	if env == nil {
		return fmt.Errorf("envelope is nil")
	}

	// Estrai telemetria dall'envelope
	telemetry := GetTelemetryFromEnvelope(env)
	if telemetry == nil {
		return fmt.Errorf("no telemetry data in envelope ID %s", env.GetTraceId())
	}

	w.convertLogger.Printf("Publishing telemetry from envelope with ID %s", env.GetTraceId())

	// Usa il broker per pubblicare la telemetria
	return w.broker.PublishTelemetry(telemetry, env.GetTraceId())
}

// GetTelemetryFromEnvelope estrae il datagramma di telemetria da un envelope AXCP
func GetTelemetryFromEnvelope(env *axcp.Envelope) *pb.TelemetryDatagram {
	if env == nil {
		return nil
	}

	return env.GetTelemetry()
}
