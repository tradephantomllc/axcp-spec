package internal

import (
	"errors"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"sync"
	"testing"
	"time"
)

type delayedToken struct {
	done chan struct{}
	err  error
}

func (t *delayedToken) Wait() bool { <-t.done; return true }
func (t *delayedToken) WaitTimeout(d time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(d):
		return false
	}
}
func (t *delayedToken) Done() <-chan struct{} { return t.done }
func (t *delayedToken) Error() error {
	select {
	case <-t.done:
		return t.err
	default:
		return nil
	}
}

type mockMQTT struct {
	mqtt.Client
	mu      sync.Mutex
	token   *delayedToken
	topic   string
	qos     byte
	payload interface{}
}

func (m *mockMQTT) Publish(topic string, qos byte, _ bool, payload interface{}) mqtt.Token {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.topic = topic
	m.qos = qos
	m.payload = payload
	return m.token
}

func TestBrokerWaitsForPublishAcknowledgement(t *testing.T) {
	for _, fail := range []bool{false, true} {
		token := &delayedToken{done: make(chan struct{})}
		if fail {
			token.err = errors.New("ack failed")
		}
		client := &mockMQTT{token: token}
		broker := &Broker{cli: client}
		done := make(chan error, 1)
		go func() { done <- broker.Publish(&pb.AxcpEnvelope{Version: 1, TraceId: "original"}) }()
		select {
		case err := <-done:
			t.Fatalf("returned before acknowledgement: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
		close(token.done)
		err := <-done
		if fail && !errors.Is(err, token.err) {
			t.Fatal(err)
		}
		if !fail && err != nil {
			t.Fatal(err)
		}
		client.mu.Lock()
		if client.qos != 1 || client.topic != "axcp/envelope" {
			t.Error("wrong publish contract")
		}
		client.mu.Unlock()
	}
}

func TestBrokerGenericTelemetryUsesInput(t *testing.T) {
	token := &delayedToken{done: make(chan struct{})}
	close(token.done)
	client := &mockMQTT{token: token}
	b := &Broker{cli: client}
	if err := b.PublishTelemetryData(map[string]interface{}{"cpu": 37}, "id"); err != nil {
		t.Fatal(err)
	}
	if string(client.payload.([]byte)) != "{\"cpu\":37}" || client.topic != "telemetry/id" {
		t.Fatal("telemetry was replaced")
	}
	if err := b.PublishTelemetryData(map[string]interface{}{"unsupported": make(chan int)}, "id"); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
