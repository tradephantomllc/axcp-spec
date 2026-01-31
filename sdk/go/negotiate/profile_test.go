package negotiate

import (
	"testing"
)

// --- Basic Negotiation Tests ---

func TestNegotiate_Success_SecureBaseline(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := DefaultCapabilities()

	result, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	if result.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q", result.Profile, ProfileSecureBaseline)
	}

	if result.Version != ProtocolVersionV1 {
		t.Errorf("Version = %q, want %q", result.Version, ProtocolVersionV1)
	}

	if !result.Cap.RequireAuth {
		t.Error("RequireAuth should be true for SecureBaseline")
	}

	if !result.Cap.RequireReplay {
		t.Error("RequireReplay should be true for SecureBaseline")
	}

	if len(result.Cap.SigAlgos) != 1 || result.Cap.SigAlgos[0] != SigAlgoEd25519 {
		t.Errorf("SigAlgos = %v, want [%s]", result.Cap.SigAlgos, SigAlgoEd25519)
	}
}

func TestNegotiate_ClientMultipleProfiles_SelectsSecure(t *testing.T) {
	// Client supports both, but server only supports SecureBaseline
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileTransportOnly, ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := DefaultCapabilities()

	result, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	if result.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q", result.Profile, ProfileSecureBaseline)
	}
}

func TestNegotiate_BothSupportBoth_SelectsSecureFirst(t *testing.T) {
	// Even if client lists transport-only first, we should select secure-baseline
	// because it's security-first selection
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileTransportOnly, ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileTransportOnly, ProfileSecureBaseline}
	serverCap := DefaultCapabilities()

	result, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	// Security-first: should select SecureBaseline even though client listed transport-only first
	if result.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q (security-first)", result.Profile, ProfileSecureBaseline)
	}
}

// --- Determinism Tests ---

func TestNegotiate_Deterministic(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap: Capabilities{
			RequireAuth:   true,
			RequireReplay: true,
			SigAlgos:      []string{SigAlgoEd25519, "ed448"},
		},
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := Capabilities{
		RequireAuth:   true,
		RequireReplay: true,
		SigAlgos:      []string{"ed448", SigAlgoEd25519},
	}

	// Run negotiation multiple times
	var results []ServerHello
	for i := 0; i < 100; i++ {
		result, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
		if err != nil {
			t.Fatalf("Negotiate failed on iteration %d: %v", i, err)
		}
		results = append(results, result)
	}

	// All results must be identical
	first := results[0]
	for i, r := range results[1:] {
		if r.Profile != first.Profile {
			t.Errorf("iteration %d: Profile = %q, want %q", i+1, r.Profile, first.Profile)
		}
		if r.Version != first.Version {
			t.Errorf("iteration %d: Version = %q, want %q", i+1, r.Version, first.Version)
		}
		if len(r.Cap.SigAlgos) != len(first.Cap.SigAlgos) || r.Cap.SigAlgos[0] != first.Cap.SigAlgos[0] {
			t.Errorf("iteration %d: SigAlgos = %v, want %v", i+1, r.Cap.SigAlgos, first.Cap.SigAlgos)
		}
	}

	// Verify ed25519 is preferred
	if first.Cap.SigAlgos[0] != SigAlgoEd25519 {
		t.Errorf("SigAlgo = %q, want %q (preferred)", first.Cap.SigAlgos[0], SigAlgoEd25519)
	}
}

// --- No Overlap Tests ---

func TestNegotiate_NoOverlap(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileTransportOnly},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := DefaultCapabilities()

	_, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != ErrNoProfileOverlap {
		t.Errorf("expected ErrNoProfileOverlap, got %v", err)
	}
}

func TestNegotiate_NoOverlap_ServerEmpty(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{}
	serverCap := DefaultCapabilities()

	_, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != ErrNoProfileOverlap {
		t.Errorf("expected ErrNoProfileOverlap, got %v", err)
	}
}

// --- Deprecated Profile Tests ---

func TestNegotiate_DeprecatedProfile_Rejected(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileTransportOnly},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileTransportOnly}
	serverCap := DefaultCapabilities()

	_, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != ErrDeprecatedProfile {
		t.Errorf("expected ErrDeprecatedProfile, got %v", err)
	}
}

func TestNegotiate_DeprecatedProfile_AllowedWithOption(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileTransportOnly},
		Cap: Capabilities{
			RequireAuth:   false,
			RequireReplay: false,
			SigAlgos:      []string{SigAlgoEd25519},
		},
	}

	serverSupported := []Profile{ProfileTransportOnly}
	serverCap := DefaultCapabilities()

	opts := NegotiateOptions{AllowDeprecated: true}

	result, err := Negotiate(client, serverSupported, serverCap, opts)
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	if result.Profile != ProfileTransportOnly {
		t.Errorf("Profile = %q, want %q", result.Profile, ProfileTransportOnly)
	}

	// Transport-only doesn't require auth/replay
	if result.Cap.RequireAuth {
		t.Error("RequireAuth should be false for TransportOnly")
	}
}

// --- Validation Tests ---

func TestNegotiate_InvalidVersion(t *testing.T) {
	client := ClientHello{
		Version:  "2",
		Profiles: []Profile{ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	_, err := Negotiate(client, []Profile{ProfileSecureBaseline}, DefaultCapabilities(), DefaultNegotiateOptions())
	if err != ErrInvalidVersion {
		t.Errorf("expected ErrInvalidVersion, got %v", err)
	}
}

func TestNegotiate_EmptyVersion(t *testing.T) {
	client := ClientHello{
		Version:  "",
		Profiles: []Profile{ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	_, err := Negotiate(client, []Profile{ProfileSecureBaseline}, DefaultCapabilities(), DefaultNegotiateOptions())
	if err != ErrInvalidVersion {
		t.Errorf("expected ErrInvalidVersion, got %v", err)
	}
}

func TestNegotiate_EmptyProfiles(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{},
		Cap:      DefaultCapabilities(),
	}

	_, err := Negotiate(client, []Profile{ProfileSecureBaseline}, DefaultCapabilities(), DefaultNegotiateOptions())
	if err != ErrEmptyProfiles {
		t.Errorf("expected ErrEmptyProfiles, got %v", err)
	}
}

func TestNegotiate_UnknownProfile(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{"unknown-profile-v99"},
		Cap:      DefaultCapabilities(),
	}

	_, err := Negotiate(client, []Profile{ProfileSecureBaseline}, DefaultCapabilities(), DefaultNegotiateOptions())
	if err != ErrUnknownProfile {
		t.Errorf("expected ErrUnknownProfile, got %v", err)
	}
}

// --- Algorithm Negotiation Tests ---

func TestNegotiate_AlgoMismatch(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap: Capabilities{
			RequireAuth:   true,
			RequireReplay: true,
			SigAlgos:      []string{"rsa-sha256"},
		},
	}

	serverCap := Capabilities{
		RequireAuth:   true,
		RequireReplay: true,
		SigAlgos:      []string{SigAlgoEd25519},
	}

	_, err := Negotiate(client, []Profile{ProfileSecureBaseline}, serverCap, DefaultNegotiateOptions())
	if err != ErrNoAlgoOverlap {
		t.Errorf("expected ErrNoAlgoOverlap, got %v", err)
	}
}

func TestNegotiate_AlgoEmpty_DefaultsToEd25519(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap: Capabilities{
			RequireAuth:   true,
			RequireReplay: true,
			SigAlgos:      []string{}, // Empty
		},
	}

	serverCap := Capabilities{
		RequireAuth:   true,
		RequireReplay: true,
		SigAlgos:      []string{SigAlgoEd25519},
	}

	result, err := Negotiate(client, []Profile{ProfileSecureBaseline}, serverCap, DefaultNegotiateOptions())
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	if result.Cap.SigAlgos[0] != SigAlgoEd25519 {
		t.Errorf("SigAlgo = %q, want %q", result.Cap.SigAlgos[0], SigAlgoEd25519)
	}
}

func TestNegotiate_AlgoMultiple_PrefersEd25519(t *testing.T) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap: Capabilities{
			RequireAuth:   true,
			RequireReplay: true,
			SigAlgos:      []string{"ed448", SigAlgoEd25519, "p256"},
		},
	}

	serverCap := Capabilities{
		RequireAuth:   true,
		RequireReplay: true,
		SigAlgos:      []string{"p256", SigAlgoEd25519, "ed448"},
	}

	result, err := Negotiate(client, []Profile{ProfileSecureBaseline}, serverCap, DefaultNegotiateOptions())
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	// Should prefer ed25519 even though other algos have overlap
	if result.Cap.SigAlgos[0] != SigAlgoEd25519 {
		t.Errorf("SigAlgo = %q, want %q (preferred)", result.Cap.SigAlgos[0], SigAlgoEd25519)
	}
}

// --- Helper Function Tests ---

func TestIsSecureProfile(t *testing.T) {
	if !IsSecureProfile(ProfileSecureBaseline) {
		t.Error("SecureBaseline should be secure")
	}
	if IsSecureProfile(ProfileTransportOnly) {
		t.Error("TransportOnly should not be secure")
	}
}

func TestIsDeprecatedProfile(t *testing.T) {
	if IsDeprecatedProfile(ProfileSecureBaseline) {
		t.Error("SecureBaseline should not be deprecated")
	}
	if !IsDeprecatedProfile(ProfileTransportOnly) {
		t.Error("TransportOnly should be deprecated")
	}
}

func TestValidateProfile(t *testing.T) {
	if err := ValidateProfile(ProfileSecureBaseline); err != nil {
		t.Errorf("SecureBaseline should be valid: %v", err)
	}

	if err := ValidateProfile(ProfileTransportOnly); err != nil {
		t.Errorf("TransportOnly should be valid: %v", err)
	}

	if err := ValidateProfile("unknown"); err != ErrUnknownProfile {
		t.Errorf("Unknown profile should return ErrUnknownProfile, got %v", err)
	}
}

func TestDefaultCapabilities(t *testing.T) {
	cap := DefaultCapabilities()

	if !cap.RequireAuth {
		t.Error("Default RequireAuth should be true")
	}
	if !cap.RequireReplay {
		t.Error("Default RequireReplay should be true")
	}
	if len(cap.SigAlgos) != 1 || cap.SigAlgos[0] != SigAlgoEd25519 {
		t.Errorf("Default SigAlgos = %v, want [%s]", cap.SigAlgos, SigAlgoEd25519)
	}
}

// --- Edge Cases ---

func TestNegotiate_ClientPrefersDeprecated_ServerHasSecure(t *testing.T) {
	// Client prefers deprecated, but server only has secure
	// Should fail because no overlap (server doesn't support deprecated)
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileTransportOnly},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := DefaultCapabilities()

	_, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != ErrNoProfileOverlap {
		t.Errorf("expected ErrNoProfileOverlap, got %v", err)
	}
}

func TestNegotiate_DuplicateProfiles(t *testing.T) {
	// Client lists same profile twice - should still work
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline, ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := DefaultCapabilities()

	result, err := Negotiate(client, serverSupported, serverCap, DefaultNegotiateOptions())
	if err != nil {
		t.Fatalf("Negotiate failed: %v", err)
	}

	if result.Profile != ProfileSecureBaseline {
		t.Errorf("Profile = %q, want %q", result.Profile, ProfileSecureBaseline)
	}
}

// --- Benchmark ---

func BenchmarkNegotiate(b *testing.B) {
	client := ClientHello{
		Version:  ProtocolVersionV1,
		Profiles: []Profile{ProfileSecureBaseline},
		Cap:      DefaultCapabilities(),
	}

	serverSupported := []Profile{ProfileSecureBaseline}
	serverCap := DefaultCapabilities()
	opts := DefaultNegotiateOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Negotiate(client, serverSupported, serverCap, opts)
	}
}
