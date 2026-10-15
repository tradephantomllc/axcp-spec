package internal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

type Broker struct {
	cli mqtt.Client
}

type BrokerConfig struct {
	URL string
}

func NewBroker(cfg BrokerConfig) (*Broker, error) {
	// Set up MQTT client
	opts := mqtt.NewClientOptions().AddBroker(cfg.URL).SetClientID("axcp-gateway").SetConnectTimeout(10 * time.Second).SetWriteTimeout(10 * time.Second)
	cli := mqtt.NewClient(opts)
	token := cli.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		cli.Disconnect(0)
		return nil, errors.New("mqtt connect acknowledgement timed out")
	}
	if token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect error: %w", token.Error())
	}

	return &Broker{
		cli: cli,
	}, nil
}

func (b *Broker) Publish(env *pb.AxcpEnvelope) error {
	if env == nil {
		return errors.New("cannot publish nil envelope")
	}
	raw, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	// Use a generic topic because the envelope structure may have changed
	topic := "axcp/envelope"
	return b.publish(topic, base64.StdEncoding.EncodeToString(raw))
}

// PublishTelemetry publishes telemetry data to MQTT with the given trace ID
func (b *Broker) PublishTelemetry(td *pb.TelemetryDatagram, trace string) error {
	// Serialize and publish the telemetry data
	if td == nil {
		return errors.New("cannot publish nil telemetry")
	}
	raw, err := proto.Marshal(td)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry data: %w", err)
	}

	topic := "telemetry/" + trace
	return b.publish(topic, raw)
}

// PublishTelemetryData publishes generic telemetry data as JSON
func (b *Broker) PublishTelemetryData(data map[string]interface{}, trace string) error {
	if data == nil {
		return errors.New("cannot publish nil telemetry data")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode telemetry JSON: %w", err)
	}
	return b.publish("telemetry/"+trace, raw)
}

var ErrMQTTPublishTimeout = errors.New("mqtt publish acknowledgement timed out; delivery is uncertain")

func (b *Broker) publish(topic string, payload interface{}) error {
	if b == nil || b.cli == nil {
		return errors.New("mqtt client is not configured")
	}
	token := b.cli.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(10 * time.Second) {
		return ErrMQTTPublishTimeout
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt publish %s: %w", topic, err)
	}
	return nil
}

func (b *Broker) Close() {
	if b != nil && b.cli != nil {
		b.cli.Disconnect(250)
	}
}
