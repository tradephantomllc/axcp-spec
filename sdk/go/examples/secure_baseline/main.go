// Secure Baseline example demonstrates a complete AXCP secure-baseline-v1
// client-server exchange: did:key identities, Ed25519 signatures on every
// message, recipient binding, timestamp validation and replay protection in
// both directions.
//
// Run as server:  go run . -server
// Run as client:  go run . -addr localhost:61300 -server-did <DID printed by the server>
//
// The example uses netquic.InsecureTLSConfig, a self-signed TLS certificate
// without verification, so it must only run on localhost. Message
// authentication does not depend on TLS: every envelope is verified against
// the sender's did:key.
package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
)

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	addr := flag.String("addr", "localhost:61300", "Server address")
	serverDID := flag.String("server-did", "", "Server did:key (client mode)")
	flag.Parse()

	if *serverMode {
		if err := runServer(*addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
		return
	}
	if err := runClient(*addr, *serverDID); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}

// newSigner returns a signing session for messages from did to peers.
func newSigner(did string, key ed25519.PrivateKey) *auth.Session {
	session := auth.NewSession(auth.SessionConfig{LocalDID: did, PrivateKey: key})
	session.SetNegotiated(axcp.ProfileSecureBaseline, "")
	return session
}

// sign fills the auth fields of env, binding it to recipient.
func sign(session *auth.Session, env *axcp.Envelope, recipient string) error {
	env.RecipientDid = recipient
	out, err := session.SignEnvelope(context.Background(), auth.SignatureInput{
		RecipientDID: recipient,
		PayloadForSequence: func(sequence uint64) ([]byte, error) {
			env.Sequence = sequence
			return axcp.SigningPayload(env)
		},
	})
	if err != nil {
		return err
	}
	env.SenderDid = out.SenderDID
	env.TimestampMs = out.TimestampMs
	env.Sequence = out.Sequence
	env.Signature = out.Signature
	return nil
}

func newPatch(traceID, contextID string, version uint64) *axcp.Envelope {
	env := axcp.NewEnvelope(traceID, 1)
	env.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{ContextId: contextID, BaseVersion: version}}
	return env
}

func runServer(addr string) error {
	serverDID, _, serverKey, err := auth.GenerateIdentity()
	if err != nil {
		return err
	}
	log.Printf("Server DID: %s", serverDID)

	listener, err := netquic.Listen(netquic.DefaultListenerConfig(addr, netquic.InsecureTLSConfig()))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()
	log.Printf("AXCP Server listening on %s (secure-baseline-v1)", addr)

	// did:key identities are self-certifying: the resolver derives each
	// sender's key from its DID. Add axcp.WithAllowedDIDs to restrict senders.
	validator := axcp.NewValidator(axcp.ValidatorConfig{
		LocalDID: serverDID,
		Resolver: auth.DIDKeyResolver{},
	})
	signer := newSigner(serverDID, serverKey)

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			if errors.Is(err, netquic.ErrListenerClosed) {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				continue // no client within the accept timeout
			}
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConnection(conn, serverDID, signer, validator)
	}
}

func handleConnection(conn *netquic.ServerConnection, serverDID string, signer *auth.Session, validator *axcp.Validator) {
	defer conn.Close()
	log.Printf("New connection from %s", conn.RemoteAddr())

	for {
		env, err := conn.RecvEnvelope()
		if err != nil {
			log.Printf("Recv: %v", err)
			return
		}

		// Full Secure Baseline verification: structure, recipient, timestamp,
		// signature and, after the signature, replay protection.
		if perr := validator.ValidateAndVerify(context.Background(), env); perr != nil {
			log.Printf("Rejected TraceID=%s: %v", env.TraceId, perr)
			_ = conn.SendEnvelope(axcp.NewErrorEnvelope(env.TraceId, serverDID, env.SenderDid, perr))
			continue
		}
		log.Printf("Verified TraceID=%s From=%s Seq=%d", env.TraceId, env.SenderDid, env.Sequence)

		resp := newPatch(env.TraceId, env.GetContextPatch().GetContextId(), env.GetContextPatch().GetBaseVersion()+1)
		if err := sign(signer, resp, env.SenderDid); err != nil {
			log.Printf("Sign response: %v", err)
			return
		}
		if err := conn.SendEnvelope(resp); err != nil {
			log.Printf("Send: %v", err)
			return
		}
	}
}

func runClient(serverAddr, serverDID string) error {
	if _, err := auth.DecodeDIDKey(serverDID); err != nil {
		return fmt.Errorf("-server-did must be the did:key printed by the server: %w", err)
	}

	clientDID, _, clientKey, err := auth.GenerateIdentity()
	if err != nil {
		return err
	}
	log.Printf("Client DID: %s", clientDID)

	client, err := netquic.Dial(serverAddr, netquic.InsecureTLSConfig())
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()
	log.Printf("Connected to %s", serverAddr)

	signer := newSigner(clientDID, clientKey)
	// Responses are verified too, and only the expected server is accepted.
	validator := axcp.NewValidator(axcp.ValidatorConfig{
		LocalDID:    clientDID,
		Resolver:    auth.DIDKeyResolver{},
		AllowedDIDs: []string{serverDID},
	})

	for i := 1; i <= 3; i++ {
		traceID := uuid.NewString()
		env := newPatch(traceID, "demo", uint64(i))
		if err := sign(signer, env, serverDID); err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		log.Printf("[%d] Sending TraceID=%s Seq=%d", i, traceID[:8], env.Sequence)
		if err := client.SendEnvelope(env); err != nil {
			return fmt.Errorf("send: %w", err)
		}

		resp, err := client.RecvEnvelope()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		if axcp.IsErrorEnvelope(resp) {
			perr, _ := axcp.ErrorFromEnvelope(resp)
			// Error envelopes are unsigned and therefore only advisory.
			log.Printf("[%d] Server reported (unauthenticated): %v", i, perr)
			continue
		}
		if perr := validator.ValidateAndVerify(context.Background(), resp); perr != nil {
			return fmt.Errorf("response failed verification: %w", perr)
		}
		log.Printf("[%d] Verified response from %s (base_version=%d)", i, resp.SenderDid, resp.GetContextPatch().GetBaseVersion())
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("AXCP exchange complete")
	return nil
}
