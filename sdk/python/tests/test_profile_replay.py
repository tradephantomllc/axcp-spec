from datetime import datetime, timedelta, timezone

import pytest

from axcp.errors import DeprecatedProfileError, NoProfileOverlapError, ReplayDetectedError, SequenceTooOldError
from axcp.profile import Capabilities, ClientHello, Profile, negotiate
from axcp.replay import ReplayProtector


def test_secure_baseline_negotiation() -> None:
    hello = ClientHello()
    result = negotiate(hello)

    assert result.version == "1"
    assert result.profile == Profile.SECURE_BASELINE
    assert result.capabilities == Capabilities(sig_algos=("ed25519",))


def test_transport_only_rejected_by_default() -> None:
    hello = ClientHello(profiles=(Profile.TRANSPORT_ONLY,))

    with pytest.raises(DeprecatedProfileError):
        negotiate(hello, server_profiles=(Profile.TRANSPORT_ONLY,))


def test_no_profile_overlap() -> None:
    hello = ClientHello(profiles=(Profile.SECURE_BASELINE,))

    with pytest.raises(NoProfileOverlapError):
        negotiate(hello, server_profiles=())


def test_replay_sequence_window() -> None:
    replay = ReplayProtector(ttl=timedelta(minutes=1), window=5)
    now = datetime(2026, 1, 1, tzinfo=timezone.utc)

    replay.check_and_mark("peer", 100, now=now)
    replay.check_and_mark("peer", 96, now=now)

    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", 96, now=now)
    with pytest.raises(SequenceTooOldError):
        replay.check_and_mark("peer", 95, now=now)


def test_replay_nonce_ttl_cleanup() -> None:
    replay = ReplayProtector(ttl=timedelta(seconds=1), window=5)
    now = datetime(2026, 1, 1, tzinfo=timezone.utc)

    replay.check_and_mark_nonce("peer", "nonce", now=now)
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark_nonce("peer", "nonce", now=now)

    replay.check_and_mark_nonce("peer", "nonce", now=now + timedelta(seconds=2))
