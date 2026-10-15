package auth

import (
	"container/list"
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"time"
)

// DefaultCacheTTL is the default cache time-to-live.
const DefaultCacheTTL = 5 * time.Minute

// DefaultNegativeCacheTTL is the default lifetime of cached definitive
// resolution failures.
const DefaultNegativeCacheTTL = 30 * time.Second

// DefaultCacheMaxEntries is the default maximum number of cached DIDs.
const DefaultCacheMaxEntries = 10000

// CacheEntry represents a cached DID resolution result.
type CacheEntry struct {
	PublicKey ed25519.PublicKey
	ExpiresAt time.Time
	Error     error
}

type cacheItem struct {
	did   string
	entry CacheEntry
}

// CachingDIDResolver wraps a DIDResolver with a bounded in-memory cache.
//
// Only validated results are cached: the resolved document must pass the same
// checks as ResolveEd25519PublicKey (document ID, key type, key length,
// small-order and did:key binding). Definitive failures are cached for a short
// negative TTL; transient failures such as context cancellation or timeouts are
// never cached. The least recently used entry is evicted when the cache is full.
type CachingDIDResolver struct {
	resolver    DIDResolver
	ttl         time.Duration
	negativeTTL time.Duration
	maxEntries  int

	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List
	hits    int64
	misses  int64
	now     func() time.Time
}

// NewCachingDIDResolver creates a new caching resolver.
// The ttl parameter specifies how long successful entries are cached;
// a ttl of 0 means successful entries never expire (they can still be evicted).
func NewCachingDIDResolver(resolver DIDResolver, ttl time.Duration) *CachingDIDResolver {
	negativeTTL := DefaultNegativeCacheTTL
	if ttl > 0 && ttl < negativeTTL {
		negativeTTL = ttl
	}
	return &CachingDIDResolver{
		resolver:    resolver,
		ttl:         ttl,
		negativeTTL: negativeTTL,
		maxEntries:  DefaultCacheMaxEntries,
		entries:     make(map[string]*list.Element),
		lru:         list.New(),
		now:         time.Now,
	}
}

// SetMaxEntries sets the maximum number of cached DIDs (<= 0 restores the default).
func (c *CachingDIDResolver) SetMaxEntries(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n <= 0 {
		n = DefaultCacheMaxEntries
	}
	c.maxEntries = n
	c.evictLocked()
}

// Resolve resolves a DID to a DID Document, using cache when available.
// Implements the DIDResolver interface.
func (c *CachingDIDResolver) Resolve(ctx context.Context, did string) (*DIDDocument, error) {
	if entry, ok := c.lookup(did); ok {
		if entry.Error != nil {
			return nil, entry.Error
		}
		return documentForKey(did, entry.PublicKey), nil
	}

	if c.resolver == nil {
		return nil, ErrDIDResolverMissing
	}
	doc, err := c.resolver.Resolve(ctx, did)
	if err == nil {
		var key ed25519.PublicKey
		key, err = SelectEd25519Key(did, doc)
		if err == nil {
			c.store(did, CacheEntry{PublicKey: key}, c.ttl)
			return doc, nil
		}
	}
	if isDefinitiveResolutionError(err) {
		c.store(did, CacheEntry{Error: err}, c.negativeTTL)
	}
	return nil, err
}

// isDefinitiveResolutionError reports whether a resolution failure depends only
// on the DID or its document and can therefore be cached briefly.
func isDefinitiveResolutionError(err error) bool {
	for _, definitive := range []error{
		ErrDIDNotFound, ErrDIDMismatch, ErrNoAcceptableKey, ErrSmallOrderKey,
		ErrInvalidDIDKey, ErrInvalidDIDFormat, ErrDIDMethodEmpty, ErrDIDIDEmpty,
		ErrDIDContainsInvalid, ErrDIDTooLong,
	} {
		if errors.Is(err, definitive) {
			return true
		}
	}
	return false
}

func documentForKey(did string, key ed25519.PublicKey) *DIDDocument {
	return &DIDDocument{
		ID: did,
		PublicKeys: []PublicKeyRecord{{
			ID:             did + "#key-1",
			Type:           "Ed25519VerificationKey2020",
			PublicKeyBytes: append([]byte(nil), key...),
		}},
	}
}

func (c *CachingDIDResolver) lookup(did string) (CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.entries[did]
	if !ok {
		c.misses++
		return CacheEntry{}, false
	}
	item := elem.Value.(*cacheItem)
	if !item.entry.ExpiresAt.IsZero() && !c.now().Before(item.entry.ExpiresAt) {
		c.lru.Remove(elem)
		delete(c.entries, did)
		c.misses++
		return CacheEntry{}, false
	}
	c.lru.MoveToFront(elem)
	c.hits++
	return item.entry, true
}

func (c *CachingDIDResolver) store(did string, entry CacheEntry, ttl time.Duration) {
	if ttl > 0 {
		entry.ExpiresAt = c.now().Add(ttl)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.entries[did]; ok {
		elem.Value.(*cacheItem).entry = entry
		c.lru.MoveToFront(elem)
		return
	}
	c.entries[did] = c.lru.PushFront(&cacheItem{did: did, entry: entry})
	c.evictLocked()
}

func (c *CachingDIDResolver) evictLocked() {
	for c.lru.Len() > c.maxEntries {
		oldest := c.lru.Back()
		c.lru.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheItem).did)
	}
}

// Invalidate removes a DID from the cache.
func (c *CachingDIDResolver) Invalidate(did string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.entries[did]; ok {
		c.lru.Remove(elem)
		delete(c.entries, did)
	}
}

// InvalidateAll clears the entire cache.
func (c *CachingDIDResolver) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*list.Element)
	c.lru.Init()
}

// CacheStats returns cache statistics.
type CacheStats struct {
	Size   int
	Hits   int64
	Misses int64
}

// Stats returns cache statistics.
func (c *CachingDIDResolver) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CacheStats{Size: c.lru.Len(), Hits: c.hits, Misses: c.misses}
}

// PreloadDID adds a DID to the cache without resolving it.
// Useful for bootstrapping known DIDs. The DID and key are validated with the
// same rules as resolved documents.
func (c *CachingDIDResolver) PreloadDID(did string, publicKey ed25519.PublicKey) error {
	if _, err := ParseDID(did); err != nil {
		return err
	}
	key, err := SelectEd25519Key(did, documentForKey(did, publicKey))
	if err != nil {
		return err
	}
	c.store(did, CacheEntry{PublicKey: key}, c.ttl)
	return nil
}
