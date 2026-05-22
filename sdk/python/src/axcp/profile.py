"""AXCP Secure Baseline profile negotiation primitives."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum

from axcp.errors import (
    DeprecatedProfileError,
    InvalidCapabilitiesError,
    NegotiationError,
    NoProfileOverlapError,
    NoSignatureAlgorithmOverlapError,
)


class Profile(str, Enum):
    SECURE_BASELINE = "secure-baseline-v1"
    TRANSPORT_ONLY = "transport-only-v0"


PROTOCOL_VERSION_V1 = "1"
SIG_ALGO_ED25519 = "ed25519"


@dataclass(frozen=True)
class Capabilities:
    require_auth: bool = True
    require_replay: bool = True
    sig_algos: tuple[str, ...] = (SIG_ALGO_ED25519,)


@dataclass(frozen=True)
class ClientHello:
    version: str = PROTOCOL_VERSION_V1
    profiles: tuple[Profile, ...] = (Profile.SECURE_BASELINE,)
    capabilities: Capabilities = field(default_factory=Capabilities)


@dataclass(frozen=True)
class ServerHello:
    version: str
    profile: Profile
    capabilities: Capabilities


def negotiate(
    client: ClientHello,
    *,
    server_profiles: tuple[Profile, ...] = (Profile.SECURE_BASELINE,),
    server_capabilities: Capabilities = Capabilities(),
    allow_deprecated: bool = False,
) -> ServerHello:
    if client.version != PROTOCOL_VERSION_V1:
        raise NegotiationError("unsupported protocol version")
    if not client.profiles:
        raise NoProfileOverlapError("client did not provide profiles")

    selected = _select_profile(client.profiles, server_profiles, allow_deprecated)
    _validate_capabilities(selected, client.capabilities)
    selected_algo = _resolve_sig_algo(client.capabilities.sig_algos, server_capabilities.sig_algos)
    return ServerHello(
        version=PROTOCOL_VERSION_V1,
        profile=selected,
        capabilities=Capabilities(
            require_auth=client.capabilities.require_auth or _profile_requires_auth(selected),
            require_replay=client.capabilities.require_replay or _profile_requires_replay(selected),
            sig_algos=(selected_algo,),
        ),
    )


def _select_profile(
    client_profiles: tuple[Profile, ...],
    server_profiles: tuple[Profile, ...],
    allow_deprecated: bool,
) -> Profile:
    server_set = set(server_profiles)
    if Profile.SECURE_BASELINE in client_profiles and Profile.SECURE_BASELINE in server_set:
        return Profile.SECURE_BASELINE
    if Profile.TRANSPORT_ONLY in client_profiles and Profile.TRANSPORT_ONLY in server_set:
        if not allow_deprecated:
            raise DeprecatedProfileError("transport-only-v0 is deprecated")
        return Profile.TRANSPORT_ONLY
    raise NoProfileOverlapError("no common AXCP profile")


def _validate_capabilities(profile: Profile, capabilities: Capabilities) -> None:
    if profile == Profile.SECURE_BASELINE:
        if not capabilities.sig_algos:
            raise InvalidCapabilitiesError("secure-baseline-v1 requires a signature algorithm")
        return
    if profile == Profile.TRANSPORT_ONLY:
        return
    raise NoProfileOverlapError(f"unknown AXCP profile: {profile}")


def _resolve_sig_algo(client_algos: tuple[str, ...], server_algos: tuple[str, ...]) -> str:
    for algo in (SIG_ALGO_ED25519,):
        if algo in client_algos and algo in server_algos:
            return algo
    raise NoSignatureAlgorithmOverlapError("no common signature algorithm")


def _profile_requires_auth(profile: Profile) -> bool:
    return profile == Profile.SECURE_BASELINE


def _profile_requires_replay(profile: Profile) -> bool:
    return profile == Profile.SECURE_BASELINE
