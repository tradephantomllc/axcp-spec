package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "crypto/rsa"
    "crypto/tls"
    "crypto/x509"
    "encoding/pem"
    "flag"
    "io/ioutil"
    "log"
    "math/big"
    "net/http"
    "time"

    "github.com/tradephantom/axcp-spec/enterprise/secure/telemetry"
)

func loadCAPool(path string) (*x509.CertPool, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(data) {
        return nil, err
    }
    return pool, nil
}

func main() {
    var (
        listenAddr   string
        piiSchema    string
        jwtSecretStr string
        tlsCAFile    string
    )
    flag.StringVar(&listenAddr, "listen", ":8443", "Address to bind")
    flag.StringVar(&piiSchema, "pii-schema", "", "Path to PII schema YAML")
    flag.StringVar(&jwtSecretStr, "jwt-secret", "", "JWT HMAC secret (optional)")
    flag.StringVar(&tlsCAFile, "tls-ca-file", "", "Path to CA file for client auth (optional)")
    flag.Parse()

    // Signer: generate random keypair each start (real deployment would load)
    _, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        log.Fatalf("failed to generate key: %v", err)
    }
    signer, _ := telemetry.NewSigner(priv)

    // PII filter
    var filter *telemetry.PIIFilter
    if piiSchema != "" {
        filter, err = telemetry.NewPIIFilterFromFile(piiSchema)
        if err != nil {
            log.Fatalf("failed to load PII schema: %v", err)
        }
    }

    // Auth
    var jwtSecret []byte
    if jwtSecretStr != "" {
        jwtSecret = []byte(jwtSecretStr)
    }
    var caPool *x509.CertPool
    if tlsCAFile != "" {
        caPool, err = loadCAPool(tlsCAFile)
        if err != nil {
            log.Fatalf("failed to load CA file: %v", err)
        }
    }

    h := telemetry.NewTelemetryHandler(signer, filter, jwtSecret, caPool)

    mux := http.NewServeMux()
    mux.Handle("/v1/secure/telemetry", h)

    srv := &http.Server{
        Addr:              listenAddr,
        Handler:           mux,
        ReadHeaderTimeout: 15 * time.Second,
    }

    if caPool != nil {
        srv.TLSConfig = &tls.Config{
            ClientAuth: tls.RequireAndVerifyClientCert,
            ClientCAs:  caPool,
            MinVersion: tls.VersionTLS13,
        }
    }

    log.Printf("Starting secure telemetry server on %s", listenAddr)
    if caPool != nil {
        // Need server cert; for demo, generate self-signed.
        certPEM, keyPEM, genErr := genSelfSigned()
        if genErr != nil {
            log.Fatalf("failed to generate cert: %v", genErr)
        }
        cert, _ := tls.X509KeyPair(certPEM, keyPEM)
        srv.TLSConfig.Certificates = []tls.Certificate{cert}
        log.Fatal(srv.ListenAndServeTLS("", ""))
    } else {
        log.Fatal(srv.ListenAndServe())
    }
}

// genSelfSigned creates an in-memory self-signed certificate for demo purposes.
func genSelfSigned() ([]byte, []byte, error) {
    priv, err := rsaGenerateKey()
    if err != nil {
        return nil, nil, err
    }
    template := x509.Certificate{
        SerialNumber:          big.NewInt(1),
        NotBefore:             time.Now(),
        NotAfter:              time.Now().Add(365 * 24 * time.Hour),
        KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
        ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
        BasicConstraintsValid: true,
    }
    der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
    if err != nil {
        return nil, nil, err
    }
    certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
    keyBytes := x509.MarshalPKCS1PrivateKey(priv)
    keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
    return certPEM, keyPEM, nil
}

// rsaGenerateKey generates 2048-bit RSA key.
func rsaGenerateKey() (*rsa.PrivateKey, error) {
    return rsa.GenerateKey(rand.Reader, 2048)
}
