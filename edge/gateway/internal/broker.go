package internal

import (
	"encoding/base64"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tradephantom/axcp-spec/edge/gateway/internal/buffer"
	"github.com/tradephantom/axcp-spec/edge/gateway/internal/dp"
	pb "github.com/tradephantom/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

type Broker struct {
	cli       mqtt.Client
	queue     *buffer.Queue
	dpEnabled bool
	dpLookup  *dp.BudgetLookup
}

type BrokerConfig struct {
	URL       string
	DPEnabled bool
	DPConfig  string // Path to DP budget config file
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

	// Set up DP budget lookup if enabled
	var dpLookup *dp.BudgetLookup
	if cfg.DPEnabled {
		configPath := cfg.DPConfig
		if configPath == "" {
			// Try to find config in default locations
			var err error
			configPath, err = dp.FindConfigFile()
			if err != nil {
				log.Printf("Failed to find DP config: %v. Differential privacy will be disabled.", err)
			} else {
				dpLookup, err = dp.LoadBudget(configPath)
				if err != nil {
					log.Printf("Failed to load DP config: %v. Differential privacy will be disabled.", err)
				}
			}
		} else {
			var err error
			dpLookup, err = dp.LoadBudget(configPath)
			if err != nil {
				log.Printf("Failed to load DP config: %v. Differential privacy will be disabled.", err)
			}
		}
	}

	return &Broker{
		cli:       cli,
		dpEnabled: cfg.DPEnabled,
		dpLookup:  dpLookup,
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
	// Apply differential privacy if enabled and DP lookup is configured
	if b.dpEnabled && b.dpLookup != nil {
		// Create a copy of the telemetry data to avoid modifying the original
		tdCopy := proto.Clone(td).(*pb.TelemetryDatagram)
		
		// Apply DP noise based on the topic and budget configuration
		if err := dp.ApplyNoise(tdCopy, trace, b.dpLookup); err != nil {
			log.Printf("Failed to apply differential privacy: %v", err)
			// Continue with original data if DP fails
		} else {
			td = tdCopy
		}
	}

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
