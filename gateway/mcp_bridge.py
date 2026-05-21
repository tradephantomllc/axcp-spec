#!/usr/bin/env python3
"""
Gateway PoC: translate MCP JSON request <-> AXCP Envelope.

This bridge supports both transport-only (profile 0) and secure-baseline-v1 (profile 1).
For secure-baseline-v1, envelopes include DID auth fields and Ed25519 signatures.

Run: python gateway/mcp_bridge.py mcp.json [--profile secure-baseline]
"""
import json
import sys
import os
import base64
import time
import hashlib
from typing import Optional, Tuple
from dataclasses import dataclass

# Add the root directory and proto directory to sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'proto')))

try:
    from nacl.signing import SigningKey, VerifyKey
    from nacl.encoding import RawEncoder
    NACL_AVAILABLE = True
except ImportError:
    NACL_AVAILABLE = False

from google.protobuf import json_format
import proto.axcp_pb2 as axcp


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

    def next_sequence(self) -> int:
        """Get next sequence number for replay protection."""
        self.sequence += 1
        return self.sequence


def generate_test_identity() -> BridgeIdentity:
    """Generate a test identity with DID and Ed25519 keypair."""
    if not NACL_AVAILABLE:
        # Fallback: create identity without signing capability
        import secrets
        random_bytes = secrets.token_bytes(8)
        did = f"did:key:test-{base64.urlsafe_b64encode(random_bytes).decode().rstrip('=')}"
        return BridgeIdentity(did=did)

    signing_key = SigningKey.generate()
    verify_key = signing_key.verify_key

    # Create DID from public key
    pk_bytes = bytes(verify_key)
    did = f"did:key:test-{base64.urlsafe_b64encode(pk_bytes[:8]).decode().rstrip('=')}"

    return BridgeIdentity(
        did=did,
        signing_key=bytes(signing_key),
        verify_key=pk_bytes
    )


def build_auth_transcript(sender_did: str, recipient_did: str, payload: bytes, timestamp_ms: int) -> bytes:
    """
    Build canonical auth transcript for signing.

    Format:
        AXCP-DID-AUTH-v1
        <sender_did>
        <recipient_did>
        <base64(payload)>
        <timestamp_rfc3339>
    """
    from datetime import datetime, timezone

    payload_b64 = base64.b64encode(payload).decode()
    ts = datetime.fromtimestamp(timestamp_ms / 1000, tz=timezone.utc)
    timestamp_str = ts.strftime('%Y-%m-%dT%H:%M:%SZ')

    transcript = '\n'.join([
        DID_AUTH_TRANSCRIPT_VERSION,
        sender_did,
        recipient_did,
        payload_b64,
        timestamp_str
    ])

    return transcript.encode('utf-8')


def signing_payload(env: axcp.AxcpEnvelope) -> bytes:
    """Return the canonical envelope bytes covered by the auth transcript."""
    env_copy = axcp.AxcpEnvelope()
    env_copy.CopyFrom(env)
    env_copy.ClearField('signature')
    env_copy.ClearField('timestamp_ms')
    env_copy.ClearField('sequence')
    env_copy.ClearField('sender_did')
    env_copy.ClearField('recipient_did')
    env_copy.ClearField('attestation_proof')
    return env_copy.SerializeToString(deterministic=True)


def sign_envelope(session: BridgeSession, payload: bytes) -> Tuple[bytes, int, int]:
    """
    Sign a payload for secure-baseline profile.

    Returns: (signature, timestamp_ms, sequence)
    """
    if not NACL_AVAILABLE or not session.local_identity.signing_key:
        raise ValueError("Signing not available: PyNaCl not installed or no signing key")

    timestamp_ms = int(time.time() * 1000)
    sequence = session.next_sequence()

    transcript = build_auth_transcript(
        session.local_identity.did,
        session.remote_did or "",
        payload,
        timestamp_ms
    )

    signing_key = SigningKey(session.local_identity.signing_key)
    signed = signing_key.sign(transcript, encoder=RawEncoder)
    signature = signed.signature

    return signature, timestamp_ms, sequence


def mcp_to_axcp(mcp_json: dict, session: Optional[BridgeSession] = None) -> axcp.AxcpEnvelope:
    """
    Convert MCP JSON request to AXCP Envelope.

    If session is provided and profile is secure-baseline, adds auth fields.
    """
    profile = PROFILE_TRANSPORT_ONLY
    if session and session.negotiated:
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
        env.sender_did = session.local_identity.did
        if session.remote_did:
            env.recipient_did = session.remote_did

        payload = signing_payload(env)

        try:
            signature, timestamp_ms, sequence = sign_envelope(session, payload)
            env.timestamp_ms = timestamp_ms
            env.sequence = sequence
            env.signature = signature
        except ValueError as e:
            # Signing not available, log warning but continue
            print(f"Warning: Could not sign envelope: {e}", file=sys.stderr)

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
    if not NACL_AVAILABLE:
        print("Warning: PyNaCl not available, cannot verify signatures", file=sys.stderr)
        return False

    if not env.sender_did or not env.signature:
        return False

    # Look up public key
    public_key_bytes = resolver.get(env.sender_did)
    if not public_key_bytes:
        print(f"Warning: DID not found in resolver: {env.sender_did}", file=sys.stderr)
        return False

    payload = signing_payload(env)

    transcript = build_auth_transcript(
        env.sender_did,
        env.recipient_did or "",
        payload,
        env.timestamp_ms
    )

    try:
        verify_key = VerifyKey(public_key_bytes)
        verify_key.verify(transcript, env.signature)
        return True
    except Exception as e:
        print(f"Signature verification failed: {e}", file=sys.stderr)
        return False


def main():
    import argparse

    parser = argparse.ArgumentParser(description='MCP <-> AXCP Bridge')
    parser.add_argument('input_file', help='MCP JSON input file')
    parser.add_argument('--profile', choices=['transport-only', 'secure-baseline'],
                        default='transport-only', help='Security profile to use')
    parser.add_argument('--local-did', help='Local DID (auto-generated if not provided)')
    parser.add_argument('--remote-did', help='Remote DID for directed messages')
    args = parser.parse_args()

    # Create session
    if args.local_did:
        identity = BridgeIdentity(did=args.local_did)
    else:
        identity = generate_test_identity()
        print(f"Generated test identity: {identity.did}")

    session = BridgeSession(local_identity=identity)

    if args.profile == 'secure-baseline':
        session.profile = PROFILE_SECURE_BASELINE
        session.remote_did = args.remote_did or "did:key:gateway-server"
        session.negotiated = True
        print(f"Profile: secure-baseline-v1")
        print(f"Remote DID: {session.remote_did}")
        if not NACL_AVAILABLE:
            print("Warning: PyNaCl not installed, signatures will be skipped")
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
