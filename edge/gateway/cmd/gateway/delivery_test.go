package main

import (
	"errors"
	"github.com/tradephantomllc/axcp-spec/edge/gateway/internal"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
	"sync"
	"testing"
	"time"
)

type deliveryMock struct {
	mu        sync.Mutex
	fail      bool
	envelopes chan *pb.AxcpEnvelope
	telemetry chan *pb.TelemetryDatagram
	traces    chan string
}

func (m *deliveryMock) Publish(e *pb.AxcpEnvelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("broker unavailable")
	}
	m.envelopes <- proto.Clone(e).(*pb.AxcpEnvelope)
	return nil
}
func (m *deliveryMock) PublishTelemetry(d *pb.TelemetryDatagram, trace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("broker unavailable")
	}
	m.telemetry <- proto.Clone(d).(*pb.TelemetryDatagram)
	m.traces <- trace
	return nil
}

func TestDeliveryRetryPreservesContentAndRoute(t *testing.T) {
	for _, secure := range []bool{true, false} {
		t.Run(map[bool]string{true: "signed", false: "legacy"}[secure], func(t *testing.T) {
			m := &deliveryMock{fail: true, envelopes: make(chan *pb.AxcpEnvelope, 2), telemetry: make(chan *pb.TelemetryDatagram, 2), traces: make(chan string, 2)}
			p := gatewayPublisher{m}
			retries := internal.NewRetryBuffer(nil, nil, func(e *axcp.Envelope) error { return p.publish(&e.AxcpEnvelope) })
			defer retries.Close()
			env := &pb.AxcpEnvelope{Version: 1, TraceId: "retained-trace", Payload: &pb.AxcpEnvelope_Telemetry{Telemetry: &pb.TelemetryDatagram{TimestampMs: 123, Payload: &pb.TelemetryDatagram_System{System: &pb.SystemStats{CpuPercent: 37}}}}}
			if secure {
				env.Profile = 1
				env.Signature = []byte("original-signature")
				env.Sequence = 42
				env.SenderDid = "did:example:sender"
				env.RecipientDid = "did:example:receiver"
			}
			want := proto.Clone(env).(*pb.AxcpEnvelope)
			if err := publishOrQueue(p, retries, env); err != nil {
				t.Fatal(err)
			}
			env.TraceId = "caller-mutated"
			env.GetTelemetry().GetSystem().CpuPercent = 99
			m.mu.Lock()
			m.fail = false
			m.mu.Unlock()
			retries.Start()
			if secure {
				select {
				case got := <-m.envelopes:
					if !proto.Equal(got, want) {
						t.Fatal("retry changed signed envelope")
					}
				case <-time.After(time.Second):
					t.Fatal("retry not delivered")
				}
			} else {
				select {
				case got := <-m.telemetry:
					if !proto.Equal(got, want.GetTelemetry()) {
						t.Fatal("retry changed telemetry")
					}
				case <-time.After(time.Second):
					t.Fatal("telemetry retry not delivered")
				}
				if trace := <-m.traces; trace != want.TraceId {
					t.Fatal("retry changed topic")
				}
			}
		})
	}
}

func TestAuthenticationDefaultAndStartupValidation(t *testing.T) {
	if !defaultSecureBaseline {
		t.Fatal("authentication disabled by default")
	}
	for _, c := range []struct {
		did string
		n   int
	}{{"", 0}, {"invalid", 1}, {"did:example:gateway", 0}} {
		if err := validateSecuritySettings(true, c.did, c.n); err == nil {
			t.Fatal("invalid secure settings accepted")
		}
	}
	if err := validateSecuritySettings(true, "did:example:gateway", 1); err != nil {
		t.Fatal(err)
	}
	if err := validateSecuritySettings(false, "", 0); err != nil {
		t.Fatal("explicit transport-only rejected")
	}
}
