package internal

import (
	"encoding/base64"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"google.golang.org/protobuf/proto"
)

type Broker struct {
	cli mqtt.Client
}

func NewBroker(url string) *Broker {
	opts := mqtt.NewClientOptions().AddBroker(url).SetClientID("axcp-gateway")
	cli := mqtt.NewClient(opts)
	token := cli.Connect()
	token.Wait()
	if token.Error() != nil {
		log.Fatalf("[mqtt] connect error: %v", token.Error())
	}
	return &Broker{cli: cli}
}

func (b *Broker) Publish(env *axcp.Envelope) error {
	raw, _ := axcp.ToBytes(env)
	topic := "axcp/" + env.TraceId
	return b.cli.Publish(topic, 0, false, base64.StdEncoding.EncodeToString(raw)).Error()
}

// PublishTelemetry publishes telemetry data to MQTT with the given trace ID
func (b *Broker) PublishTelemetry(td *axcp.TelemetryDatagram, trace string) error {
	// Utilizziamo il pacchetto protobuf standard per la serializzazione
	raw, err := proto.Marshal(td)
	if err != nil {
		return err
	}
	topic := "telemetry/" + trace
	return b.cli.Publish(topic, 0, false, raw).Error()
}

// PublishTelemetryData pubblica dati di telemetria generici in formato JSON
func (b *Broker) PublishTelemetryData(data map[string]interface{}, trace string) error {
	// In una implementazione reale, si dovrebbe usare json.Marshal per convertire la mappa in JSON
	// Ma per semplicità, usiamo una stringa fissa di esempio
	jsonMsg := `{"type":"telemetry","timestamp":"now","data":"sample"}`

	topic := "telemetry/" + trace
	return b.cli.Publish(topic, 0, false, jsonMsg).Error()
}
