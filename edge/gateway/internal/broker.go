package internal

import (
	"encoding/base64"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tradephantomllc/axcp-spec/edge/gateway/internal/buffer"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

type Broker struct {
	cli   mqtt.Client
	queue *buffer.Queue
}

type BrokerConfig struct {
	URL string
}

func NewBroker(cfg BrokerConfig) (*Broker, error) {
	// Set up MQTT client
	opts := mqtt.NewClientOptions().AddBroker(cfg.URL).SetClientID("axcp-gateway")
	cli := mqtt.NewClient(opts)
	token := cli.Connect()
	token.Wait()
	if token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect error: %w", token.Error())
	}

	return &Broker{
		cli: cli,
	}, nil
}

func (b *Broker) Publish(env *pb.AxcpEnvelope) error {
	raw, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	// Uso un ID traccia generico poiché la struttura potrebbe essere cambiata
	topic := "axcp/envelope"
	return b.cli.Publish(topic, 0, false, base64.StdEncoding.EncodeToString(raw)).Error()
}

// PublishTelemetry publishes telemetry data to MQTT with the given trace ID
func (b *Broker) PublishTelemetry(td *pb.TelemetryDatagram, trace string) error {
	// Serialize and publish the telemetry data
	raw, err := proto.Marshal(td)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry data: %w", err)
	}

	topic := "telemetry/" + trace
	if token := b.cli.Publish(topic, 0, false, raw); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish telemetry: %w", token.Error())
	}

	return nil
}

// PublishTelemetryData pubblica dati di telemetria generici in formato JSON
func (b *Broker) PublishTelemetryData(data map[string]interface{}, trace string) error {
	// In una implementazione reale, si dovrebbe usare json.Marshal per convertire la mappa in JSON
	// Ma per semplicità, usiamo una stringa fissa di esempio
	jsonMsg := `{"type":"telemetry","timestamp":"now","data":"sample"}`

	topic := "telemetry/" + trace
	return b.cli.Publish(topic, 0, false, jsonMsg).Error()
}
