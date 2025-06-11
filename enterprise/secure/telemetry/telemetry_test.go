package telemetry

import (
    "crypto/rand"
    "crypto/ed25519"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "testing"

    jwt "github.com/golang-jwt/jwt/v5"
)

func TestSignerRoundtrip(t *testing.T) {
    signer, err := GenerateSigner()
    if err != nil {
        t.Fatalf("generate signer: %v", err)
    }
    msg := []byte("hello world")
    sig := signer.Sign(msg)
    if !signer.Verify(msg, sig) {
        t.Fatalf("verification failed")
    }
}

func TestPIIFilter(t *testing.T) {
    // create temp schema file
    schema := "fields:\n  - user.email\n  - device.id\n"
    f, err := os.CreateTemp("", "schema*.yaml")
    if err != nil {
        t.Fatalf("tempfile: %v", err)
    }
    defer os.Remove(f.Name())
    if _, err := f.WriteString(schema); err != nil {
        t.Fatalf("write: %v", err)
    }
    f.Close()

    filter, err := NewPIIFilterFromFile(f.Name())
    if err != nil {
        t.Fatalf("new filter: %v", err)
    }

    obj := map[string]any{
        "user": map[string]any{"email": "foo@bar.com"},
        "device": map[string]any{"id": "abc"},
        "other": "visible",
    }
    redacted := filter.RedactInPlace(obj)
    if redacted != 2 {
        t.Fatalf("expected 2 redacted, got %d", redacted)
    }

    if email := obj["user"].(map[string]any)["email"]; email != "REDACTED" {
        t.Fatalf("email not redacted: %v", email)
    }
}

func TestHandlerJWTAuth(t *testing.T) {
    // signer
    _, priv, _ := ed25519.GenerateKey(rand.Reader)
    signer, _ := NewSigner(priv)

    secret := []byte("mysecret")

    handler := NewTelemetryHandler(signer, nil, secret, nil)

    // craft JWT token
    token := jwt.New(jwt.SigningMethodHS256)
    tokenString, err := token.SignedString(secret)
    if err != nil {
        t.Fatalf("token sign: %v", err)
    }

    body := `{"temperature": 42}`
    req := httptest.NewRequest("POST", "/v1/secure/telemetry", strings.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+tokenString)
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusAccepted {
        t.Fatalf("expected 202, got %d", rec.Code)
    }

    if got := rec.Header().Get("Trailer"); got != "X-Signature" {
        t.Fatalf("missing trailer header, got %q", got)
    }
}
