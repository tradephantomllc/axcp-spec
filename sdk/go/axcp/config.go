package axcp

import (
	"crypto/ed25519"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// ServerConfig holds all configuration for an AXCP server.
type ServerConfig struct {
	// LocalDID is this server's DID
	LocalDID string

	// PrivateKey is the server's Ed25519 private key for signing
	PrivateKey ed25519.PrivateKey

	// Resolver resolves DIDs to public keys
	Resolver auth.DIDResolver

	// Network configuration
	Network NetworkConfig

	// Validation configuration
	Validation ValidationSettings

	// Security configuration
	Security SecuritySettings

	// Profile is the security profile to use (default: secure-baseline-v1)
	Profile string
}

// NetworkConfig holds network-related configuration.
type NetworkConfig struct {
	// ListenAddr is the address to listen on (default: ":61300")
	ListenAddr string

	// MaxConnections is the maximum number of concurrent connections (0 = unlimited)
	MaxConnections int

	// ConnectionTimeout is the timeout for accepting connections
	ConnectionTimeout time.Duration

	// IdleTimeout is the timeout for idle connections
	IdleTimeout time.Duration

	// MaxMessageSize is the maximum message size in bytes (default: 16MB)
	MaxMessageSize int
}

// ValidationSettings holds validation-related configuration.
type ValidationSettings struct {
	// TimestampMaxAge is the maximum age of a message timestamp
	TimestampMaxAge time.Duration

	// TimestampClockSkew is the allowed clock skew for future timestamps
	TimestampClockSkew time.Duration

	// ReplayWindowDuration is the duration of the replay protection window
	ReplayWindowDuration time.Duration

	// ReplayWindowSize is the maximum number of sequence numbers to track per peer
	ReplayWindowSize int

	// SkipSignatureVerification skips signature verification (testing only)
	SkipSignatureVerification bool
}

// SecuritySettings holds security-related configuration.
type SecuritySettings struct {
	// RequireClientCert requires client TLS certificates
	RequireClientCert bool

	// AllowedDIDs is a whitelist of allowed sender DIDs (empty = allow all)
	AllowedDIDs []string

	// RateLimitPerPeer is the maximum requests per second per peer (0 = unlimited)
	RateLimitPerPeer int
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Profile: ProfileSecureBaseline,
		Network: DefaultNetworkConfig(),
		Validation: ValidationSettings{
			TimestampMaxAge:      5 * time.Minute,
			TimestampClockSkew:   30 * time.Second,
			ReplayWindowDuration: 5 * time.Minute,
			ReplayWindowSize:     1000,
		},
	}
}

// DefaultNetworkConfig returns a NetworkConfig with sensible defaults.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		ListenAddr:        ":61300",
		MaxConnections:    0, // unlimited
		ConnectionTimeout: 30 * time.Second,
		IdleTimeout:       5 * time.Minute,
		MaxMessageSize:    16 * 1024 * 1024, // 16MB
	}
}

// ClientConfig holds all configuration for an AXCP client.
type ClientConfig struct {
	// LocalDID is this client's DID
	LocalDID string

	// PrivateKey is the client's Ed25519 private key for signing
	PrivateKey ed25519.PrivateKey

	// Resolver resolves DIDs to public keys
	Resolver auth.DIDResolver

	// ServerDID is the expected server DID (for mutual authentication)
	ServerDID string

	// ServerAddr is the server address to connect to
	ServerAddr string

	// Connection configuration
	Connection ConnectionSettings

	// Profile is the security profile to use (default: secure-baseline-v1)
	Profile string

	// Capabilities to advertise during handshake
	Capabilities []string
}

// ConnectionSettings holds connection-related configuration.
type ConnectionSettings struct {
	// DialTimeout is the timeout for establishing a connection
	DialTimeout time.Duration

	// KeepAliveInterval is the interval for keep-alive messages
	KeepAliveInterval time.Duration

	// MaxRetries is the maximum number of connection retries
	MaxRetries int

	// RetryDelay is the delay between retries
	RetryDelay time.Duration
}

// DefaultClientConfig returns a ClientConfig with sensible defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Profile: ProfileSecureBaseline,
		Connection: ConnectionSettings{
			DialTimeout:       10 * time.Second,
			KeepAliveInterval: 30 * time.Second,
			MaxRetries:        3,
			RetryDelay:        1 * time.Second,
		},
	}
}

// Security profile constants.
const (
	// ProfileSecureBaseline is the secure-baseline-v1 profile.
	// Requires: Ed25519 signatures, timestamp validation, replay protection.
	ProfileSecureBaseline = "secure-baseline-v1"

	// ProfileMinimal is the minimal profile (no security features).
	// WARNING: Only use for testing or trusted networks.
	ProfileMinimal = "minimal-v1"
)

// ConfigOption is a function that modifies a config.
type ConfigOption[T any] func(*T)

// WithDID sets the local DID.
func WithDID[T ServerConfig | ClientConfig](did string) ConfigOption[T] {
	return func(c *T) {
		switch cfg := any(c).(type) {
		case *ServerConfig:
			cfg.LocalDID = did
		case *ClientConfig:
			cfg.LocalDID = did
		}
	}
}

// NewServerConfig creates a ServerConfig with the given options.
func NewServerConfig(opts ...ConfigOption[ServerConfig]) ServerConfig {
	config := DefaultServerConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return config
}

// NewClientConfig creates a ClientConfig with the given options.
func NewClientConfig(opts ...ConfigOption[ClientConfig]) ClientConfig {
	config := DefaultClientConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return config
}

// Validate checks that the ServerConfig is valid.
func (c *ServerConfig) Validate() error {
	if c.LocalDID == "" {
		return ErrInvalidContext.WithReason("LocalDID is required")
	}
	if len(c.PrivateKey) == 0 {
		return ErrInvalidContext.WithReason("PrivateKey is required")
	}
	if c.Resolver == nil {
		return ErrInvalidContext.WithReason("Resolver is required")
	}
	return nil
}

// Validate checks that the ClientConfig is valid.
func (c *ClientConfig) Validate() error {
	if c.LocalDID == "" {
		return ErrInvalidContext.WithReason("LocalDID is required")
	}
	if len(c.PrivateKey) == 0 {
		return ErrInvalidContext.WithReason("PrivateKey is required")
	}
	if c.ServerAddr == "" {
		return ErrInvalidContext.WithReason("ServerAddr is required")
	}
	return nil
}

// BuildValidator creates a Validator from the server config.
func (c *ServerConfig) BuildValidator() (*Validator, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Create timestamp validator
	var tsValidator *auth.TimestampValidator
	if c.Validation.TimestampMaxAge > 0 {
		tsValidator = auth.NewTimestampValidator(
			c.Validation.TimestampMaxAge,
			c.Validation.TimestampClockSkew,
		)
	}

	// Create replay protector
	var replayProtector *auth.ReplayProtector
	if c.Validation.ReplayWindowDuration > 0 {
		var err error
		replayProtector, err = auth.NewReplayProtector(
			c.Validation.ReplayWindowDuration,
			uint64(c.Validation.ReplayWindowSize),
			nil,
		)
		if err != nil {
			return nil, ErrInvalidContext.WithReason("failed to create replay protector: " + err.Error())
		}
	}

	return NewValidator(ValidatorConfig{
		LocalDID:                  c.LocalDID,
		Resolver:                  c.Resolver,
		TimestampValidator:        tsValidator,
		ReplayProtector:           replayProtector,
		SkipSignatureVerification: c.Validation.SkipSignatureVerification,
	}), nil
}
