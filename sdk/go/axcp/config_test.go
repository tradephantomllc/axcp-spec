package axcp

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q", config.Profile, ProfileSecureBaseline)
	}
	if config.Validation.ReplayWindowDuration < config.Validation.TimestampMaxAge+config.Validation.TimestampClockSkew {
		t.Errorf("ReplayWindowDuration %v must cover max age + skew", config.Validation.ReplayWindowDuration)
	}
	if config.Validation.TimestampMaxAge != 5*time.Minute {
		t.Errorf("Validation.TimestampMaxAge = %v, want %v", config.Validation.TimestampMaxAge, 5*time.Minute)
	}
	if config.Validation.ReplayWindowSize != 1000 {
		t.Errorf("Validation.ReplayWindowSize = %d, want %d", config.Validation.ReplayWindowSize, 1000)
	}
}

func TestServerConfig_Validate(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:test", pub)

	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ServerConfig{
				LocalDID:   "did:example:test",
				PrivateKey: priv,
				Resolver:   resolver,
			},
			wantErr: false,
		},
		{
			name: "missing LocalDID",
			config: ServerConfig{
				PrivateKey: priv,
				Resolver:   resolver,
			},
			wantErr: true,
		},
		{
			name: "missing PrivateKey",
			config: ServerConfig{
				LocalDID: "did:example:test",
				Resolver: resolver,
			},
			wantErr: true,
		},
		{
			name: "missing Resolver",
			config: ServerConfig{
				LocalDID:   "did:example:test",
				PrivateKey: priv,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


func TestServerConfig_BuildValidator(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:test", pub)

	config := ServerConfig{
		LocalDID:   "did:example:test",
		PrivateKey: priv,
		Resolver:   resolver,
		Validation: ValidationSettings{
			TimestampMaxAge:      5 * time.Minute,
			TimestampClockSkew:   30 * time.Second,
			ReplayWindowDuration: 5 * time.Minute,
			ReplayWindowSize:     100,
		},
	}

	validator, err := config.BuildValidator()
	if err != nil {
		t.Fatalf("BuildValidator() error: %v", err)
	}
	if validator == nil {
		t.Fatal("BuildValidator() returned nil validator")
	}
}

func TestServerConfig_BuildValidator_Invalid(t *testing.T) {
	config := ServerConfig{} // Invalid - missing required fields

	_, err := config.BuildValidator()
	if err == nil {
		t.Error("BuildValidator() should fail for invalid config")
	}
}

func TestNewServerConfig(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:server", pub)

	config := NewServerConfig(
		func(c *ServerConfig) {
			c.LocalDID = "did:example:server"
			c.PrivateKey = priv
			c.Resolver = resolver
		},
	)

	if config.LocalDID != "did:example:server" {
		t.Errorf("LocalDID = %q, want %q", config.LocalDID, "did:example:server")
	}
	if config.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q (from defaults)", config.Profile, ProfileSecureBaseline)
	}
}

func TestProfileConstants(t *testing.T) {
	// Ensure profile constants are defined correctly
	if ProfileSecureBaseline != "secure-baseline-v1" {
		t.Errorf("ProfileSecureBaseline = %q, want %q", ProfileSecureBaseline, "secure-baseline-v1")
	}
	if ProfileMinimal != "minimal-v1" {
		t.Errorf("ProfileMinimal = %q, want %q", ProfileMinimal, "minimal-v1")
	}
}

func TestServerConfig_BuildValidator_ZeroReplayDurationKeepsProtection(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:test", pub)

	// A zero replay duration selects the default; it never disables protection.
	config := ServerConfig{
		LocalDID:   "did:example:test",
		PrivateKey: priv,
		Resolver:   resolver,
		Validation: ValidationSettings{
			TimestampMaxAge:      5 * time.Minute,
			TimestampClockSkew:   30 * time.Second,
			ReplayWindowDuration: 0,
		},
	}

	validator, err := config.BuildValidator()
	if err != nil {
		t.Fatalf("BuildValidator() error: %v", err)
	}
	rp := validator.ReplayProtector()
	if rp == nil {
		t.Fatal("replay protection must stay enabled")
	}
	if rp.TTL() < 5*time.Minute+30*time.Second {
		t.Fatalf("replay TTL %v must cover max age + skew", rp.TTL())
	}
}

func TestServerConfig_BuildValidator_RaisesShortReplayTTL(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:test", pub)

	config := ServerConfig{
		LocalDID:   "did:example:test",
		PrivateKey: priv,
		Resolver:   resolver,
		Validation: ValidationSettings{
			TimestampMaxAge:      5 * time.Minute,
			TimestampClockSkew:   30 * time.Second,
			ReplayWindowDuration: time.Minute,
		},
	}
	validator, err := config.BuildValidator()
	if err != nil {
		t.Fatalf("BuildValidator() error: %v", err)
	}
	if got := validator.ReplayProtector().TTL(); got != 5*time.Minute+30*time.Second {
		t.Fatalf("replay TTL = %v, want max age + skew", got)
	}
}

func TestServerConfig_ValidateRejectsUnsafeValues(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:test", pub)
	base := ServerConfig{LocalDID: "did:example:test", PrivateKey: priv, Resolver: resolver}

	cases := map[string]func(*ServerConfig){
		"negative window":   func(c *ServerConfig) { c.Validation.ReplayWindowSize = -1 },
		"negative duration": func(c *ServerConfig) { c.Validation.ReplayWindowDuration = -time.Second },
		"invalid local DID": func(c *ServerConfig) { c.LocalDID = "not a did" },
		"invalid allowlist": func(c *ServerConfig) { c.Security.AllowedDIDs = []string{"did:example:ok", "bad"} },
		"short private key": func(c *ServerConfig) { c.PrivateKey = priv[:10] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() should fail")
			}
		})
	}
}

func TestServerConfig_BuildValidator_SkipSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:example:test", pub)

	config := ServerConfig{
		LocalDID:   "did:example:test",
		PrivateKey: priv,
		Resolver:   resolver,
		Validation: ValidationSettings{
			SkipSignatureVerification: true,
		},
	}

	validator, err := config.BuildValidator()
	if err != nil {
		t.Fatalf("BuildValidator() error: %v", err)
	}
	if validator == nil {
		t.Fatal("BuildValidator() returned nil validator")
	}
	// Can't easily test the skip behavior here, but the config should be passed through
}
