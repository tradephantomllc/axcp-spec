import base64

import pytest

from axcp.errors import InvalidDIDFormat, KeyMismatchError, SignatureVerificationError
from axcp.identity import (
    DID,
    Identity,
    did_key_from_public_key,
    public_key_from_did_key,
)


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


def test_did_parser_rejects_invalid_values() -> None:
    invalid = ["", "alice", "did::alice", "did:example:", "did:example:alice bob"]
    for value in invalid:
        with pytest.raises(InvalidDIDFormat):
            DID.parse(value)


def test_from_json_requires_private_seed() -> None:
    with pytest.raises(Exception):
        Identity.from_json({"did": "did:key:zabc"})

    identity = Identity.generate()
    malformed = identity.to_json()
    malformed["private_seed_b64"] = base64.b64encode(b"short").decode("ascii")
    with pytest.raises(Exception):
        Identity.from_json(malformed)
