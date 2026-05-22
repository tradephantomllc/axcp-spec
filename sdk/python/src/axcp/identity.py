"""DID and Ed25519 identity helpers for AXCP Core."""

from __future__ import annotations

import base64
import dataclasses
import re
from collections.abc import Mapping
from typing import Protocol

from nacl.exceptions import BadSignatureError
from nacl.signing import SigningKey, VerifyKey

from axcp.errors import (
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
BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
BASE58_INDEX = {char: index for index, char in enumerate(BASE58_ALPHABET)}
_DID_RE = re.compile(r"^did:([^:\s]+):(.+)$")


def _b58encode(data: bytes) -> str:
    value = int.from_bytes(data, "big")
    encoded = ""
    while value:
        value, rem = divmod(value, 58)
        encoded = BASE58_ALPHABET[rem] + encoded

    leading_zeroes = len(data) - len(data.lstrip(b"\x00"))
    return "1" * leading_zeroes + (encoded or "")


def _b58decode(text: str) -> bytes:
    value = 0
    for char in text:
        if char not in BASE58_INDEX:
            raise InvalidDIDFormat(f"invalid base58btc character: {char!r}")
        value = value * 58 + BASE58_INDEX[char]

    raw = value.to_bytes((value.bit_length() + 7) // 8, "big") if value else b""
    leading_zeroes = len(text) - len(text.lstrip("1"))
    return b"\x00" * leading_zeroes + raw


@dataclasses.dataclass(frozen=True)
class DID:
    """Parsed decentralized identifier."""

    value: str
    method: str
    identifier: str

    @classmethod
    def parse(cls, value: str) -> "DID":
        if not value:
            raise InvalidDIDFormat("DID cannot be empty")
        if any(char.isspace() or ord(char) < 0x20 for char in value):
            raise InvalidDIDFormat("DID cannot contain whitespace or control characters")
        match = _DID_RE.match(value)
        if not match:
            raise InvalidDIDFormat("expected DID format did:<method>:<id>")
        method, identifier = match.groups()
        if not method:
            raise InvalidDIDFormat("DID method cannot be empty")
        if not identifier:
            raise InvalidDIDFormat("DID identifier cannot be empty")
        return cls(value=value, method=method, identifier=identifier)

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
    """Protocol implemented by DID resolvers."""

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
            raise DIDResolutionError(f"DID not found: {did}") from exc


def did_key_from_public_key(public_key: bytes) -> str:
    """Return a did:key identifier for an Ed25519 public key."""

    if len(public_key) != ED25519_PUBLIC_KEY_SIZE:
        raise InvalidDIDFormat("Ed25519 public key must be 32 bytes")
    multicodec = DID_KEY_ED25519_MULTICODEC_PREFIX + public_key
    return "did:key:z" + _b58encode(multicodec)


def public_key_from_did_key(did: str) -> bytes:
    """Extract the Ed25519 public key from a did:key identifier."""

    parsed = DID.parse(did)
    if parsed.method != "key":
        raise InvalidDIDFormat("only did:key can be decoded without a resolver")
    if not parsed.identifier.startswith("z"):
        raise InvalidDIDFormat("did:key identifier must use base58btc multibase")
    decoded = _b58decode(parsed.identifier[1:])
    if not decoded.startswith(DID_KEY_ED25519_MULTICODEC_PREFIX):
        raise InvalidDIDFormat("did:key is not an Ed25519 verification key")
    public_key = decoded[len(DID_KEY_ED25519_MULTICODEC_PREFIX) :]
    if len(public_key) != ED25519_PUBLIC_KEY_SIZE:
        raise InvalidDIDFormat("did:key Ed25519 public key must be 32 bytes")
    return public_key


def resolve_ed25519_public_key(did: str, resolver: DIDResolver | None = None) -> bytes:
    """Resolve an Ed25519 public key from did:key or a configured resolver."""

    parsed = DID.parse(did)
    if parsed.method == "key":
        return public_key_from_did_key(did)
    if resolver is None:
        raise DIDResolutionError("resolver is required for non did:key identifiers")

    document = resolver.resolve(did)
    if document.id != did:
        raise DIDResolutionError("resolved DID document ID does not match request")
    for record in document.public_keys:
        if record.type not in {"Ed25519VerificationKey2020", "Ed25519VerificationKey2018"}:
            continue
        if len(record.public_key_bytes) == ED25519_PUBLIC_KEY_SIZE:
            return record.public_key_bytes
    raise DIDResolutionError("no acceptable Ed25519 public key found")


@dataclasses.dataclass(frozen=True)
class Identity:
    """AXCP Ed25519 identity with a did:key identifier."""

    _signing_key: SigningKey
    did: str

    @classmethod
    def generate(cls) -> "Identity":
        signing_key = SigningKey.generate()
        return cls.from_seed(bytes(signing_key.encode()))

    @classmethod
    def from_seed(cls, seed: bytes, did: str | None = None) -> "Identity":
        if len(seed) != ED25519_PRIVATE_SEED_SIZE:
            raise InvalidPrivateKeyError("Ed25519 private seed must be 32 bytes")
        signing_key = SigningKey(seed)
        public_key = bytes(signing_key.verify_key.encode())
        if did is None:
            resolved_did = did_key_from_public_key(public_key)
        else:
            parsed = DID.parse(did)
            resolved_did = parsed.value
        if DID.parse(resolved_did).method == "key" and public_key_from_did_key(resolved_did) != public_key:
            raise KeyMismatchError("DID public key does not match private key")
        return cls(_signing_key=signing_key, did=resolved_did)

    @classmethod
    def from_json(cls, data: Mapping[str, str]) -> "Identity":
        seed_b64 = data.get("private_seed_b64")
        if not seed_b64:
            raise InvalidPrivateKeyError("private_seed_b64 is required")
        seed = base64.b64decode(seed_b64, validate=True)
        return cls.from_seed(seed, did=data.get("did"))

    @property
    def public_key(self) -> bytes:
        return bytes(self._signing_key.verify_key.encode())

    @property
    def private_seed(self) -> bytes:
        return bytes(self._signing_key.encode())

    def sign(self, payload: bytes) -> bytes:
        return bytes(self._signing_key.sign(payload).signature)

    def verify(self, payload: bytes, signature: bytes) -> None:
        verify_key = VerifyKey(self.public_key)
        try:
            verify_key.verify(payload, signature)
        except BadSignatureError as exc:
            raise SignatureVerificationError("signature verification failed") from exc

    def to_json(self) -> dict[str, str]:
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
    try:
        VerifyKey(public_key).verify(payload, signature)
    except BadSignatureError as exc:
        raise SignatureVerificationError("signature verification failed") from exc
