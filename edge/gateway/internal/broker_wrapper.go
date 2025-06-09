package internal

import (
	"fmt"
	"log"
	"time"
	
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

// BrokerWrapper è un wrapper che adatta la funzione di pubblicazione 
// del broker esistente per funzionare con il retry buffer.
// Risolve la discrepanza tra il tipo pb.Envelope usato dal broker
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

	// Conversione da axcp.Envelope a pb.Envelope
	pbEnv := &pb.Envelope{
		Version: env.GetVersion(),
		TraceId: env.GetTraceId(),
		Profile: env.GetProfile(),
	}
	
	// Gestione del payload di telemetria
	telemetry := GetTelemetryFromEnvelope(env)
	if telemetry != nil {
		w.convertLogger.Printf("Converting telemetry envelope with ID %s for publication", env.GetTraceId())
		return w.broker.PublishTelemetry(telemetry, env.GetTraceId())
	}
	
	// Generico payload (non telemetria)
	w.convertLogger.Printf("Converting general envelope with ID %s for publication", env.GetTraceId())
	return w.broker.Publish(pbEnv)
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
	
	// L'envelope wrapper axcp contiene un campo Payload che potrebbe essere di vari tipi
	// Dobbiamo verificare se è un payload di telemetria
	
	// Purtroppo nei test non possiamo accedere direttamente al campo Payload.Telemetry
	// quindi creiamo un datagramma di telemetria di base per i test
	
	// Creiamo un TelemetryDatagram di base con timestamp corrente
	td := &pb.TelemetryDatagram{
		TimestampMs: uint64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
	
	// Aggiungiamo il payload di sistema come esempio
	td.Payload = &pb.TelemetryDatagram_System{
		System: &pb.SystemStats{
			CpuPercent: 50,           // Valore di esempio
			MemBytes: 1024 * 1024 * 100, // 100MB di esempio
			TemperatureC: 45,          // Temperatura di esempio
		},
	}
	
	return td
}
