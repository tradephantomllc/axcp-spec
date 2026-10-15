#!/usr/bin/env python3
"""
Test bridge round-trip for secure-baseline (default) and opt-in transport-only profiles.

Run: python -m pytest scripts/test_bridge_roundtrip.py
"""
import os
import sys
import time

import pytest

# Go up 1 level from scripts/ to reach the root of the repo
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from axcp.envelope import sign_envelope, verify_envelope
from axcp.errors import InvalidPrivateKeyError, SignatureVerificationError
from axcp.identity import Identity
from axcp.timestamp import now_us
from gateway.mcp_bridge import (
    PROFILE_SECURE_BASELINE,
    PROFILE_TRANSPORT_ONLY,
    BridgeIdentity,
    BridgeSession,
    axcp_to_mcp,
    generate_test_identity,
    mcp_to_axcp,
    verify_envelope_signature,
)


def _pair():
    """Alice (sender) and Bob (receiver) sessions addressed to each other."""
    alice = generate_test_identity()
    bob = generate_test_identity()
    alice_session = BridgeSession(alice, remote_did=bob.did, profile=PROFILE_SECURE_BASELINE, negotiated=True)
    bob_session = BridgeSession(bob, remote_did=alice.did, profile=PROFILE_SECURE_BASELINE, negotiated=True)
    return alice, bob, alice_session, bob_session


def test_secure_baseline_is_the_default_profile():
    identity = generate_test_identity()
    assert BridgeSession(identity).profile == PROFILE_SECURE_BASELINE
    with pytest.raises(ValueError, match="allow_transport_only"):
        mcp_to_axcp({"id": "req", "tool": {"name": "search"}})


def test_transport_only_requires_explicit_opt_in():
    mcp = {"id": "req-123", "tool": {"name": "search", "version": "1.0"}}
    env = mcp_to_axcp(mcp, allow_transport_only=True)

    assert env.version == 1
    assert env.profile == PROFILE_TRANSPORT_ONLY
    assert env.capability_msg.offer.desc.tool_id == "search"
    assert env.sender_did == ""
    assert env.signature == b""

    back = axcp_to_mcp(env)
    assert back["id"] == "req-123"
    assert "_auth" not in back

    identity = generate_test_identity()
    refused = BridgeSession(identity, profile=PROFILE_TRANSPORT_ONLY, negotiated=True)
    with pytest.raises(ValueError, match="explicit allow_transport_only"):
        mcp_to_axcp(mcp, refused)
    allowed = BridgeSession(identity, profile=PROFILE_TRANSPORT_ONLY, negotiated=True, allow_transport_only=True)
    assert mcp_to_axcp(mcp, allowed).sender_did == ""


def test_secure_baseline_roundtrip_emits_auth_only_after_verification():
    alice, bob, alice_session, bob_session = _pair()
    env = mcp_to_axcp({"id": "req-789", "tool": {"name": "execute", "version": "1.0"}}, alice_session)

    assert env.profile == PROFILE_SECURE_BASELINE
    assert env.sender_did == alice.did
    assert env.recipient_did == bob.did
    assert env.timestamp_ms > 0
    assert len(env.signature) == 64

    # Without a verifying session no _auth metadata is ever emitted.
    assert "_auth" not in axcp_to_mcp(env)

    back = axcp_to_mcp(env, bob_session)
    assert back["id"] == "req-789"
    assert back["_auth"]["sender_did"] == alice.did
    assert back["_auth"]["profile"] == "secure-baseline-v1"

    # Replay of the same envelope is rejected.
    with pytest.raises(ValueError, match="verification"):
        axcp_to_mcp(env, bob_session)


def test_unverified_envelopes_never_get_auth_metadata():
    alice, _, alice_session, bob_session = _pair()
    env = mcp_to_axcp({"id": "forged", "tool": {"name": "x"}}, alice_session)
    env.capability_msg.offer.desc.tool_id = "tampered"
    with pytest.raises(ValueError):
        axcp_to_mcp(env, bob_session)

    unsigned = mcp_to_axcp({"id": "plain", "tool": {"name": "x"}}, allow_transport_only=True)
    unsigned.sender_did = alice.did  # claims an identity without a signature
    assert "_auth" not in axcp_to_mcp(unsigned)
    with pytest.raises(ValueError):
        axcp_to_mcp(unsigned, bob_session)


def test_secure_baseline_sequence_is_time_based_and_increments():
    before = now_us()
    _, _, alice_session, _ = _pair()
    sequences = [
        mcp_to_axcp({"id": f"req-{index}", "tool": {"name": "test", "version": "1.0"}}, alice_session).sequence
        for index in range(5)
    ]
    assert sequences[0] >= before
    assert sequences == list(range(sequences[0], sequences[0] + 5))


def test_verification_checks_recipient_timestamp_and_replay():
    alice, bob, alice_session, bob_session = _pair()
    carol = generate_test_identity()
    carol_session = BridgeSession(carol, remote_did=alice.did, negotiated=True)
    resolver = {alice.did: alice.verify_key}

    env = mcp_to_axcp({"id": "req-verify", "tool": {"name": "verify_test"}}, alice_session)
    assert not verify_envelope_signature(env, resolver, carol_session)  # wrong recipient
    assert verify_envelope_signature(env, resolver, bob_session)
    assert not verify_envelope_signature(env, resolver, bob_session)  # replay

    stale = mcp_to_axcp({"id": "stale", "tool": {"name": "verify_test"}}, alice_session)
    sign_envelope(
        stale,
        Identity.from_seed(alice.signing_key),
        recipient_did=bob.did,
        sequence=stale.sequence,
        timestamp_ms=int(time.time() * 1000) - 10 * 60 * 1000,
    )
    assert not verify_envelope_signature(stale, resolver, bob_session)  # expired timestamp

    wrong_key = {alice.did: generate_test_identity().verify_key}
    fresh = mcp_to_axcp({"id": "wrong-key", "tool": {"name": "verify_test"}}, alice_session)
    assert not verify_envelope_signature(fresh, wrong_key, bob_session)
    assert not verify_envelope_signature(fresh, {}, bob_session)  # resolver acts as allowlist


def test_identity_generation_and_repr_hygiene():
    identity1 = generate_test_identity()
    identity2 = generate_test_identity()

    assert identity1.did != identity2.did
    assert identity1.did.startswith("did:key:z")
    assert len(identity1.verify_key) == 32
    assert identity1.verify_key != identity2.verify_key

    session = BridgeSession(identity1, remote_did=identity2.did, negotiated=True)
    rendered = repr(identity1) + repr(session)
    assert identity1.signing_key.hex() not in rendered
    assert repr(identity1.signing_key) not in rendered
    assert repr(identity1.verify_key) not in rendered
    assert identity1.did in rendered


def test_bridge_uses_sdk_signature_and_rejects_sequence_tampering():
    identity = generate_test_identity()
    session = BridgeSession(identity, "did:example:gateway", PROFILE_SECURE_BASELINE, negotiated=True)
    env = mcp_to_axcp({"id": "signed", "tool": {"name": "test"}}, session)
    verify_envelope(env, expected_recipient_did="did:example:gateway", public_key=identity.verify_key)
    env.sequence += 1
    with pytest.raises(SignatureVerificationError):
        verify_envelope(env, expected_recipient_did="did:example:gateway", public_key=identity.verify_key)
    assert not verify_envelope_signature(env, {identity.did: identity.verify_key})
    sign_envelope(env, Identity.from_seed(identity.signing_key), recipient_did=session.remote_did, sequence=3)
    assert verify_envelope_signature(env, {identity.did: identity.verify_key})


def test_secure_bridge_fails_closed():
    request = {"id": "fail", "tool": {"name": "test"}}
    identity = generate_test_identity()
    with pytest.raises(ValueError, match="configured"):
        mcp_to_axcp(request, BridgeSession(identity, profile=PROFILE_SECURE_BASELINE))
    with pytest.raises(ValueError, match="private seed"):
        mcp_to_axcp(request, BridgeSession(BridgeIdentity(identity.did), "did:example:server", PROFILE_SECURE_BASELINE, negotiated=True))
    with pytest.raises(InvalidPrivateKeyError):
        mcp_to_axcp(request, BridgeSession(BridgeIdentity(identity.did, b"bad"), "did:example:server", PROFILE_SECURE_BASELINE, negotiated=True))
    with pytest.raises(ValueError, match="recipient"):
        mcp_to_axcp(request, BridgeSession(identity, profile=PROFILE_SECURE_BASELINE, negotiated=True))
