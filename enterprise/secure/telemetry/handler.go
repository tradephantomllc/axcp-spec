package telemetry

import (
    "crypto/x509"
    "encoding/json"
    "io"
    "net/http"
    "strings"

    jwt "github.com/golang-jwt/jwt/v5"
    "github.com/prometheus/client_golang/prometheus"
)

// TelemetryHandler handles the /v1/secure/telemetry endpoint.
// It supports authentication via JWT (Authorization: Bearer) or mutual TLS.
// The request body is expected to be a JSON telemetry datagram. The handler
// redacts PII fields according to the configured filter, signs the payload
// with Ed25519, appends the signature as trailer metadata, and returns 202.
//
// An implementation that stores or forwards the signed telemetry is left
// to an injected Sink function.
type TelemetryHandler struct {
    Signer *Signer
    Filter *PIIFilter

    // Auth configuration
    JWTSecret []byte // if nil, JWT auth is disabled
    TLSCA     *x509.CertPool // if nil, mTLS is disabled

    // Metrics
    metricSigned      prometheus.Counter
    metricPIIRedacted prometheus.Counter

    // Sink is invoked with the redacted payload and its signature.
    // Returning error will result in 500 to the client.
    Sink func(payload []byte, signature []byte) error
}

// NewTelemetryHandler constructs a handler and registers Prom metrics.
func NewTelemetryHandler(signer *Signer, filter *PIIFilter, jwtSecret []byte, tlsCA *x509.CertPool) *TelemetryHandler {
    metricSigned := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "enterprise_telemetry_signed_total",
        Help: "Total telemetry payloads successfully signed",
    })
    metricPIIRedacted := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "enterprise_pii_redacted_total",
        Help: "Total PII fields redacted across payloads",
    })
    prometheus.MustRegister(metricSigned, metricPIIRedacted)

    return &TelemetryHandler{
        Signer:            signer,
        Filter:            filter,
        JWTSecret:         jwtSecret,
        TLSCA:             tlsCA,
        metricSigned:      metricSigned,
        metricPIIRedacted: metricPIIRedacted,
        Sink: func(payload, sig []byte) error {
            // default sink: no-op
            return nil
        },
    }
}

func (h *TelemetryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Authenticate
    if !h.authenticate(r) {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Read payload
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // 3. PII redaction
    var obj map[string]any
    if err := json.Unmarshal(body, &obj); err != nil {
        http.Error(w, "invalid json", http.StatusBadRequest)
        return
    }
    redacted := 0
    if h.Filter != nil {
        redacted = h.Filter.RedactInPlace(obj)
        if redacted > 0 {
            h.metricPIIRedacted.Add(float64(redacted))
        }
    }
    payload, err := json.Marshal(obj)
    if err != nil {
        http.Error(w, "marshal error", http.StatusInternalServerError)
        return
    }

    // 4. Sign
    sig := h.Signer.Sign(payload)
    h.metricSigned.Inc()

    // 5. Sink
    if err := h.Sink(payload, sig); err != nil {
        http.Error(w, "sink error", http.StatusInternalServerError)
        return
    }

    // 6. Respond with signature in Trailer
    w.Header().Add("Trailer", "X-Signature")
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)
    if _, err := w.Write(payload); err == nil {
        w.Header().Set("X-Signature", encodeSig(sig))
    }
}

func encodeSig(sig []byte) string {
    const hex = "0123456789abcdef"
    sb := strings.Builder{}
    for _, b := range sig {
        sb.WriteByte(hex[b>>4])
        sb.WriteByte(hex[b&0x0f])
    }
    return sb.String()
}

// authenticate returns true if the request is authorized.
// Precedence: JWT header first, then mTLS client cert CN presence.
func (h *TelemetryHandler) authenticate(r *http.Request) bool {
    // JWT
    if h.JWTSecret != nil {
        auth := r.Header.Get("Authorization")
        if strings.HasPrefix(auth, "Bearer ") {
            tokenStr := strings.TrimPrefix(auth, "Bearer ")
            token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
                return h.JWTSecret, nil
            })
            if err == nil && token.Valid {
                return true
            }
        }
    }

    // mTLS
    if h.TLSCA != nil && r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
        // Verify chain against pool
        opts := x509.VerifyOptions{Roots: h.TLSCA}
        if _, err := r.TLS.PeerCertificates[0].Verify(opts); err == nil {
            return true
        }
    }

    return false
}
