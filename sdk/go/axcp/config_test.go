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
	if config.Network.ListenAddr != ":61300" {
		t.Errorf("Network.ListenAddr = %q, want %q", config.Network.ListenAddr, ":61300")
	}
	if config.Validation.TimestampMaxAge != 5*time.Minute {
		t.Errorf("Validation.TimestampMaxAge = %v, want %v", config.Validation.TimestampMaxAge, 5*time.Minute)
	}
	if config.Validation.ReplayWindowSize != 1000 {
		t.Errorf("Validation.ReplayWindowSize = %d, want %d", config.Validation.ReplayWindowSize, 1000)
	}
}

func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	if config.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q", config.Profile, ProfileSecureBaseline)
	}
	if config.Connection.DialTimeout != 10*time.Second {
		t.Errorf("Connection.DialTimeout = %v, want %v", config.Connection.DialTimeout, 10*time.Second)
	}
	if config.Connection.MaxRetries != 3 {
		t.Errorf("Connection.MaxRetries = %d, want %d", config.Connection.MaxRetries, 3)
	}
}

func TestDefaultNetworkConfig(t *testing.T) {
	config := DefaultNetworkConfig()

	if config.ListenAddr != ":61300" {
		t.Errorf("ListenAddr = %q, want %q", config.ListenAddr, ":61300")
	}
	if config.MaxMessageSize != 16*1024*1024 {
		t.Errorf("MaxMessageSize = %d, want %d", config.MaxMessageSize, 16*1024*1024)
	}
}

func TestServerConfig_Validate(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:key:test", pub)

	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ServerConfig{
				LocalDID:   "did:key:test",
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
				LocalDID: "did:key:test",
				Resolver: resolver,
			},
			wantErr: true,
		},
		{
			name: "missing Resolver",
			config: ServerConfig{
				LocalDID:   "did:key:test",
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

func TestClientConfig_Validate(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)

	tests := []struct {
		name    string
		config  ClientConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ClientConfig{
				LocalDID:   "did:key:test",
				PrivateKey: priv,
				ServerAddr: "localhost:61300",
			},
			wantErr: false,
		},
		{
			name: "missing LocalDID",
			config: ClientConfig{
				PrivateKey: priv,
				ServerAddr: "localhost:61300",
			},
			wantErr: true,
		},
		{
			name: "missing PrivateKey",
			config: ClientConfig{
				LocalDID:   "did:key:test",
				ServerAddr: "localhost:61300",
			},
			wantErr: true,
		},
		{
			name: "missing ServerAddr",
			config: ClientConfig{
				LocalDID:   "did:key:test",
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
	resolver.AddDID("did:key:test", pub)

	config := ServerConfig{
		LocalDID:   "did:key:test",
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
	resolver.AddDID("did:key:server", pub)

	config := NewServerConfig(
		func(c *ServerConfig) {
			c.LocalDID = "did:key:server"
			c.PrivateKey = priv
			c.Resolver = resolver
		},
	)

	if config.LocalDID != "did:key:server" {
		t.Errorf("LocalDID = %q, want %q", config.LocalDID, "did:key:server")
	}
	if config.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q (from defaults)", config.Profile, ProfileSecureBaseline)
	}
}

func TestNewClientConfig(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)

	config := NewClientConfig(
		func(c *ClientConfig) {
			c.LocalDID = "did:key:client"
			c.PrivateKey = priv
			c.ServerAddr = "localhost:61300"
		},
	)

	if config.LocalDID != "did:key:client" {
		t.Errorf("LocalDID = %q, want %q", config.LocalDID, "did:key:client")
	}
	if config.ServerAddr != "localhost:61300" {
		t.Errorf("ServerAddr = %q, want %q", config.ServerAddr, "localhost:61300")
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

func TestServerConfig_BuildValidator_NoReplay(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:key:test", pub)

	// Config with replay protection disabled
	config := ServerConfig{
		LocalDID:   "did:key:test",
		PrivateKey: priv,
		Resolver:   resolver,
		Validation: ValidationSettings{
			TimestampMaxAge:      5 * time.Minute,
			ReplayWindowDuration: 0, // Disabled
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

func TestServerConfig_BuildValidator_SkipSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID("did:key:test", pub)

	config := ServerConfig{
		LocalDID:   "did:key:test",
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
