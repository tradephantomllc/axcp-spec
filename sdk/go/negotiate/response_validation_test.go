package negotiate

import (
	"context"
	"crypto/ed25519"
	"errors"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"testing"
)

type fixedResponseStream struct{ response []byte }

func (s fixedResponseStream) SendMessage([]byte) error        { return nil }
func (s fixedResponseStream) ReceiveMessage() ([]byte, error) { return s.response, nil }

func TestClientRejectsSignedPolicyViolations(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:server", public)
	_, clientKey, _ := ed25519.GenerateKey(nil)
	cases := []struct {
		name   string
		mutate func(*WireServerHello)
		want   error
	}{
		{"valid", func(*WireServerHello) {}, nil},
		{"nonoffered", func(h *WireServerHello) { h.SelectedProfile = ProfileTransportOnly }, ErrProfileNotOffered},
		{"version", func(h *WireServerHello) { h.Version = 2 }, ErrInvalidVersion},
		{"stale", func(h *WireServerHello) { h.TimestampMs = 1 }, auth.ErrTimestampExpired},
		{"no auth", func(h *WireServerHello) { h.Capabilities.RequireAuth = false }, ErrInvalidCapabilities},
		{"no replay", func(h *WireServerHello) { h.Capabilities.RequireReplay = false }, ErrInvalidCapabilities},
		{"unsupported algorithm", func(h *WireServerHello) { h.Capabilities.SigAlgos = []string{"rsa"} }, ErrNoAlgoOverlap},
		{"multiple algorithms", func(h *WireServerHello) { h.Capabilities.SigAlgos = []string{SigAlgoEd25519, "rsa"} }, ErrInvalidCapabilities},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := &WireServerHello{Version: 1, SenderDID: "did:example:server", SelectedProfile: ProfileSecureBaseline, Capabilities: DefaultCapabilities(), TimestampMs: auth.NowMs()}
			tc.mutate(response)
			if err := response.Sign(private); err != nil {
				t.Fatal(err)
			}
			data, err := response.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			h := NewHandshaker(DefaultHandshakerConfig("did:example:client", clientKey, resolver))
			_, err = h.ClientHandshake(context.Background(), fixedResponseStream{data})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
	offer := &WireClientHello{Version: 1, Profiles: []Profile{ProfileTransportOnly}}
	response := &WireServerHello{Version: 1, SelectedProfile: ProfileTransportOnly, TimestampMs: auth.NowMs(), Capabilities: Capabilities{SigAlgos: []string{SigAlgoEd25519}}}
	if err := validateServerHello(offer, response, NegotiateOptions{}); !errors.Is(err, ErrDeprecatedProfile) {
		t.Fatal(err)
	}
	if err := validateServerHello(offer, response, NegotiateOptions{AllowDeprecated: true}); err != nil {
		t.Fatal(err)
	}
}

func TestHelloRejectsTruncationTrailingDataAndInvalidKeys(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(nil)
	hello := &WireServerHello{Version: 1, SenderDID: "did:example:server", SelectedProfile: ProfileSecureBaseline, Capabilities: DefaultCapabilities(), TimestampMs: auth.NowMs()}
	if err := hello.Sign(key); err != nil {
		t.Fatal(err)
	}
	data, _ := hello.Marshal()
	for n := 0; n < len(data); n++ {
		if _, err := UnmarshalServerHello(data[:n]); err == nil {
			t.Fatalf("accepted truncated length %d", n)
		}
	}
	if _, err := UnmarshalServerHello(append(data, 0)); err == nil {
		t.Fatal("accepted trailing byte")
	}
	if err := hello.Sign(nil); err == nil {
		t.Fatal("accepted invalid private key")
	}
	if err := hello.Verify(nil); !errors.Is(err, ErrSignatureVerificationFailed) {
		t.Fatal(err)
	}
	if _, err := resolveSignatureAlgorithm([]string{"rsa"}, nil); !errors.Is(err, ErrNoAlgoOverlap) {
		t.Fatal(err)
	}
}
