#!/usr/bin/env python3
"""
Gateway PoC: translate MCP JSON request <-> AXCP Envelope.

This bridge supports both transport-only (profile 0) and secure-baseline-v1 (profile 1).
For secure-baseline-v1, envelopes include DID auth fields and Ed25519 signatures.

Requires the installed Python SDK: pip install -e sdk/python
Run: python gateway/mcp_bridge.py mcp.json --profile secure-baseline --remote-did <recipient>
"""
import json
import sys
import os
import base64
import time
from typing import Optional
from dataclasses import dataclass, field
from threading import Lock

# Add the root directory and proto directory to sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'proto')))

from axcp.errors import AXCPError
from axcp.identity import DID, Identity
from axcp.envelope import (
    signing_payload,
    sign_envelope as sdk_sign_envelope,
    verify_envelope as sdk_verify_envelope,
)
from axcp.pb import axcp_pb2 as axcp

NACL_AVAILABLE = True  # PyNaCl is a required dependency of the Python SDK.


# Profile constants
PROFILE_TRANSPORT_ONLY = 0
PROFILE_SECURE_BASELINE = 1

# Transcript version for DID auth
DID_AUTH_TRANSCRIPT_VERSION = "AXCP-DID-AUTH-v1"


@dataclass
class BridgeIdentity:
    """Identity for signing/verifying messages."""
    did: str
    signing_key: Optional[bytes] = None  # Ed25519 private key (32 bytes)
    verify_key: Optional[bytes] = None   # Ed25519 public key (32 bytes)


@dataclass
class BridgeSession:
    """Session state for bridge connections."""
    local_identity: BridgeIdentity
    remote_did: Optional[str] = None
    profile: int = PROFILE_TRANSPORT_ONLY
    sequence: int = 0
    negotiated: bool = False
    _sequence_lock: Lock = field(default_factory=Lock, repr=False, compare=False)

    def next_sequence(self) -> int:
        """Get next sequence number for replay protection."""
        with self._sequence_lock:
            if not 0 <= self.sequence < (1 << 64) - 1:
                raise ValueError("sequence exhausted or invalid; create a new identity/session")
            self.sequence += 1
            return self.sequence


def generate_test_identity() -> BridgeIdentity:
    """Generate an ephemeral, canonical did:key identity for local demonstrations."""
    identity = Identity.generate()
    return BridgeIdentity(identity.did, identity.private_seed, identity.public_key)


def sign_envelope(session: BridgeSession, env: axcp.AxcpEnvelope) -> None:
    """Sign the complete envelope, including its allocated replay sequence."""
    if not session.local_identity.signing_key:
        raise ValueError("secure-baseline requires an Ed25519 private seed")
    identity = Identity.from_seed(session.local_identity.signing_key, did=session.local_identity.did)
    if session.local_identity.verify_key is not None and session.local_identity.verify_key != identity.public_key:
        raise ValueError("configured verification key does not match the private seed")
    if not session.remote_did:
        raise ValueError("secure-baseline requires the recipient DID")
    DID.parse(session.remote_did)
    sdk_sign_envelope(env, identity, recipient_did=session.remote_did, sequence=session.next_sequence())


def mcp_to_axcp(mcp_json: dict, session: Optional[BridgeSession] = None) -> axcp.AxcpEnvelope:
    """
    Convert MCP JSON request to AXCP Envelope.

    If session is provided and profile is secure-baseline, adds auth fields.
    """
    profile = PROFILE_TRANSPORT_ONLY
    if session:
        if session.profile not in (PROFILE_TRANSPORT_ONLY, PROFILE_SECURE_BASELINE):
            raise ValueError("unsupported bridge profile")
        if session.profile == PROFILE_SECURE_BASELINE and not session.negotiated:
            raise ValueError("secure-baseline session must be explicitly configured before conversion")
        if session.negotiated:
            profile = session.profile

    env = axcp.AxcpEnvelope(
        version=1,
        trace_id=mcp_json.get("id", ""),
        profile=profile
    )

    # Build capability offer from MCP tool
    offer = axcp.CapabilityOffer(
        desc=axcp.CapabilityDescriptor(
            tool_id=mcp_json["tool"]["name"],
            descriptor_version=mcp_json["tool"].get("version", "0")
        )
    )
    env.capability_msg.offer.CopyFrom(offer)

    # Add auth fields for secure-baseline profile
    if profile == PROFILE_SECURE_BASELINE and session:
        sign_envelope(session, env)

    return env


def axcp_to_mcp(env: axcp.AxcpEnvelope) -> dict:
    """
    Convert AXCP Envelope to MCP JSON response.

    Very narrow mapping: ContextPatch -> MCP context_delta
    """
    patch = env.context_patch
    delta_b64 = base64.b64encode(patch.SerializeToString()).decode()

    result = {
        "id": env.trace_id,
        "context_delta": delta_b64,
        "ts": int(time.time() * 1000)
    }

    # Include auth metadata if present (for verification purposes)
    if env.sender_did:
        result["_auth"] = {
            "sender_did": env.sender_did,
            "timestamp_ms": env.timestamp_ms,
            "sequence": env.sequence,
            "profile": "secure-baseline-v1" if env.profile == PROFILE_SECURE_BASELINE else "transport-only"
        }

    return result


def verify_envelope_signature(env: axcp.AxcpEnvelope, resolver: dict) -> bool:
    """
    Verify an envelope's signature using a DID resolver (dict of DID -> public key bytes).

    Returns True if signature is valid, False otherwise.
    """
    public_key = resolver.get(env.sender_did)
    if public_key is None:
        return False
    try:
        sdk_verify_envelope(env, public_key=public_key)
    except AXCPError:
        return False
    return True


def main():
    import argparse

    parser = argparse.ArgumentParser(description='MCP <-> AXCP Bridge')
    parser.add_argument('input_file', help='MCP JSON input file')
    parser.add_argument('--profile', choices=['transport-only', 'secure-baseline'],
                        default='transport-only', help='Security profile to use')
    parser.add_argument('--identity-file', help='Python SDK identity JSON containing a private seed')
    parser.add_argument('--remote-did', help='Remote DID for directed messages')
    args = parser.parse_args()

    if args.identity_file:
        with open(args.identity_file) as identity_file:
            loaded = Identity.from_json(json.load(identity_file))
        identity = BridgeIdentity(loaded.did, loaded.private_seed, loaded.public_key)
    else:
        identity = generate_test_identity()
        print(f"Generated ephemeral identity: {identity.did}")

    session = BridgeSession(local_identity=identity)

    if args.profile == 'secure-baseline':
        session.profile = PROFILE_SECURE_BASELINE
        if not args.remote_did:
            parser.error("--remote-did is required for secure-baseline")
        session.remote_did = args.remote_did
        session.negotiated = True
        print(f"Profile: secure-baseline-v1")
        print(f"Remote DID: {session.remote_did}")
    else:
        session.profile = PROFILE_TRANSPORT_ONLY
        session.negotiated = True
        print(f"Profile: transport-only-v0")

    # Load and convert MCP JSON
    with open(args.input_file) as f:
        mcp = json.load(f)

    print(f"\nInput MCP JSON:")
    print(json.dumps(mcp, indent=2))

    # Convert to AXCP
    env = mcp_to_axcp(mcp, session)
    print(f"\nAXCP Envelope:")
    print(f"  version: {env.version}")
    print(f"  trace_id: {env.trace_id}")
    print(f"  profile: {env.profile}")
    print(f"  tool_id: {env.capability_msg.offer.desc.tool_id}")

    if env.sender_did:
        print(f"  sender_did: {env.sender_did}")
        print(f"  recipient_did: {env.recipient_did}")
        print(f"  timestamp_ms: {env.timestamp_ms}")
        print(f"  sequence: {env.sequence}")
        print(f"  signature: {len(env.signature)} bytes")

    # Round-trip demo
    back = axcp_to_mcp(env)
    print(f"\nBack to MCP JSON:")
    print(json.dumps(back, indent=2))


if __name__ == "__main__":
    main()
