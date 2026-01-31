package auth

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestCachingDIDResolver_BasicCaching(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	did := "did:key:test"

	// Create underlying resolver with one DID
	underlying := NewMemoryDIDResolver()
	underlying.AddDID(did, pub)

	// Create caching resolver
	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	ctx := context.Background()

	// First resolution should go to underlying resolver
	doc1, err := caching.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("First resolve failed: %v", err)
	}
	if doc1.ID != did {
		t.Errorf("doc1.ID = %q, want %q", doc1.ID, did)
	}

	// Remove from underlying resolver
	underlying.RemoveDID(did)

	// Second resolution should come from cache
	doc2, err := caching.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("Second resolve failed: %v", err)
	}
	if doc2.ID != did {
		t.Errorf("doc2.ID = %q, want %q", doc2.ID, did)
	}
}

func TestCachingDIDResolver_Expiration(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	did := "did:key:test"

	underlying := NewMemoryDIDResolver()
	underlying.AddDID(did, pub)

	// Create caching resolver with very short TTL
	caching := NewCachingDIDResolver(underlying, 10*time.Millisecond)

	ctx := context.Background()

	// First resolution
	_, err := caching.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("First resolve failed: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Remove from underlying - this should cause failure since cache expired
	underlying.RemoveDID(did)

	_, err = caching.Resolve(ctx, did)
	if err == nil {
		t.Error("Expected error after cache expiration and DID removal")
	}
}

func TestCachingDIDResolver_NoExpiration(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	did := "did:key:test"

	underlying := NewMemoryDIDResolver()
	underlying.AddDID(did, pub)

	// Create caching resolver with TTL = 0 (no expiration)
	caching := NewCachingDIDResolver(underlying, 0)

	ctx := context.Background()

	// First resolution
	_, err := caching.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("First resolve failed: %v", err)
	}

	// Remove from underlying
	underlying.RemoveDID(did)

	// Should still work from cache
	_, err = caching.Resolve(ctx, did)
	if err != nil {
		t.Errorf("Expected success from permanent cache: %v", err)
	}
}

func TestCachingDIDResolver_NegativeCaching(t *testing.T) {
	underlying := NewMemoryDIDResolver()

	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	ctx := context.Background()

	// First resolution should fail and cache the failure
	_, err1 := caching.Resolve(ctx, "did:key:unknown")
	if err1 == nil {
		t.Fatal("Expected error for unknown DID")
	}

	// Second resolution should also fail (from cache)
	_, err2 := caching.Resolve(ctx, "did:key:unknown")
	if err2 == nil {
		t.Fatal("Expected cached error for unknown DID")
	}
}

func TestCachingDIDResolver_Invalidate(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	did := "did:key:test"

	underlying := NewMemoryDIDResolver()
	underlying.AddDID(did, pub)

	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	ctx := context.Background()

	// First resolution
	_, err := caching.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("First resolve failed: %v", err)
	}

	// Invalidate the cache entry
	caching.Invalidate(did)

	// Remove from underlying
	underlying.RemoveDID(did)

	// Should fail now since cache was invalidated
	_, err = caching.Resolve(ctx, did)
	if err == nil {
		t.Error("Expected error after invalidation")
	}
}

func TestCachingDIDResolver_InvalidateAll(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	did1 := "did:key:test1"
	did2 := "did:key:test2"

	underlying := NewMemoryDIDResolver()
	underlying.AddDID(did1, pub1)
	underlying.AddDID(did2, pub2)

	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	ctx := context.Background()

	// Resolve both
	_, _ = caching.Resolve(ctx, did1)
	_, _ = caching.Resolve(ctx, did2)

	// Check stats
	stats := caching.Stats()
	if stats.Size != 2 {
		t.Errorf("Stats.Size = %d, want 2", stats.Size)
	}

	// Invalidate all
	caching.InvalidateAll()

	// Check stats again
	stats = caching.Stats()
	if stats.Size != 0 {
		t.Errorf("Stats.Size after InvalidateAll = %d, want 0", stats.Size)
	}
}

func TestCachingDIDResolver_Preload(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	did := "did:key:preloaded"

	// Create caching resolver with empty underlying
	underlying := NewMemoryDIDResolver()
	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	// Preload a DID
	caching.PreloadDID(did, pub)

	ctx := context.Background()

	// Should resolve from preloaded cache
	doc, err := caching.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("Resolve preloaded DID failed: %v", err)
	}
	if doc.ID != did {
		t.Errorf("doc.ID = %q, want %q", doc.ID, did)
	}
	if len(doc.PublicKeys) == 0 {
		t.Fatal("Expected public key in document")
	}
}

func TestCachingDIDResolver_Stats(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	did := "did:key:test"

	underlying := NewMemoryDIDResolver()
	underlying.AddDID(did, pub)

	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	// Initial stats
	stats := caching.Stats()
	if stats.Size != 0 {
		t.Errorf("Initial Size = %d, want 0", stats.Size)
	}

	// Resolve
	_, _ = caching.Resolve(context.Background(), did)

	// Check stats
	stats = caching.Stats()
	if stats.Size != 1 {
		t.Errorf("Size after resolve = %d, want 1", stats.Size)
	}
}

func TestCachingDIDResolver_ImplementsInterface(t *testing.T) {
	underlying := NewMemoryDIDResolver()
	caching := NewCachingDIDResolver(underlying, 5*time.Minute)

	// Verify it implements DIDResolver
	var _ DIDResolver = caching
}
