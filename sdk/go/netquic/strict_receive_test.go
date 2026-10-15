package netquic

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
)

// ambiguousEnvelope encodes an envelope carrying two different payload members,
// which protobuf libraries resolve differently and must therefore be rejected.
func ambiguousEnvelope(t *testing.T) []byte {
	t.Helper()
	raw, err := proto.Marshal(&pb.AxcpEnvelope{
		Version: 1,
		Payload: &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{ContextId: "ctx"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	errMsg, _ := proto.Marshal(&pb.ErrorMessage{Code: 1})
	return protowire.AppendBytes(protowire.AppendTag(raw, 7, protowire.BytesType), errMsg)
}

func TestRecvEnvelopeRejectsAmbiguousEnvelopes(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	l, err := Listen(DefaultListenerConfig("127.0.0.1:0", tlsConf))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	accepted := make(chan *ServerConnection, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := l.Accept(ctx)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			close(accepted)
			return
		}
		accepted <- conn
	}()

	client, err := Dial(l.Addr().String(), tlsConf)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	// Client -> server: the server must reject the frame.
	if err := writeFrame(client.stream, ambiguousEnvelope(t), binary.LittleEndian, true); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn := <-accepted
	if conn == nil {
		t.FailNow()
	}
	defer conn.Close()
	if env, err := conn.RecvEnvelope(); err == nil {
		t.Fatalf("server accepted ambiguous envelope: %v", env)
	}

	// Server -> client: the client must reject it too.
	if err := writeFrame(conn.stream, ambiguousEnvelope(t), binary.LittleEndian, true); err != nil {
		t.Fatalf("write: %v", err)
	}
	if env, err := client.RecvEnvelope(); err == nil {
		t.Fatalf("client accepted ambiguous envelope: %v", env)
	}
}
