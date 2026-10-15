"""DID and Ed25519 identity helpers for AXCP Core."""

from __future__ import annotations

import base64
import binascii
import dataclasses
from collections.abc import Mapping
from typing import Protocol

from nacl.exceptions import BadSignatureError, CryptoError
from nacl.signing import SigningKey, VerifyKey

from axcp.errors import (
    AXCPError,
    DIDResolutionError,
    InvalidDIDFormat,
    InvalidPrivateKeyError,
    KeyMismatchError,
    SignatureVerificationError,
)

ED25519_PUBLIC_KEY_SIZE = 32
ED25519_PRIVATE_SEED_SIZE = 32
ED25519_SIGNATURE_SIZE = 64
DID_KEY_ED25519_MULTICODEC_PREFIX = b"\xed\x01"
MAX_DID_LENGTH = 512
"""Maximum DID length in bytes (D1); checked before any other parsing."""
MAX_DID_KEY_ID_LENGTH = 128
"""Maximum did:key method-specific identifier length, checked before base58 decoding (D2)."""
MAX_BASE58_DECODE_LENGTH = 256
ALLOWED_VERIFICATION_KEY_TYPES = frozenset({"Ed25519VerificationKey2020", "Ed25519VerificationKey2018"})
BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
BASE58_INDEX = {char: index for index, char in enumerate(BASE58_ALPHABET)}

_DID_KEY_DECODED_LENGTH = len(DID_KEY_ED25519_MULTICODEC_PREFIX) + ED25519_PUBLIC_KEY_SIZE
_METHOD_CHARS = frozenset("abcdefghijklmnopqrstuvwxyz0123456789")
_ID_CHARS = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789.-_")
_HEX_CHARS = frozenset("0123456789abcdefABCDEF")

# Small-order Ed25519 points (little-endian encodings, sign bit cleared); identical to the
# libsodium blocklist (D9).
SMALL_ORDER_PUBLIC_KEYS = frozenset(
    bytes.fromhex(value)
    for value in (
        "0000000000000000000000000000000000000000000000000000000000000000",
        "0100000000000000000000000000000000000000000000000000000000000000",
        "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
        "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
        "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
        "edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
        "eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
    )
)


def _b58encode(data: bytes) -> str:
    value = int.from_bytes(data, "big")
    encoded = ""
    while value:
        value, rem = divmod(value, 58)
        encoded = BASE58_ALPHABET[rem] + encoded

    leading_zeroes = len(data) - len(data.lstrip(b"\x00"))
    return "1" * leading_zeroes + (encoded or "")


def _b58decode(text: str, max_length: int = MAX_BASE58_DECODE_LENGTH) -> bytes:
    # Decoding is quadratic in the input length: callers apply a smaller limit first.
    if len(text) > max_length:
        raise InvalidDIDFormat(f"base58btc input exceeds {max_length} characters")
    value = 0
    for char in text:
        digit = BASE58_INDEX.get(char)
        if digit is None:
            raise InvalidDIDFormat("invalid base58btc character")
        value = value * 58 + digit

    raw = value.to_bytes((value.bit_length() + 7) // 8, "big") if value else b""
    leading_zeroes = len(text) - len(text.lstrip("1"))
    return b"\x00" * leading_zeroes + raw


def _invalid_char_message(char: str) -> str:
    if char in "/?#":
        return "DID cannot contain a path, query or fragment"
    if ord(char) <= 0x20 or ord(char) == 0x7F or char.isspace():
        return "DID cannot contain whitespace or control characters"
    if ord(char) > 0x7F:
        return "DID must be ASCII"
    return "DID contains an invalid character"


@dataclasses.dataclass(frozen=True)
class DID:
    """Parsed decentralized identifier."""

    value: str
    method: str
    identifier: str

    @classmethod
    def parse(cls, value: str) -> "DID":
        """Parse a DID per the W3C DID Core ABNF (D1).

        ``did:<method>:<method-specific-id>`` with a lowercase/digit method, ASCII-only
        identifier characters (``ALPHA / DIGIT / . / - / _ / %XX``, ``:`` separators),
        no path/query/fragment and at most ``MAX_DID_LENGTH`` bytes (checked first).
        """

        if not isinstance(value, str):
            raise InvalidDIDFormat("DID must be a string")
        if not value:
            raise InvalidDIDFormat("DID cannot be empty")
        # len() counts code points; each encodes to >= 1 UTF-8 byte and non-ASCII input is
        # rejected below, so exceeding the limit in code points always exceeds it in bytes.
        if len(value) > MAX_DID_LENGTH:
            raise InvalidDIDFormat(f"DID exceeds {MAX_DID_LENGTH} bytes")
        if not value.startswith("did:"):
            raise InvalidDIDFormat("expected DID format did:<method>:<id>")

        index = 4
        length = len(value)
        while index < length and value[index] in _METHOD_CHARS:
            index += 1
        if index == 4:
            raise InvalidDIDFormat("DID method must be non-empty lowercase letters or digits")
        if index >= length or value[index] != ":":
            raise InvalidDIDFormat("DID method must be lowercase letters or digits followed by ':'")
        method = value[4:index]
        index += 1

        identifier_start = index
        if identifier_start == length:
            raise InvalidDIDFormat("DID identifier cannot be empty")
        segment_length = 0
        while index < length:
            char = value[index]
            if char == ":":
                segment_length = 0
                index += 1
                continue
            if char == "%":
                if (
                    index + 2 >= length
                    or value[index + 1] not in _HEX_CHARS
                    or value[index + 2] not in _HEX_CHARS
                ):
                    raise InvalidDIDFormat("DID contains an invalid percent-encoding")
                index += 3
                segment_length += 1
                continue
            if char not in _ID_CHARS:
                raise InvalidDIDFormat(_invalid_char_message(char))
            index += 1
            segment_length += 1
        if segment_length == 0:
            raise InvalidDIDFormat("DID identifier cannot end with ':'")
        return cls(value=value, method=method, identifier=value[identifier_start:])

    def __str__(self) -> str:
        return self.value


@dataclasses.dataclass(frozen=True)
class PublicKeyRecord:
    """A verification key entry in a minimal DID document."""

    id: str
    type: str
    public_key_bytes: bytes


@dataclasses.dataclass(frozen=True)
class DIDDocument:
    """Minimal DID document used by the SDK resolver interface."""

    id: str
    public_keys: tuple[PublicKeyRecord, ...]


class DIDResolver(Protocol):
    """Protocol implemented by DID resolvers.

    When a resolver is configured on a verifier it is consulted for every sender,
    including did:key, and therefore acts as an allowlist (D3).
    """

    def resolve(self, did: str) -> DIDDocument:
        """Resolve a DID to a minimal DID document."""


class InMemoryDIDResolver:
    """Deterministic in-memory DID resolver for tests and local examples."""

    def __init__(self, documents: Mapping[str, DIDDocument] | None = None) -> None:
        self._documents = dict(documents or {})

    def register(self, identity: "Identity", key_id: str = "key-1") -> None:
        self._documents[identity.did] = DIDDocument(
            id=identity.did,
            public_keys=(
                PublicKeyRecord(
                    id=key_id,
                    type="Ed25519VerificationKey2020",
                    public_key_bytes=identity.public_key,
                ),
            ),
        )

    def resolve(self, did: str) -> DIDDocument:
        try:
            return self._documents[did]
        except KeyError as exc:
            raise DIDResolutionError("DID not found") from exc


def is_small_order_public_key(public_key: bytes) -> bool:
    """Return True for small-order Ed25519 points (sign bit ignored, D9)."""

    if len(public_key) != ED25519_PUBLIC_KEY_SIZE:
        return False
    normalized = bytes(public_key[:31]) + bytes([public_key[31] & 0x7F])
    return normalized in SMALL_ORDER_PUBLIC_KEYS


def did_key_from_public_key(public_key: bytes) -> str:
    """Return a did:key identifier for an Ed25519 public key."""

    if len(public_key) != ED25519_PUBLIC_KEY_SIZE:
        raise InvalidDIDFormat("Ed25519 public key must be 32 bytes")
    multicodec = DID_KEY_ED25519_MULTICODEC_PREFIX + bytes(public_key)
    return "did:key:z" + _b58encode(multicodec)


def public_key_from_did_key(did: str) -> bytes:
    """Extract the Ed25519 public key from a did:key identifier (D1, D2, D9)."""

    parsed = DID.parse(did)
    if parsed.method != "key":
        raise InvalidDIDFormat("only did:key can be decoded without a resolver")
    if not parsed.identifier.startswith("z"):
        raise InvalidDIDFormat("did:key identifier must use base58btc multibase")
    if len(parsed.identifier) > MAX_DID_KEY_ID_LENGTH:
        raise InvalidDIDFormat(f"did:key identifier exceeds {MAX_DID_KEY_ID_LENGTH} characters")
    decoded = _b58decode(parsed.identifier[1:], MAX_DID_KEY_ID_LENGTH)
    if len(decoded) != _DID_KEY_DECODED_LENGTH:
        raise InvalidDIDFormat("did:key must decode to the Ed25519 multicodec prefix and a 32-byte key")
    if not decoded.startswith(DID_KEY_ED25519_MULTICODEC_PREFIX):
        raise InvalidDIDFormat("did:key is not an Ed25519 verification key")
    public_key = decoded[len(DID_KEY_ED25519_MULTICODEC_PREFIX) :]
    if is_small_order_public_key(public_key):
        raise InvalidDIDFormat("did:key encodes a small-order Ed25519 public key")
    return public_key


def _record_key(record: object) -> bytes | None:
    record_type = getattr(record, "type", None)
    key = getattr(record, "public_key_bytes", None)
    if record_type not in ALLOWED_VERIFICATION_KEY_TYPES:
        return None
    if not isinstance(key, (bytes, bytearray, memoryview)):
        return None
    key = bytes(key)
    if len(key) != ED25519_PUBLIC_KEY_SIZE:
        return None
    return key


def resolve_ed25519_public_key(did: str, resolver: DIDResolver | None = None) -> bytes:
    """Resolve the Ed25519 verification key for a DID (D3).

    With a resolver, the resolver is consulted for every DID including did:key; for
    did:key the resolved key must equal the key encoded in the DID. Without a resolver,
    did:key is decoded directly (open mode) and other methods are rejected.
    """

    parsed = DID.parse(did)
    did_key_public_key = public_key_from_did_key(did) if parsed.method == "key" else None
    if resolver is None:
        if did_key_public_key is not None:
            return did_key_public_key
        raise DIDResolutionError("resolver is required for non did:key identifiers")

    try:
        document = resolver.resolve(did)
    except AXCPError:
        raise
    except Exception as exc:  # resolver bugs or I/O errors must surface as SDK errors
        raise DIDResolutionError("DID resolution failed") from exc

    document_id = getattr(document, "id", None)
    public_keys = getattr(document, "public_keys", None)
    if not isinstance(document_id, str) or public_keys is None:
        raise DIDResolutionError("resolver returned an invalid DID document")
    if document_id != did:
        raise DIDResolutionError("resolved DID document ID does not match request")
    try:
        records = tuple(public_keys)
    except TypeError as exc:
        raise DIDResolutionError("resolver returned an invalid DID document") from exc

    for record in records:
        key = _record_key(record)
        if key is None:
            continue
        if is_small_order_public_key(key):
            raise DIDResolutionError("resolved Ed25519 public key has small order")
        if did_key_public_key is not None and key != did_key_public_key:
            raise KeyMismatchError("resolved key does not match the key encoded in the did:key")
        return key
    raise DIDResolutionError("no acceptable Ed25519 public key found")


@dataclasses.dataclass(frozen=True)
class Identity:
    """AXCP Ed25519 identity with a did:key identifier.

    The signing key is excluded from ``repr`` (D12); use :meth:`to_json` or
    :attr:`private_seed` only when the seed must be persisted.
    """

    _signing_key: SigningKey = dataclasses.field(repr=False)
    did: str

    @classmethod
    def generate(cls) -> "Identity":
        signing_key = SigningKey.generate()
        return cls.from_seed(bytes(signing_key.encode()))

    @classmethod
    def from_seed(cls, seed: bytes, did: str | None = None) -> "Identity":
        if not isinstance(seed, (bytes, bytearray)) or len(seed) != ED25519_PRIVATE_SEED_SIZE:
            raise InvalidPrivateKeyError("Ed25519 private seed must be 32 bytes")
        signing_key = SigningKey(bytes(seed))
        public_key = bytes(signing_key.verify_key.encode())
        if did is None:
            resolved_did = did_key_from_public_key(public_key)
        else:
            resolved_did = DID.parse(did).value
        if DID.parse(resolved_did).method == "key" and public_key_from_did_key(resolved_did) != public_key:
            raise KeyMismatchError("DID public key does not match private key")
        return cls(_signing_key=signing_key, did=resolved_did)

    @classmethod
    def from_json(cls, data: Mapping[str, str]) -> "Identity":
        seed_b64 = data.get("private_seed_b64")
        if not seed_b64:
            raise InvalidPrivateKeyError("private_seed_b64 is required")
        try:
            seed = base64.b64decode(seed_b64, validate=True)
        except (binascii.Error, ValueError, TypeError) as exc:
            raise InvalidPrivateKeyError("private_seed_b64 must be valid standard base64") from exc
        identity = cls.from_seed(seed, did=data.get("did"))
        declared = data.get("public_key_b64")
        if declared is not None:
            try:
                declared_key = base64.b64decode(declared, validate=True)
            except (binascii.Error, ValueError, TypeError) as exc:
                raise InvalidPrivateKeyError("public_key_b64 must be valid standard base64") from exc
            if declared_key != identity.public_key:
                raise KeyMismatchError("public_key_b64 does not match private_seed_b64")
        return identity

    @property
    def public_key(self) -> bytes:
        return bytes(self._signing_key.verify_key.encode())

    @property
    def private_seed(self) -> bytes:
        """The 32-byte private seed. Never logged or included in ``repr``."""

        return bytes(self._signing_key.encode())

    def sign(self, payload: bytes) -> bytes:
        return bytes(self._signing_key.sign(payload).signature)

    def verify(self, payload: bytes, signature: bytes) -> None:
        verify_signature(self.public_key, payload, signature)

    def to_json(self) -> dict[str, str]:
        """Explicit persistence format, INCLUDING the private seed."""

        return {
            "did": self.did,
            "public_key_b64": base64.b64encode(self.public_key).decode("ascii"),
            "private_seed_b64": base64.b64encode(self.private_seed).decode("ascii"),
        }


def verify_signature(public_key: bytes, payload: bytes, signature: bytes) -> None:
    if len(public_key) != ED25519_PUBLIC_KEY_SIZE:
        raise SignatureVerificationError("Ed25519 public key must be 32 bytes")
    if len(signature) != ED25519_SIGNATURE_SIZE:
        raise SignatureVerificationError("Ed25519 signature must be 64 bytes")
    # libsodium also rejects these, but the check is explicit for parity with Go/TS (D9).
    if is_small_order_public_key(public_key):
        raise SignatureVerificationError("Ed25519 public key has small order")
    try:
        VerifyKey(bytes(public_key)).verify(bytes(payload), bytes(signature))
    except (BadSignatureError, CryptoError, ValueError, TypeError) as exc:
        raise SignatureVerificationError("signature verification failed") from exc
