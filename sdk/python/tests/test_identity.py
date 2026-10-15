import base64
import time

import pytest

from axcp import Agent
from axcp.envelope import encode_envelope, new_envelope
from axcp.errors import (
    DIDResolutionError,
    InvalidDIDFormat,
    InvalidPrivateKeyError,
    KeyMismatchError,
    SignatureVerificationError,
)
from axcp.identity import (
    DID,
    MAX_DID_LENGTH,
    SMALL_ORDER_PUBLIC_KEYS,
    DIDDocument,
    Identity,
    InMemoryDIDResolver,
    PublicKeyRecord,
    _b58encode,
    did_key_from_public_key,
    is_small_order_public_key,
    public_key_from_did_key,
    resolve_ed25519_public_key,
    verify_signature,
)
from axcp.pb import axcp_pb2

SMALL_ORDER_HEX = [
    "0000000000000000000000000000000000000000000000000000000000000000",
    "0100000000000000000000000000000000000000000000000000000000000000",
    "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
    "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
    "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
    "edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
    "eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
]


def _context_envelope():
    env = new_envelope(trace_id="identity")
    env.context_patch.CopyFrom(axcp_pb2.ContextPatch(context_id="ctx", base_version=1))
    return env


def test_identity_generate_roundtrip_did_key() -> None:
    identity = Identity.generate()

    parsed = DID.parse(identity.did)
    assert parsed.method == "key"
    assert public_key_from_did_key(identity.did) == identity.public_key
    assert did_key_from_public_key(identity.public_key) == identity.did


def test_identity_seed_json_roundtrip() -> None:
    identity = Identity.generate()
    restored = Identity.from_json(identity.to_json())

    assert restored.did == identity.did
    assert restored.public_key == identity.public_key
    assert restored.private_seed == identity.private_seed


def test_identity_sign_and_verify() -> None:
    identity = Identity.generate()
    payload = b"AXCP test payload"
    signature = identity.sign(payload)

    identity.verify(payload, signature)
    with pytest.raises(SignatureVerificationError):
        identity.verify(b"tampered", signature)


def test_identity_rejects_did_key_mismatch() -> None:
    left = Identity.generate()
    right = Identity.generate()

    with pytest.raises(KeyMismatchError):
        Identity.from_seed(left.private_seed, did=right.did)


def test_identity_allows_non_key_did_labels() -> None:
    identity = Identity.generate()
    labeled = Identity.from_seed(identity.private_seed, did="did:example:alice")

    assert labeled.did == "did:example:alice"
    assert labeled.public_key == identity.public_key


def test_identity_repr_never_contains_key_material() -> None:
    identity = Identity.generate()
    rendered = repr(identity) + str(identity)
    for secret in (identity.private_seed.hex(), base64.b64encode(identity.private_seed).decode(), repr(identity.private_seed)):
        assert secret not in rendered
    assert "_signing_key" not in rendered
    assert identity.did in rendered


def test_from_json_requires_valid_private_seed() -> None:
    with pytest.raises(InvalidPrivateKeyError):
        Identity.from_json({"did": "did:key:zabc"})

    identity = Identity.generate()
    malformed = identity.to_json()
    malformed["private_seed_b64"] = base64.b64encode(b"short").decode("ascii")
    with pytest.raises(InvalidPrivateKeyError):
        Identity.from_json(malformed)

    malformed["private_seed_b64"] = "not base64!"
    with pytest.raises(InvalidPrivateKeyError):
        Identity.from_json(malformed)

    mismatched = identity.to_json()
    mismatched["public_key_b64"] = base64.b64encode(Identity.generate().public_key).decode("ascii")
    with pytest.raises(KeyMismatchError):
        Identity.from_json(mismatched)


# --- D1 ---------------------------------------------------------------------


@pytest.mark.parametrize(
    "value",
    [
        "did:example:alice",
        "did:web:example.com:user:alice",
        "did:example:a%20b",
        "did:example::alice",
        "did:example:A-b_c.d",
        "did:3:x",
    ],
)
def test_did_parser_accepts_valid_values(value: str) -> None:
    assert DID.parse(value).value == value


@pytest.mark.parametrize(
    "value",
    [
        "",
        "alice",
        "did::alice",
        "did:example:",
        "did:example:alice bob",
        "did:Example:alice",
        "did:ex-ample:alice",
        "did:example:alice/path",
        "did:example:alice?query=1",
        "did:example:alice#key-1",
        "did:example:alice:",
        "did:example:a%2",
        "did:example:a%zz",
        "did:example:alé",
        "did:example:a​b",
        "did:example:a\nb",
        "DID:example:alice",
    ],
)
def test_did_parser_rejects_invalid_values(value: str) -> None:
    with pytest.raises(InvalidDIDFormat):
        DID.parse(value)


def test_did_parser_rejects_non_strings() -> None:
    with pytest.raises(InvalidDIDFormat):
        DID.parse(b"did:example:alice")  # type: ignore[arg-type]


def test_did_length_limit() -> None:
    prefix = "did:example:"
    at_limit = prefix + "a" * (MAX_DID_LENGTH - len(prefix))
    assert DID.parse(at_limit).value == at_limit
    with pytest.raises(InvalidDIDFormat, match="exceeds 512 bytes"):
        DID.parse(at_limit + "a")


def test_ten_megabyte_did_is_rejected_quickly() -> None:
    huge = "did:key:z" + "6" * (10 * 1024 * 1024)
    started = time.perf_counter()
    for call in (DID.parse, public_key_from_did_key, resolve_ed25519_public_key):
        with pytest.raises(InvalidDIDFormat, match="exceeds 512 bytes"):
            call(huge)
    assert time.perf_counter() - started < 0.05


# --- D2 ---------------------------------------------------------------------


def test_did_key_requires_z_multibase() -> None:
    identity = Identity.generate()
    with pytest.raises(InvalidDIDFormat, match="base58btc multibase"):
        public_key_from_did_key(identity.did.replace(":z", ":Z"))


def test_did_key_identifier_length_is_checked_before_decoding() -> None:
    with pytest.raises(InvalidDIDFormat, match="exceeds 128 characters"):
        public_key_from_did_key("did:key:z" + "1" * 128)


def test_did_key_requires_ed25519_prefix_and_exact_length() -> None:
    key = Identity.generate().public_key
    for decoded in (b"\xed\x01" + key[:31], b"\xed\x01" + key + b"\x00"):
        with pytest.raises(InvalidDIDFormat, match="32-byte key"):
            public_key_from_did_key("did:key:z" + _b58encode(decoded))
    with pytest.raises(InvalidDIDFormat, match="not an Ed25519"):
        public_key_from_did_key("did:key:z" + _b58encode(b"\xec\x01" + key))
    assert public_key_from_did_key("did:key:z" + _b58encode(b"\xed\x01" + key)) == key


def test_did_key_rejects_small_order_keys() -> None:
    for value in SMALL_ORDER_HEX:
        with pytest.raises(InvalidDIDFormat, match="small-order"):
            public_key_from_did_key(did_key_from_public_key(bytes.fromhex(value)))


# --- D9 ---------------------------------------------------------------------


def test_small_order_blocklist_matches_design() -> None:
    assert {key.hex() for key in SMALL_ORDER_PUBLIC_KEYS} == set(SMALL_ORDER_HEX)
    for value in SMALL_ORDER_HEX:
        key = bytes.fromhex(value)
        signed = key[:31] + bytes([key[31] | 0x80])
        assert is_small_order_public_key(key)
        assert is_small_order_public_key(signed)
        with pytest.raises(SignatureVerificationError, match="small order"):
            verify_signature(key, b"message", b"\x01" + b"\x00" * 63)
    assert not is_small_order_public_key(Identity.generate().public_key)


# --- D3 ---------------------------------------------------------------------


class _StaticResolver:
    def __init__(self, document) -> None:
        self._document = document

    def resolve(self, did: str):
        if isinstance(self._document, Exception):
            raise self._document
        return self._document


def test_resolver_is_consulted_for_did_key_senders() -> None:
    alice = Agent(Identity.generate())
    resolver = InMemoryDIDResolver()
    bob = Agent(Identity.generate(), resolver=resolver)
    env = alice.sign_message(_context_envelope(), recipient_did=bob.identity.did)

    with pytest.raises(DIDResolutionError):
        bob.verify_bytes(encode_envelope(env))
    resolver.register(alice.identity)
    bob.verify_bytes(encode_envelope(env))


def test_resolved_key_must_match_did_key() -> None:
    alice = Identity.generate()
    other = Identity.generate()
    resolver = _StaticResolver(
        DIDDocument(
            id=alice.did,
            public_keys=(PublicKeyRecord("k", "Ed25519VerificationKey2020", other.public_key),),
        )
    )
    with pytest.raises(KeyMismatchError):
        resolve_ed25519_public_key(alice.did, resolver)


@pytest.mark.parametrize(
    ("document", "pattern"),
    [
        (
            DIDDocument(
                id="did:example:mallory",
                public_keys=(PublicKeyRecord("k", "Ed25519VerificationKey2020", b"\x11" * 32),),
            ),
            "does not match",
        ),
        (
            DIDDocument(
                id="did:example:alice",
                public_keys=(PublicKeyRecord("k", "X25519KeyAgreementKey2019", b"\x11" * 32),),
            ),
            "no acceptable",
        ),
        (
            DIDDocument(
                id="did:example:alice",
                public_keys=(PublicKeyRecord("k", "Ed25519VerificationKey2018", b"\x11" * 31),),
            ),
            "no acceptable",
        ),
        (
            DIDDocument(
                id="did:example:alice",
                public_keys=(PublicKeyRecord("k", "Ed25519VerificationKey2020", b"\x00" * 32),),
            ),
            "small order",
        ),
        (RuntimeError("network down"), "DID resolution failed"),
        (object(), "invalid DID document"),
    ],
)
def test_resolver_documents_are_validated(document, pattern: str) -> None:
    with pytest.raises(DIDResolutionError, match=pattern):
        resolve_ed25519_public_key("did:example:alice", _StaticResolver(document))


def test_resolver_skips_unacceptable_records() -> None:
    key = Identity.generate().public_key
    document = DIDDocument(
        id="did:example:alice",
        public_keys=(
            PublicKeyRecord("x", "X25519KeyAgreementKey2019", b"\x00" * 32),
            PublicKeyRecord("k", "Ed25519VerificationKey2018", key),
        ),
    )
    assert resolve_ed25519_public_key("did:example:alice", _StaticResolver(document)) == key


def test_open_mode_decodes_did_key_and_rejects_other_methods() -> None:
    alice = Identity.generate()
    assert resolve_ed25519_public_key(alice.did) == alice.public_key
    with pytest.raises(DIDResolutionError):
        resolve_ed25519_public_key("did:example:alice")
