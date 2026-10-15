package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"math/big"
	"strings"
)

// did:key constants for Ed25519 keys.
const (
	// MaxDIDLength is the maximum accepted length of any DID in bytes.
	// The limit is checked before any other parsing work.
	MaxDIDLength = 512

	// MaxDIDKeyIDLength bounds the multibase identifier of a did:key before
	// base58 decoding, so decoding cost stays constant.
	MaxDIDKeyIDLength = 128

	didKeyMethod = "key"
)

// ed25519MulticodecPrefix is the unsigned-varint multicodec code 0xed (ed25519-pub).
var ed25519MulticodecPrefix = []byte{0xed, 0x01}

// did:key errors.
var (
	// ErrInvalidDIDKey indicates a did:key identifier that is not a valid Ed25519 did:key.
	ErrInvalidDIDKey = errors.New("auth: invalid Ed25519 did:key")

	// ErrSmallOrderKey indicates an Ed25519 public key of small order.
	ErrSmallOrderKey = errors.New("auth: Ed25519 public key has small order")
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58Index = func() [256]int8 {
	var idx [256]int8
	for i := range idx {
		idx[i] = -1
	}
	for i := 0; i < len(base58Alphabet); i++ {
		idx[base58Alphabet[i]] = int8(i)
	}
	return idx
}()

func base58Encode(data []byte) string {
	zeros := 0
	for zeros < len(data) && data[zeros] == 0 {
		zeros++
	}
	n := new(big.Int).SetBytes(data)
	radix := big.NewInt(58)
	mod := new(big.Int)
	var out []byte
	for n.Sign() > 0 {
		n.DivMod(n, radix, mod)
		out = append(out, base58Alphabet[mod.Int64()])
	}
	for i := 0; i < zeros; i++ {
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

// base58Decode decodes a base58btc string. Callers must bound the input length.
func base58Decode(s string) ([]byte, error) {
	n := new(big.Int)
	radix := big.NewInt(58)
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	for i := 0; i < len(s); i++ {
		d := base58Index[s[i]]
		if d < 0 {
			return nil, ErrInvalidDIDKey
		}
		n.Mul(n, radix)
		n.Add(n, big.NewInt(int64(d)))
	}
	decoded := n.Bytes()
	out := make([]byte, zeros+len(decoded))
	copy(out[zeros:], decoded)
	return out, nil
}

// EncodeDIDKey returns the did:key identifier for an Ed25519 public key.
func EncodeDIDKey(publicKey ed25519.PublicKey) (string, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return "", ErrInvalidKeyLength
	}
	if IsSmallOrderPublicKey(publicKey) {
		return "", ErrSmallOrderKey
	}
	data := append(append([]byte{}, ed25519MulticodecPrefix...), publicKey...)
	return "did:key:z" + base58Encode(data), nil
}

// DecodeDIDKey extracts the Ed25519 public key encoded in a did:key identifier.
//
// The identifier must use base58btc multibase ('z'), be at most
// MaxDIDKeyIDLength characters and decode to the ed25519-pub multicodec
// prefix followed by a 32-byte key that is not of small order.
func DecodeDIDKey(did string) (ed25519.PublicKey, error) {
	parsed, err := ParseDID(did)
	if err != nil {
		return nil, err
	}
	if parsed.Method() != didKeyMethod {
		return nil, ErrInvalidDIDKey
	}
	id := parsed.ID()
	if len(id) < 2 || len(id) > MaxDIDKeyIDLength || id[0] != 'z' {
		return nil, ErrInvalidDIDKey
	}
	raw, err := base58Decode(id[1:])
	if err != nil {
		return nil, err
	}
	if len(raw) != len(ed25519MulticodecPrefix)+ed25519.PublicKeySize || !bytes.HasPrefix(raw, ed25519MulticodecPrefix) {
		return nil, ErrInvalidDIDKey
	}
	key := ed25519.PublicKey(raw[len(ed25519MulticodecPrefix):])
	if IsSmallOrderPublicKey(key) {
		return nil, ErrSmallOrderKey
	}
	return key, nil
}

// IsDIDKey reports whether the string uses the did:key method.
func IsDIDKey(did string) bool {
	return strings.HasPrefix(did, "did:key:")
}

// DIDKeyResolver resolves self-certifying did:key identifiers (Ed25519 only).
//
// It accepts any well-formed Ed25519 did:key, so it authenticates senders but
// does not authorize them. Combine it with an allowlist (for example
// ValidatorConfig.AllowedDIDs) when only known peers may send messages.
type DIDKeyResolver struct{}

// Resolve implements DIDResolver for did:key identifiers.
func (DIDKeyResolver) Resolve(_ context.Context, did string) (*DIDDocument, error) {
	key, err := DecodeDIDKey(did)
	if err != nil {
		return nil, err
	}
	return &DIDDocument{
		ID: did,
		PublicKeys: []PublicKeyRecord{{
			ID:             did + "#" + did[len("did:key:"):],
			Type:           "Ed25519VerificationKey2020",
			PublicKeyBytes: key,
		}},
	}, nil
}

// smallOrderEncodings lists the Ed25519 point encodings (with the sign bit
// cleared) whose points have small order, including non-canonical encodings.
// It is the same blocklist libsodium uses.
var smallOrderEncodings = [7][32]byte{
	{0x00},
	{0x01},
	{0xec, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
	{0x26, 0xe8, 0x95, 0x8f, 0xc2, 0xb2, 0x27, 0xb0, 0x45, 0xc3, 0xf4, 0x89, 0xf2, 0xef, 0x98, 0xf0,
		0xd5, 0xdf, 0xac, 0x05, 0xd3, 0xc6, 0x33, 0x39, 0xb1, 0x38, 0x02, 0x88, 0x6d, 0x53, 0xfc, 0x05},
	{0xc7, 0x17, 0x6a, 0x70, 0x3d, 0x4d, 0xd8, 0x4f, 0xba, 0x3c, 0x0b, 0x76, 0x0d, 0x10, 0x67, 0x0f,
		0x2a, 0x20, 0x53, 0xfa, 0x2c, 0x39, 0xcc, 0xc6, 0x4e, 0xc7, 0xfd, 0x77, 0x92, 0xac, 0x03, 0x7a},
	{0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
	{0xee, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
}

// IsSmallOrderPublicKey reports whether an Ed25519 public key encodes a point
// of small order. Such keys allow trivially forged signatures and are rejected.
func IsSmallOrderPublicKey(publicKey []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	var masked [32]byte
	copy(masked[:], publicKey)
	masked[31] &= 0x7f
	for _, enc := range smallOrderEncodings {
		if masked == enc {
			return true
		}
	}
	return false
}
