#!/usr/bin/env python3
"""
Gateway PoC: translate MCP JSON request <-> AXCP Envelope.

The bridge defaults to secure-baseline-v1 (profile 1): envelopes carry DID auth fields
and Ed25519 signatures. The unauthenticated transport-only profile 0 is available only
with an explicit opt-in (``--allow-transport-only`` / ``allow_transport_only=True``).

Requires the installed Python SDK: pip install -e sdk/python
Run: python gateway/mcp_bridge.py mcp.json --remote-did <recipient>
"""
import json
import sys
import os
import base64
import time
from typing import Mapping, Optional, Union
from dataclasses import dataclass, field
from threading import Lock

# Add the repository root to sys.path so ``gateway`` can be imported from scripts.
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from axcp.errors import AXCPError
from axcp.identity import DID, DIDResolver, Identity
from axcp.envelope import (
    sign_envelope as sdk_sign_envelope,
    verify_envelope as sdk_verify_envelope,
)
from axcp.pb import axcp_pb2 as axcp
from axcp.replay import ReplayProtector
from axcp.timestamp import TimestampValidator, now_us

NACL_AVAILABLE = True  # PyNaCl is a required dependency of the Python SDK.


# Profile constants
PROFILE_TRANSPORT_ONLY = 0
PROFILE_SECURE_BASELINE = 1

# Transcript version for DID auth
DID_AUTH_TRANSCRIPT_VERSION = "AXCP-DID-AUTH-v1"

# A resolver is either an SDK DIDResolver or a plain mapping DID -> Ed25519 public key.
BridgeResolver = Union[DIDResolver, Mapping[str, bytes], None]


@dataclass
class BridgeIdentity:
    """Identity for signing/verifying messages. Key material never appears in repr()."""
    did: str
    signing_key: Optional[bytes] = field(default=None, repr=False)  # Ed25519 private seed (32 bytes)
    verify_key: Optional[bytes] = field(default=None, repr=False)   # Ed25519 public key (32 bytes)


@dataclass
class BridgeSession:
    """Session state for bridge connections.

    ``sequence`` defaults to the current Unix time in microseconds minus one, so the
    first signed envelope uses a time-based sequence that stays monotonic across
    restarts. ``replay`` and ``timestamps`` are used when verifying inbound envelopes
    addressed to ``local_identity.did``.
    """
    local_identity: BridgeIdentity
    remote_did: Optional[str] = None
    profile: int = PROFILE_SECURE_BASELINE
    sequence: Optional[int] = None
    negotiated: bool = False
    allow_transport_only: bool = False
    replay: ReplayProtector = field(default_factory=ReplayProtector, repr=False, compare=False)
    timestamps: TimestampValidator = field(default_factory=TimestampValidator, repr=False, compare=False)
    _sequence_lock: Lock = field(default_factory=Lock, repr=False, compare=False)

    def __post_init__(self) -> None:
        if self.sequence is None:
            self.sequence = now_us() - 1
        self.replay.apply_timestamp_policy(self.timestamps)

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


def _session_profile(session: Optional[BridgeSession], allow_transport_only: bool) -> int:
    if session is None:
        if not allow_transport_only:
            raise ValueError(
                "secure-baseline requires a configured session; pass allow_transport_only=True "
                "to emit unauthenticated transport-only envelopes"
            )
        return PROFILE_TRANSPORT_ONLY
    if session.profile not in (PROFILE_TRANSPORT_ONLY, PROFILE_SECURE_BASELINE):
        raise ValueError("unsupported bridge profile")
    if not session.negotiated:
        raise ValueError("bridge session must be explicitly configured before conversion")
    if session.profile == PROFILE_TRANSPORT_ONLY and not (session.allow_transport_only or allow_transport_only):
        raise ValueError("transport-only profile requires an explicit allow_transport_only opt-in")
    return session.profile


def mcp_to_axcp(
    mcp_json: dict,
    session: Optional[BridgeSession] = None,
    *,
    allow_transport_only: bool = False,
) -> axcp.AxcpEnvelope:
    """
    Convert MCP JSON request to AXCP Envelope.

    Secure-baseline (the default) requires a configured session and signs the envelope.
    Transport-only envelopes are produced only with an explicit opt-in.
    """
    profile = _session_profile(session, allow_transport_only)

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

    if profile == PROFILE_SECURE_BASELINE:
        sign_envelope(session, env)

    return env


class _MappingResolver:
    """Adapt a DID -> public key mapping to the SDK resolver interface."""

    def __init__(self, keys: Mapping[str, bytes]) -> None:
        self._keys = keys

    def resolve(self, did: str):
        from axcp.errors import DIDResolutionError
        from axcp.identity import DIDDocument, PublicKeyRecord

        key = self._keys.get(did)
        if key is None:
            raise DIDResolutionError("DID not found")
        return DIDDocument(id=did, public_keys=(PublicKeyRecord("key-1", "Ed25519VerificationKey2020", bytes(key)),))


def _as_resolver(resolver: BridgeResolver) -> Optional[DIDResolver]:
    if resolver is None or hasattr(resolver, "resolve"):
        return resolver  # type: ignore[return-value]
    return _MappingResolver(resolver)


def verify_envelope_signature(
    env: axcp.AxcpEnvelope,
    resolver: BridgeResolver,
    session: Optional[BridgeSession] = None,
) -> bool:
    """Verify an inbound envelope. Returns True if valid, False otherwise.

    ``resolver`` is an SDK DIDResolver or a mapping DID -> Ed25519 public key; it is
    consulted for every sender (including did:key). With a ``session`` the full Secure
    Baseline checks run: recipient == ``session.local_identity.did``, signature,
    timestamp window and replay (state updated only after a valid signature). Without a
    session only the envelope structure and signature are checked, which does NOT
    protect against replay or cross-recipient reuse.
    """
    try:
        if session is None:
            sdk_verify_envelope(
                env,
                expected_recipient_did=env.recipient_did,
                resolver=_as_resolver(resolver),
            )
            return True
        sdk_verify_envelope(
            env,
            expected_recipient_did=session.local_identity.did,
            resolver=_as_resolver(resolver),
        )
        session.timestamps.validate(env.timestamp_ms)
        session.replay.check_and_mark(env.sender_did, env.sequence)
    except AXCPError:
        return False
    return True


def axcp_to_mcp(
    env: axcp.AxcpEnvelope,
    session: Optional[BridgeSession] = None,
    resolver: BridgeResolver = None,
) -> dict:
    """
    Convert AXCP Envelope to MCP JSON response.

    Very narrow mapping: ContextPatch -> MCP context_delta.

    ``_auth`` metadata is emitted only for envelopes that passed full verification
    against ``session`` (secure-baseline). With a secure-baseline session an envelope
    that fails verification raises ``ValueError``; without a session no ``_auth`` is
    ever emitted.
    """
    verified = False
    if session is not None and session.profile == PROFILE_SECURE_BASELINE:
        if not verify_envelope_signature(env, resolver, session):
            raise ValueError("envelope failed AXCP secure-baseline verification")
        verified = True

    patch = env.context_patch
    delta_b64 = base64.b64encode(patch.SerializeToString()).decode()

    result = {
        "id": env.trace_id,
        "context_delta": delta_b64,
        "ts": int(time.time() * 1000)
    }

    if verified:
        result["_auth"] = {
            "sender_did": env.sender_did,
            "timestamp_ms": env.timestamp_ms,
            "sequence": env.sequence,
            "profile": "secure-baseline-v1",
        }

    return result


def main():
    import argparse

    parser = argparse.ArgumentParser(description='MCP <-> AXCP Bridge')
    parser.add_argument('input_file', help='MCP JSON input file')
    parser.add_argument('--profile', choices=['transport-only', 'secure-baseline'],
                        default='secure-baseline', help='Security profile to use (default: secure-baseline)')
    parser.add_argument('--allow-transport-only', action='store_true',
                        help='Explicitly allow the unauthenticated transport-only profile')
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

    if args.profile == 'secure-baseline':
        if not args.remote_did:
            parser.error("--remote-did is required for secure-baseline")
        session = BridgeSession(
            local_identity=identity,
            remote_did=args.remote_did,
            profile=PROFILE_SECURE_BASELINE,
            negotiated=True,
        )
        print("Profile: secure-baseline-v1")
        print(f"Remote DID: {session.remote_did}")
    else:
        if not args.allow_transport_only:
            parser.error("transport-only is unauthenticated; pass --allow-transport-only to use it")
        session = BridgeSession(
            local_identity=identity,
            profile=PROFILE_TRANSPORT_ONLY,
            negotiated=True,
            allow_transport_only=True,
        )
        print("Profile: transport-only-v0 (UNAUTHENTICATED)")

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

    # Round-trip demo: outbound envelopes are not verified here, so no _auth is emitted.
    back = axcp_to_mcp(env)
    print(f"\nBack to MCP JSON:")
    print(json.dumps(back, indent=2))


if __name__ == "__main__":
    main()
