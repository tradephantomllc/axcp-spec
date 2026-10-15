package axcp

import (
	"crypto/ed25519"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// ServerConfig holds the configuration needed to validate incoming AXCP messages.
type ServerConfig struct {
	// LocalDID is this server's DID
	LocalDID string

	// PrivateKey is the server's Ed25519 private key for signing
	PrivateKey ed25519.PrivateKey

	// Resolver resolves DIDs to public keys
	Resolver auth.DIDResolver

	// Validation configuration
	Validation ValidationSettings

	// Security configuration
	Security SecuritySettings

	// Profile is the security profile to use (default: secure-baseline-v1)
	Profile string
}

// ValidationSettings holds validation-related configuration.
type ValidationSettings struct {
	// TimestampMaxAge is the maximum age of a message timestamp
	TimestampMaxAge time.Duration

	// TimestampClockSkew is the allowed clock skew for future timestamps
	TimestampClockSkew time.Duration

	// ReplayWindowDuration is how long idle peer replay state is kept. It is
	// raised automatically to at least TimestampMaxAge + TimestampClockSkew.
	// Zero selects auth.DefaultReplayPeerTTL.
	ReplayWindowDuration time.Duration

	// ReplayWindowSize is the sliding window size in sequence numbers per peer.
	// Zero selects auth.DefaultReplayWindow; negative values are rejected.
	ReplayWindowSize int

	// SkipSignatureVerification skips signature verification and replay
	// protection. Testing only: never enable it in a deployment.
	SkipSignatureVerification bool
}

// SecuritySettings holds security-related configuration.
type SecuritySettings struct {
	// AllowedDIDs restricts accepted senders to this list (empty = any sender
	// whose DID resolves through the configured resolver).
	AllowedDIDs []string
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Profile: ProfileSecureBaseline,
		Validation: ValidationSettings{
			TimestampMaxAge:      auth.DefaultMaxTimestampAge,
			TimestampClockSkew:   auth.DefaultClockSkew,
			ReplayWindowDuration: auth.DefaultReplayPeerTTL,
			ReplayWindowSize:     auth.DefaultReplayWindow,
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
func WithDID[T ServerConfig](did string) ConfigOption[T] {
	return func(c *T) {
		if cfg, ok := any(c).(*ServerConfig); ok {
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

// Validate checks that the ServerConfig is valid.
func (c *ServerConfig) Validate() error {
	if c.LocalDID == "" {
		return ErrInvalidContext.WithReason("LocalDID is required")
	}
	if _, err := auth.ParseDID(c.LocalDID); err != nil {
		return ErrInvalidContext.WithReason("LocalDID is not a valid DID")
	}
	if len(c.PrivateKey) == 0 {
		return ErrInvalidContext.WithReason("PrivateKey is required")
	}
	if len(c.PrivateKey) != ed25519.PrivateKeySize {
		return ErrInvalidContext.WithReason("PrivateKey must be an Ed25519 private key")
	}
	if c.Resolver == nil {
		return ErrInvalidContext.WithReason("Resolver is required")
	}
	if c.Validation.ReplayWindowSize < 0 {
		return ErrInvalidContext.WithReason("ReplayWindowSize must not be negative")
	}
	if c.Validation.ReplayWindowDuration < 0 {
		return ErrInvalidContext.WithReason("ReplayWindowDuration must not be negative")
	}
	if c.Validation.TimestampMaxAge < 0 || c.Validation.TimestampClockSkew < 0 {
		return ErrInvalidContext.WithReason("timestamp limits must not be negative")
	}
	for _, did := range c.Security.AllowedDIDs {
		if _, err := auth.ParseDID(did); err != nil {
			return ErrInvalidContext.WithReason("AllowedDIDs contains an invalid DID")
		}
	}
	return nil
}

// BuildValidator creates a Validator from the server config.
//
// Timestamp validation and replay protection are always enabled (unless
// SkipSignatureVerification is set for tests); zero values select defaults.
func (c *ServerConfig) BuildValidator() (*Validator, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	maxAge := c.Validation.TimestampMaxAge
	if maxAge == 0 {
		maxAge = auth.DefaultMaxTimestampAge
	}
	skew := c.Validation.TimestampClockSkew
	if skew == 0 && c.Validation.TimestampMaxAge == 0 {
		skew = auth.DefaultClockSkew
	}
	tsValidator := auth.NewTimestampValidator(maxAge, skew)

	ttl := c.Validation.ReplayWindowDuration
	if ttl == 0 {
		ttl = auth.DefaultReplayPeerTTL
	}
	window := uint64(c.Validation.ReplayWindowSize)
	if window == 0 {
		window = auth.DefaultReplayWindow
	}
	replayProtector, err := auth.NewReplayProtector(ttl, window, nil)
	if err != nil {
		return nil, ErrInvalidContext.WithReason("failed to create replay protector: " + err.Error())
	}

	return NewValidator(ValidatorConfig{
		LocalDID:                  c.LocalDID,
		Resolver:                  c.Resolver,
		TimestampValidator:        tsValidator,
		ReplayProtector:           replayProtector,
		AllowedDIDs:               c.Security.AllowedDIDs,
		SkipSignatureVerification: c.Validation.SkipSignatureVerification,
	}), nil
}
