package auth

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"
)

// CacheEntry represents a cached DID resolution result.
type CacheEntry struct {
	PublicKey ed25519.PublicKey
	ExpiresAt time.Time
	Error     error
}

// CachingDIDResolver wraps a DIDResolver with an in-memory cache.
type CachingDIDResolver struct {
	resolver DIDResolver
	cache    sync.Map // map[string]CacheEntry
	ttl      time.Duration
	mu       sync.Mutex
}

// NewCachingDIDResolver creates a new caching resolver.
// The ttl parameter specifies how long entries are cached.
// A ttl of 0 means entries never expire.
func NewCachingDIDResolver(resolver DIDResolver, ttl time.Duration) *CachingDIDResolver {
	return &CachingDIDResolver{
		resolver: resolver,
		ttl:      ttl,
	}
}

// Resolve resolves a DID to a DID Document, using cache when available.
// Implements the DIDResolver interface.
func (c *CachingDIDResolver) Resolve(ctx context.Context, did string) (*DIDDocument, error) {
	// Check cache first
	if entry, ok := c.cache.Load(did); ok {
		ce := entry.(CacheEntry)
		if c.ttl == 0 || time.Now().Before(ce.ExpiresAt) {
			if ce.Error != nil {
				return nil, ce.Error
			}
			// Reconstruct DIDDocument from cached public key
			return &DIDDocument{
				ID: did,
				PublicKeys: []PublicKeyRecord{
					{
						ID:             did + "#key-1",
						Type:           "Ed25519VerificationKey2020",
						PublicKeyBytes: ce.PublicKey,
					},
				},
			}, nil
		}
		// Entry expired, delete it
		c.cache.Delete(did)
	}

	// Resolve from underlying resolver
	doc, err := c.resolver.Resolve(ctx, did)
	if err != nil {
		// Cache negative results too (briefly)
		c.cacheResult(did, nil, err)
		return nil, err
	}

	// Cache the public key
	if len(doc.PublicKeys) > 0 {
		c.cacheResult(did, doc.PublicKeys[0].PublicKeyBytes, nil)
	}

	return doc, nil
}

// cacheResult stores a resolution result in the cache.
func (c *CachingDIDResolver) cacheResult(did string, publicKey ed25519.PublicKey, err error) {
	entry := CacheEntry{
		PublicKey: publicKey,
		Error:     err,
	}
	if c.ttl > 0 {
		entry.ExpiresAt = time.Now().Add(c.ttl)
	}
	c.cache.Store(did, entry)
}

// Invalidate removes a DID from the cache.
func (c *CachingDIDResolver) Invalidate(did string) {
	c.cache.Delete(did)
}

// InvalidateAll clears the entire cache.
func (c *CachingDIDResolver) InvalidateAll() {
	c.cache.Range(func(key, _ any) bool {
		c.cache.Delete(key)
		return true
	})
}

// CacheStats returns cache statistics.
type CacheStats struct {
	Size    int
	Hits    int64
	Misses  int64
}

// Stats returns cache statistics.
// Note: This is a simple implementation that doesn't track hits/misses.
func (c *CachingDIDResolver) Stats() CacheStats {
	var size int
	c.cache.Range(func(_, _ any) bool {
		size++
		return true
	})
	return CacheStats{Size: size}
}

// PreloadDID adds a DID to the cache without resolving it.
// Useful for bootstrapping known DIDs.
func (c *CachingDIDResolver) PreloadDID(did string, publicKey ed25519.PublicKey) {
	c.cacheResult(did, publicKey, nil)
}

// DefaultCacheTTL is the default cache time-to-live.
const DefaultCacheTTL = 5 * time.Minute
