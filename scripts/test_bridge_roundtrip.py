#!/usr/bin/env python3
"""
Test bridge round-trip for both transport-only and secure-baseline profiles.

Run: python scripts/test_bridge_roundtrip.py
"""
import sys
import os

# Add the root directory and proto directory to sys.path
# Go up 1 level from scripts/ to reach the root of the repo
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'proto')))

import json
import base64
from google.protobuf import json_format
import proto.axcp_pb2 as axcp
from gateway.mcp_bridge import (
    mcp_to_axcp, axcp_to_mcp,
    BridgeIdentity, BridgeSession,
    PROFILE_TRANSPORT_ONLY, PROFILE_SECURE_BASELINE,
    generate_test_identity, verify_envelope_signature,
    NACL_AVAILABLE
)


def test_transport_only():
    """Test bridge round-trip with transport-only profile (no auth)."""
    mcp = {"id": "req-123", "tool": {"name": "search", "version": "1.0"}}
    env = mcp_to_axcp(mcp)

    assert env.version == 1
    assert env.profile == PROFILE_TRANSPORT_ONLY
    assert env.capability_msg.offer.desc.tool_id == "search"
    assert env.sender_did == ""  # No auth fields for transport-only
    assert env.signature == b""

    back = axcp_to_mcp(env)
    assert back["id"] == "req-123"
    assert "_auth" not in back  # No auth metadata for transport-only

    print("  transport-only round-trip OK")


def test_transport_only_with_session():
    """Test bridge with explicit transport-only session."""
    identity = generate_test_identity()
    session = BridgeSession(
        local_identity=identity,
        profile=PROFILE_TRANSPORT_ONLY,
        negotiated=True
    )

    mcp = {"id": "req-456", "tool": {"name": "query", "version": "2.0"}}
    env = mcp_to_axcp(mcp, session)

    assert env.profile == PROFILE_TRANSPORT_ONLY
    assert env.sender_did == ""  # No auth even with session for transport-only

    back = axcp_to_mcp(env)
    assert back["id"] == "req-456"

    print("  transport-only with session OK")


def test_secure_baseline():
    """Test bridge round-trip with secure-baseline profile."""
    if not NACL_AVAILABLE:
        print("  secure-baseline SKIPPED (PyNaCl not installed)")
        return

    identity = generate_test_identity()
    session = BridgeSession(
        local_identity=identity,
        remote_did="did:key:gateway-server",
        profile=PROFILE_SECURE_BASELINE,
        negotiated=True
    )

    mcp = {"id": "req-789", "tool": {"name": "execute", "version": "1.0"}}
    env = mcp_to_axcp(mcp, session)

    assert env.version == 1
    assert env.profile == PROFILE_SECURE_BASELINE
    assert env.capability_msg.offer.desc.tool_id == "execute"
    assert env.sender_did == identity.did
    assert env.recipient_did == "did:key:gateway-server"
    assert env.timestamp_ms > 0
    assert env.sequence == 1
    assert len(env.signature) == 64  # Ed25519 signature is 64 bytes

    back = axcp_to_mcp(env)
    assert back["id"] == "req-789"
    assert "_auth" in back
    assert back["_auth"]["sender_did"] == identity.did
    assert back["_auth"]["profile"] == "secure-baseline-v1"

    print("  secure-baseline round-trip OK")


def test_secure_baseline_sequence():
    """Test that sequence numbers increment correctly."""
    if not NACL_AVAILABLE:
        print("  sequence increment SKIPPED (PyNaCl not installed)")
        return

    identity = generate_test_identity()
    session = BridgeSession(
        local_identity=identity,
        remote_did="did:key:zserver",
        profile=PROFILE_SECURE_BASELINE,
        negotiated=True
    )

    for expected_seq in range(1, 6):
        mcp = {"id": f"req-{expected_seq}", "tool": {"name": "test", "version": "1.0"}}
        env = mcp_to_axcp(mcp, session)
        assert env.sequence == expected_seq, f"Expected sequence {expected_seq}, got {env.sequence}"

    print("  sequence increment OK")


def test_signature_verification():
    """Test that signatures can be verified with the resolver."""
    if not NACL_AVAILABLE:
        print("  signature verification SKIPPED (PyNaCl not installed)")
        return

    identity = generate_test_identity()
    session = BridgeSession(
        local_identity=identity,
        remote_did="did:key:verifier",
        profile=PROFILE_SECURE_BASELINE,
        negotiated=True
    )

    mcp = {"id": "req-verify", "tool": {"name": "verify_test", "version": "1.0"}}
    env = mcp_to_axcp(mcp, session)

    # Create resolver with the identity's public key
    resolver = {identity.did: identity.verify_key}

    # Verify signature
    assert verify_envelope_signature(env, resolver), "Signature verification failed"

    # Verify that wrong key fails
    other_identity = generate_test_identity()
    wrong_resolver = {identity.did: other_identity.verify_key}
    assert not verify_envelope_signature(env, wrong_resolver), "Wrong key should fail"

    print("  signature verification OK")


def test_identity_generation():
    """Test that generated identities are unique and valid."""
    identity1 = generate_test_identity()
    identity2 = generate_test_identity()

    assert identity1.did != identity2.did, "DIDs should be unique"
    assert identity1.did.startswith("did:key:z")

    if NACL_AVAILABLE:
        assert identity1.verify_key is not None
        assert identity2.verify_key is not None
        assert identity1.verify_key != identity2.verify_key
        assert len(identity1.verify_key) == 32  # Ed25519 public key size

    print("  identity generation OK")


def main():
    print("MCP <-> AXCP Bridge Tests")
    print("-" * 40)

    test_transport_only()
    test_transport_only_with_session()
    test_secure_baseline()
    test_secure_baseline_sequence()
    test_signature_verification()
    test_identity_generation()

    print("-" * 40)
    print("All bridge tests PASSED")


if __name__ == "__main__":
    main()


def test_bridge_uses_sdk_signature_and_rejects_sequence_tampering():
    import pytest
    from axcp.envelope import verify_envelope, sign_envelope
    from axcp.errors import SignatureVerificationError
    from axcp.identity import Identity
    identity = generate_test_identity()
    session = BridgeSession(identity, "did:example:gateway", PROFILE_SECURE_BASELINE, negotiated=True)
    env = mcp_to_axcp({"id": "signed", "tool": {"name": "test"}}, session)
    verify_envelope(env, public_key=identity.verify_key)
    env.sequence += 1
    with pytest.raises(SignatureVerificationError):
        verify_envelope(env, public_key=identity.verify_key)
    assert not verify_envelope_signature(env, {identity.did: identity.verify_key})
    sign_envelope(env, Identity.from_seed(identity.signing_key), recipient_did=session.remote_did, sequence=3)
    assert verify_envelope_signature(env, {identity.did: identity.verify_key})


def test_secure_bridge_fails_closed():
    import pytest
    from axcp.errors import InvalidPrivateKeyError
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
