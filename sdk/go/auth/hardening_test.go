package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseDID_StrictSyntax(t *testing.T) {
	valid := []string{
		"did:example:alice",
		"did:key:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd",
		"did:web:example.com:user:alice",
		"did:example:a%20b",
		"did:example1:a.b-c_d",
	}
	for _, did := range valid {
		if _, err := ParseDID(did); err != nil {
			t.Errorf("ParseDID(%q) = %v, want valid", did, err)
		}
	}

	invalid := map[string]string{
		"fragment":       "did:example:alice#key-1",
		"query":          "did:example:alice?service=x",
		"path":           "did:example:alice/path",
		"uppercase":      "did:Example:alice",
		"method symbol":  "did:ex!:alice",
		"trailing colon": "did:example:alice:",
		"bad pct":        "did:example:a%2",
		"bad pct hex":    "did:example:a%zz",
		"zero width":     "did:example:alice\u200b",
		"invalid utf8":   "did:example:\xff\xfe",
		"newline":        "did:example:al\nice",
		"space":          "did:example:al ice",
		"empty id":       "did:example:",
		"empty method":   "did::alice",
		"no prefix":      "example:alice",
	}
	for name, did := range invalid {
		if _, err := ParseDID(did); err == nil {
			t.Errorf("%s: ParseDID(%q) accepted", name, did)
		}
	}
}

func TestParseDID_LengthLimitIsCheap(t *testing.T) {
	huge := "did:key:z" + strings.Repeat("1", 10<<20)
	start := time.Now()
	if _, err := ParseDID(huge); !errors.Is(err, ErrDIDTooLong) {
		t.Fatalf("got %v, want ErrDIDTooLong", err)
	}
	if _, err := DecodeDIDKey(huge); !errors.Is(err, ErrDIDTooLong) {
		t.Fatalf("DecodeDIDKey: got %v, want ErrDIDTooLong", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("length check took %v", elapsed)
	}
	if _, err := ParseDID("did:example:" + strings.Repeat("a", MaxDIDLength)); !errors.Is(err, ErrDIDTooLong) {
		t.Fatalf("over-limit DID accepted: %v", err)
	}
}

func TestDIDKey_RoundTripAndVector(t *testing.T) {
	// Shared Secure Baseline vector identity (testdata/sdk/secure_baseline_vector.json).
	vectorKey, _ := base64.StdEncoding.DecodeString("A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg=")
	const vectorDID = "did:key:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd"

	got, err := EncodeDIDKey(vectorKey)
	if err != nil || got != vectorDID {
		t.Fatalf("EncodeDIDKey = %q, %v; want %q", got, err, vectorDID)
	}
	decoded, err := DecodeDIDKey(vectorDID)
	if err != nil || !decoded.Equal(ed25519.PublicKey(vectorKey)) {
		t.Fatalf("DecodeDIDKey = %x, %v", decoded, err)
	}

	for i := 0; i < 20; i++ {
		did, pub, _, err := GenerateIdentity()
		if err != nil {
			t.Fatal(err)
		}
		back, err := DecodeDIDKey(did)
		if err != nil || !back.Equal(pub) {
			t.Fatalf("round trip failed for %s: %v", did, err)
		}
	}
}

func TestDecodeDIDKey_Rejects(t *testing.T) {
	cases := map[string]string{
		"wrong method":    "did:example:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd",
		"no multibase":    "did:key:6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd",
		"too long id":     "did:key:z" + strings.Repeat("2", MaxDIDKeyIDLength),
		"bad base58":      "did:key:z0OIl",
		"wrong codec":     "did:key:z" + base58Encode(append([]byte{0xe7, 0x01}, make([]byte, 32)...)),
		"short key":       "did:key:z" + base58Encode(append([]byte{0xed, 0x01}, make([]byte, 31)...)),
		"small order":     "did:key:z" + base58Encode(append([]byte{0xed, 0x01}, smallOrderEncodings[1][:]...)),
		"label not key":   "did:key:sender",
		"empty multibase": "did:key:z",
	}
	for name, did := range cases {
		if _, err := DecodeDIDKey(did); err == nil {
			t.Errorf("%s: DecodeDIDKey(%q) accepted", name, did)
		}
	}
}

func TestSmallOrderKeysRejected(t *testing.T) {
	canonical := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000080",
		"0100000000000000000000000000000000000000000000000000000000000000",
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc85",
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa",
		"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
		// non-canonical encodings and sign variants
		"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
		"eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"0100000000000000000000000000000000000000000000000000000000000080",
	}
	for _, h := range canonical {
		key, _ := hex.DecodeString(h)
		if !IsSmallOrderPublicKey(key) {
			t.Errorf("%s not detected as small order", h)
		}
	}

	pub, _, _ := ed25519.GenerateKey(nil)
	if IsSmallOrderPublicKey(pub) {
		t.Fatal("random key detected as small order")
	}

	// The identity key with R = identity, S = 0 is a universal forgery that
	// crypto/ed25519 alone would accept; VerifyEd25519 must reject it.
	identity, _ := hex.DecodeString("0100000000000000000000000000000000000000000000000000000000000000")
	forged := append(append([]byte{}, identity...), make([]byte, 32)...)
	for _, msg := range []string{"", "any message", "AXCP-DID-AUTH-v1"} {
		if VerifyEd25519(identity, []byte(msg), forged) {
			t.Fatalf("forged signature accepted for %q", msg)
		}
	}
}

type staticResolver struct {
	doc   *DIDDocument
	err   error
	calls int
}

func (r *staticResolver) Resolve(context.Context, string) (*DIDDocument, error) {
	r.calls++
	return r.doc, r.err
}

func TestSelectEd25519Key_BindsDIDKeyToEncodedKey(t *testing.T) {
	did, pub, _, _ := GenerateIdentity()
	other, _, _ := ed25519.GenerateKey(nil)

	if _, err := SelectEd25519Key(did, documentForKey(did, other)); !errors.Is(err, ErrDIDMismatch) {
		t.Fatalf("resolver binding a did:key to another key: got %v, want ErrDIDMismatch", err)
	}
	key, err := SelectEd25519Key(did, documentForKey(did, pub))
	if err != nil || !key.Equal(pub) {
		t.Fatalf("matching key rejected: %v", err)
	}
	if _, err := SelectEd25519Key("did:key:sender", documentForKey("did:key:sender", pub)); err == nil {
		t.Fatal("non-standard did:key label accepted")
	}
	identity, _ := hex.DecodeString("0100000000000000000000000000000000000000000000000000000000000000")
	if _, err := SelectEd25519Key("did:example:x", documentForKey("did:example:x", identity)); !errors.Is(err, ErrSmallOrderKey) {
		t.Fatalf("small-order key: got %v", err)
	}
	if _, err := SelectEd25519Key("did:example:x", nil); !errors.Is(err, ErrDIDNotFound) {
		t.Fatalf("nil document: got %v", err)
	}
}

func TestDIDKeyResolver(t *testing.T) {
	did, pub, _, _ := GenerateIdentity()
	key, err := ResolveEd25519PublicKey(context.Background(), DIDKeyResolver{}, did)
	if err != nil || !key.Equal(pub) {
		t.Fatalf("ResolveEd25519PublicKey = %v", err)
	}
	if _, err := ResolveEd25519PublicKey(context.Background(), DIDKeyResolver{}, "did:example:x"); err == nil {
		t.Fatal("DIDKeyResolver resolved a non did:key")
	}
}

func TestCachingResolver_DoesNotCacheInvalidDocuments(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	const did = "did:example:victim"
	// The underlying resolver returns a document for a different DID.
	underlying := &staticResolver{doc: documentForKey("did:example:attacker", pub)}
	cache := NewCachingDIDResolver(underlying, time.Hour)

	for i := 0; i < 3; i++ {
		if _, err := ResolveEd25519PublicKey(context.Background(), cache, did); !errors.Is(err, ErrDIDMismatch) {
			t.Fatalf("call %d: got %v, want ErrDIDMismatch", i, err)
		}
	}

	// A key-agreement key listed first must never be returned as a signing key.
	sign, _, _ := ed25519.GenerateKey(nil)
	agreement := make([]byte, 32)
	agreement[0] = 9
	mixed := &staticResolver{doc: &DIDDocument{ID: did, PublicKeys: []PublicKeyRecord{
		{ID: "k1", Type: "X25519KeyAgreementKey2020", PublicKeyBytes: agreement},
		{ID: "k2", Type: "Ed25519VerificationKey2020", PublicKeyBytes: sign},
	}}}
	cache = NewCachingDIDResolver(mixed, time.Hour)
	for i := 0; i < 2; i++ {
		key, err := ResolveEd25519PublicKey(context.Background(), cache, did)
		if err != nil || !key.Equal(sign) {
			t.Fatalf("call %d returned %x, %v", i, key, err)
		}
	}
}

func TestCachingResolver_NegativeCaching(t *testing.T) {
	const did = "did:example:missing"
	underlying := &staticResolver{err: ErrDIDNotFound}
	cache := NewCachingDIDResolver(underlying, time.Hour)
	clockNow := time.Now()
	cache.now = func() time.Time { return clockNow }

	for i := 0; i < 3; i++ {
		if _, err := cache.Resolve(context.Background(), did); !errors.Is(err, ErrDIDNotFound) {
			t.Fatal(err)
		}
	}
	if underlying.calls != 1 {
		t.Fatalf("definitive failure resolved %d times, want 1 (cached)", underlying.calls)
	}

	// Negative entries are short-lived even with a long positive TTL.
	clockNow = clockNow.Add(DefaultNegativeCacheTTL + time.Second)
	_, _ = cache.Resolve(context.Background(), did)
	if underlying.calls != 2 {
		t.Fatalf("negative entry outlived its TTL (calls=%d)", underlying.calls)
	}
}

func TestCachingResolver_TransientErrorsNotCached(t *testing.T) {
	for _, transient := range []error{context.Canceled, context.DeadlineExceeded, errors.New("network down")} {
		underlying := &staticResolver{err: transient}
		cache := NewCachingDIDResolver(underlying, 0)
		_, _ = cache.Resolve(context.Background(), "did:example:x")
		_, _ = cache.Resolve(context.Background(), "did:example:x")
		if underlying.calls != 2 {
			t.Fatalf("%v was cached (calls=%d)", transient, underlying.calls)
		}
	}
}

func TestCachingResolver_BoundedAndStats(t *testing.T) {
	resolver := NewMemoryDIDResolver()
	cache := NewCachingDIDResolver(resolver, time.Hour)
	cache.SetMaxEntries(3)
	var dids []string
	for i := 0; i < 5; i++ {
		did, pub, _, _ := GenerateIdentity()
		resolver.AddDID(did, pub)
		dids = append(dids, did)
		if _, err := cache.Resolve(context.Background(), did); err != nil {
			t.Fatal(err)
		}
	}
	if size := cache.Stats().Size; size != 3 {
		t.Fatalf("cache size %d, want 3", size)
	}
	if _, err := cache.Resolve(context.Background(), dids[4]); err != nil {
		t.Fatal(err)
	}
	stats := cache.Stats()
	if stats.Hits != 1 || stats.Misses != 5 {
		t.Fatalf("stats = %+v, want 1 hit and 5 misses", stats)
	}
	if err := cache.PreloadDID("did:key:sender", make([]byte, 32)); err == nil {
		t.Fatal("PreloadDID accepted an invalid did:key binding")
	}
}
