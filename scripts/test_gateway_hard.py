#!/usr/bin/env python3
"""Bridge hardening tests that use the bundled MCP samples (cwd-independent)."""
import json
import os
import sys
from pathlib import Path

import pytest

# Add the project root to Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from axcp.envelope import encode_envelope
from axcp.errors import InvalidEnvelopeError
from axcp.identity import Identity
from axcp.agent import Agent
from gateway.mcp_bridge import (
    PROFILE_SECURE_BASELINE,
    PROFILE_TRANSPORT_ONLY,
    BridgeSession,
    axcp_to_mcp,
    generate_test_identity,
    mcp_to_axcp,
    verify_envelope_signature,
)

SAMPLES = Path(__file__).resolve().parents[1] / "gateway" / "samples"


def load(name):
    return json.loads((SAMPLES / name).read_text(encoding="utf-8"))


def _sessions():
    alice = generate_test_identity()
    bob = generate_test_identity()
    return (
        alice,
        bob,
        BridgeSession(alice, remote_did=bob.did, profile=PROFILE_SECURE_BASELINE, negotiated=True),
        BridgeSession(bob, remote_did=alice.did, profile=PROFILE_SECURE_BASELINE, negotiated=True),
    )


def test_mcp_roundtrip():
    claude = load("mcp_claude_search.json")
    _, _, alice_session, bob_session = _sessions()
    env = mcp_to_axcp(claude, alice_session)
    assert env.capability_msg.offer.desc.tool_id == claude["tool"]["name"]
    back = axcp_to_mcp(env, bob_session)
    assert back["id"] == claude["id"]
    assert back["context_delta"] == ""  # capability offers carry no context patch
    assert back["_auth"]["profile"] == "secure-baseline-v1"
    assert isinstance(back["ts"], int) and back["ts"] > 0


def test_profile_downgrade():
    """A secure-baseline envelope downgraded on the wire must not be accepted."""
    claude = load("mcp_claude_search.json")
    alice, bob, alice_session, bob_session = _sessions()
    resolver = {alice.did: alice.verify_key}
    bob_agent = Agent(Identity.from_seed(bob.signing_key))

    for downgraded_profile in (PROFILE_TRANSPORT_ONLY, 2, 3):
        env = mcp_to_axcp(claude, alice_session)
        env.profile = downgraded_profile
        # The profile is covered by the signature and must be secure-baseline-v1.
        with pytest.raises(InvalidEnvelopeError, match="secure-baseline-v1"):
            bob_agent.verify_bytes(encode_envelope(env))
        assert not verify_envelope_signature(env, resolver, bob_session)
        with pytest.raises(ValueError, match="verification"):
            axcp_to_mcp(env, bob_session)

    # A downgrade that also strips the auth fields is not authenticated either.
    stripped = mcp_to_axcp(claude, alice_session)
    stripped.profile = PROFILE_TRANSPORT_ONLY
    stripped.ClearField("signature")
    assert not verify_envelope_signature(stripped, resolver, bob_session)
    assert "_auth" not in axcp_to_mcp(stripped)

    # The bridge itself refuses to emit transport-only envelopes without an opt-in.
    with pytest.raises(ValueError, match="allow_transport_only"):
        mcp_to_axcp(claude)
    plain = BridgeSession(alice, profile=PROFILE_TRANSPORT_ONLY, negotiated=True)
    with pytest.raises(ValueError, match="allow_transport_only"):
        mcp_to_axcp(claude, plain)
