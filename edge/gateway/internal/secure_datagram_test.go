package internal

import (
	"context"
	"crypto/ed25519"
	"errors"
	"github.com/quic-go/quic-go"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func secureFixture(t *testing.T) (ServerConfig, func(uint64) *pb.AxcpEnvelope) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)
	verifier, err := NewEnvelopeAuthenticator(resolver, "did:example:gateway", DefaultAuthConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	config := ServerConfig{Profile: negotiate.ProfileSecureBaseline, Authenticator: verifier}
	makeEnv := func(seq uint64) *pb.AxcpEnvelope {
		env := &pb.AxcpEnvelope{Version: 1, Profile: 1, TraceId: "same-trace", SenderDid: "did:example:sender", RecipientDid: "did:example:gateway", TimestampMs: auth.NowMs(), Sequence: seq,
			Payload: &pb.AxcpEnvelope_Telemetry{Telemetry: &pb.TelemetryDatagram{TimestampMs: auth.NowMs(), Payload: &pb.TelemetryDatagram_System{System: &pb.SystemStats{CpuPercent: 12}}}}}
		payload, err := axcp.SigningPayloadFromProto(env)
		if err != nil {
			t.Fatal(err)
		}
		env.Signature = ed25519.Sign(priv, auth.BuildDIDAuthTranscript(env.SenderDid, env.RecipientDid, payload, time.UnixMilli(int64(env.TimestampMs))))
		return env
	}
	return config, makeEnv
}

func datagramBytes(t *testing.T, env *pb.AxcpEnvelope) []byte {
	t.Helper()
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{0xA1}, data...)
}

func TestDatagramAuthenticationAndCrossTransportReplay(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*pb.AxcpEnvelope)
	}{
		{"unsigned", func(e *pb.AxcpEnvelope) { e.Signature = nil }},
		{"missing recipient", func(e *pb.AxcpEnvelope) { e.RecipientDid = "" }},
		{"wrong recipient", func(e *pb.AxcpEnvelope) { e.RecipientDid = "did:example:other" }},
		{"sequence tamper", func(e *pb.AxcpEnvelope) { e.Sequence++ }},
		{"wrong profile", func(e *pb.AxcpEnvelope) { e.Profile = 0 }},
		{"wrong version", func(e *pb.AxcpEnvelope) { e.Version = 2 }},
		{"wrong payload", func(e *pb.AxcpEnvelope) { e.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{}} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, makeEnv := secureFixture(t)
			env := makeEnv(1)
			tc.mutate(env)
			called := false
			handler := func(*pb.AxcpEnvelope, *AuthResult) { called = true }
			if err := handleDatagram(datagramBytes(t, env), cfg, handler, nil); err == nil || called {
				t.Fatalf("accepted invalid packet: %v", err)
			}
			// Invalid packets must not burn the legitimate sequence.
			if err := handleDatagram(datagramBytes(t, makeEnv(1)), cfg, handler, nil); err != nil || !called {
				t.Fatalf("valid control rejected: %v", err)
			}
		})
	}
	for _, streamFirst := range []bool{true, false} {
		cfg, makeEnv := secureFixture(t)
		env := makeEnv(10)
		if streamFirst {
			if r := authenticateEnvelope(env, cfg); !r.Authenticated {
				t.Fatal(r.Error)
			}
			if err := handleDatagram(datagramBytes(t, env), cfg, nil, nil); err == nil {
				t.Fatal("stream replay accepted as datagram")
			}
		} else {
			if err := handleDatagram(datagramBytes(t, env), cfg, nil, nil); err != nil {
				t.Fatal(err)
			}
			if r := authenticateEnvelope(env, cfg); r.Authenticated {
				t.Fatal("datagram replay accepted as stream")
			}
		}
	}
	cfg, makeEnv := secureFixture(t)
	raw, _ := proto.Marshal(makeEnv(1).GetTelemetry())
	legacy := append([]byte{0xA0}, raw...)
	if err := handleDatagram(legacy, cfg, nil, func(*pb.TelemetryDatagram) { t.Error("unauthenticated handler invoked") }); !errors.Is(err, ErrAuthRequired) {
		t.Fatal(err)
	}
	cfg.Profile = negotiate.ProfileTransportOnly
	called := false
	if err := handleDatagram(legacy, cfg, nil, func(*pb.TelemetryDatagram) { called = true }); err != nil || !called {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, {0xA1, 0xff}, {0xA0}, make([]byte, 1201)} {
		if err := handleDatagram(data, cfg, nil, nil); err == nil {
			t.Fatal("malformed accepted")
		}
	}
}

func TestQUICStreamAndDatagramDelivery(t *testing.T) {
	cfg, makeEnv := secureFixture(t)
	tlsConf := netquic.InsecureTLSConfig()
	listener, err := quic.ListenAddr("127.0.0.1:0", tlsConf, &quic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	received := make(chan *pb.AxcpEnvelope, 4)
	done := make(chan error, 1)
	go func() {
		done <- serveQUIC(ctx, listener, cfg, func(env *pb.AxcpEnvelope, _ *AuthResult) { received <- env }, nil)
	}()
	client, err := netquic.Dial(listener.Addr().String(), tlsConf)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	env := makeEnv(1)
	if err := client.SendTelemetryEnvelope(axcp.CloneEnvelope(env)); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if !proto.Equal(got, env) {
			t.Fatal("datagram changed")
		}
	case <-ctx.Done():
		t.Fatal("no datagram delivered")
	}
	env = makeEnv(2)
	if err := client.SendEnvelope(axcp.CloneEnvelope(env)); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if !proto.Equal(got, env) {
			t.Fatal("stream changed")
		}
	case <-ctx.Done():
		t.Fatal("no stream delivered")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not complete")
	}
}
