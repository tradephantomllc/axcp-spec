import time
from datetime import datetime, timedelta, timezone

import pytest

from axcp import Agent, Identity
from axcp.envelope import encode_envelope, new_envelope, sign_envelope
from axcp.errors import (
    DeprecatedProfileError,
    InvalidEnvelopeError,
    InvalidReplayConfigError,
    NoProfileOverlapError,
    ReplayCapacityError,
    ReplayDetectedError,
    SequenceTooOldError,
)
from axcp.pb import axcp_pb2
from axcp.profile import Capabilities, ClientHello, Profile, negotiate
from axcp.replay import DEFAULT_PEER_TTL, ReplayProtector
from axcp.timestamp import TimestampValidator, now_us

T0 = datetime(2026, 1, 1, tzinfo=timezone.utc)
MINUTE = timedelta(minutes=1)


def _context_envelope():
    env = new_envelope(trace_id="replay")
    env.context_patch.CopyFrom(axcp_pb2.ContextPatch(context_id="ctx", base_version=1))
    return env


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


# --- D6 ---------------------------------------------------------------------


def test_replay_sequence_window() -> None:
    replay = ReplayProtector(window=5)

    replay.check_and_mark("peer", 100, now=T0)
    replay.check_and_mark("peer", 96, now=T0)
    replay.check_and_mark("peer", 99, now=T0)

    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", 96, now=T0)
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", 100, now=T0)
    with pytest.raises(SequenceTooOldError):
        replay.check_and_mark("peer", 95, now=T0)
    replay.check_and_mark("peer", 97, now=T0)


def test_replay_accepts_any_first_sequence_and_large_gaps() -> None:
    replay = ReplayProtector(window=64)
    base = 1_700_000_000_000_000
    replay.check_and_mark("peer", base, now=T0)
    replay.check_and_mark("peer", base + 100, now=T0)
    with pytest.raises(SequenceTooOldError):
        replay.check_and_mark("peer", base, now=T0)
    replay.check_and_mark("peer", base + 99, now=T0)
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", base + 99, now=T0)
    replay.check_and_mark("peer", base + 98, now=T0)
    replay.check_and_mark("peer", (1 << 64) - 1, now=T0)


def test_replay_rejected_while_sender_stays_active_beyond_old_ttl() -> None:
    replay = ReplayProtector()
    replay.check_and_mark("peer", 10, now=T0)
    for step in range(1, 7):
        replay.check_and_mark("peer", 10 + step, now=T0 + step * 8 * MINUTE)
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", 10, now=T0 + 48 * MINUTE)


def test_default_peer_ttl_and_idle_eviction() -> None:
    replay = ReplayProtector()
    assert replay.peer_ttl == DEFAULT_PEER_TTL == timedelta(minutes=10)
    replay.check_and_mark("peer", 50, now=T0)
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", 50, now=T0 + 10 * MINUTE)
    replay.check_and_mark("peer", 50, now=T0 + 20 * MINUTE + timedelta(milliseconds=1))


def test_peer_ttl_is_raised_to_max_age_plus_skew() -> None:
    policy = TimestampValidator(max_age=5 * MINUTE, clock_skew=timedelta(seconds=30))
    replay = ReplayProtector(peer_ttl=MINUTE, timestamp_policy=policy)
    assert replay.peer_ttl == 5 * MINUTE + timedelta(seconds=30)
    replay.check_and_mark("peer", 1, now=T0)
    # An envelope dated +25 s is still timestamp-valid at T+5m10s: still a replay.
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark("peer", 1, now=T0 + 5 * MINUTE + timedelta(seconds=10))


def test_agent_raises_ttl_of_supplied_replay_protector() -> None:
    replay = ReplayProtector(peer_ttl=timedelta(seconds=1))
    agent = Agent(
        Identity.generate(),
        replay=replay,
        timestamps=TimestampValidator(max_age=2 * MINUTE, clock_skew=timedelta(seconds=10)),
    )
    assert agent.replay.peer_ttl == 2 * MINUTE + timedelta(seconds=10)


def test_replay_capacity_fails_closed() -> None:
    replay = ReplayProtector(peer_ttl=MINUTE, max_peers=2)
    replay.check_and_mark("a", 1, now=T0)
    replay.check_and_mark("b", 1, now=T0)
    with pytest.raises(ReplayCapacityError):
        replay.check_and_mark("c", 1, now=T0 + timedelta(seconds=1))
    replay.check_and_mark("a", 2, now=T0 + timedelta(seconds=1))
    replay.check_and_mark("c", 1, now=T0 + MINUTE + timedelta(milliseconds=500))
    assert replay.tracked_peers == 2
    with pytest.raises(ReplayCapacityError):
        replay.check_and_mark("d", 1, now=T0 + MINUTE + timedelta(milliseconds=600))


def test_idle_eviction_is_amortized() -> None:
    replay = ReplayProtector(peer_ttl=MINUTE)
    for index in range(100):
        replay.check_and_mark(f"peer-{index}", 1, now=T0)
    replay.check_and_mark("fresh", 1, now=T0 + 2 * MINUTE)
    assert 1 < replay.tracked_peers < 101
    for index in range(20):
        replay.check_and_mark("fresh", 2 + index, now=T0 + 2 * MINUTE)
    assert replay.tracked_peers == 1


def test_replay_configuration_and_inputs_are_validated() -> None:
    with pytest.raises(InvalidReplayConfigError):
        ReplayProtector(window=0)
    with pytest.raises(InvalidReplayConfigError):
        ReplayProtector(peer_ttl=timedelta(0))
    with pytest.raises(InvalidReplayConfigError):
        ReplayProtector(max_peers=0)
    replay = ReplayProtector()
    with pytest.raises(InvalidReplayConfigError):
        replay.check_and_mark("", 1)
    with pytest.raises(InvalidReplayConfigError):
        replay.check_and_mark("peer", 0)
    with pytest.raises(InvalidReplayConfigError):
        replay.check_and_mark("peer", 1 << 64)


def test_replay_nonce_ttl_cleanup_and_bound() -> None:
    replay = ReplayProtector(ttl=timedelta(seconds=1), window=5, max_nonces=2)

    replay.check_and_mark_nonce("peer", "nonce", now=T0)
    with pytest.raises(ReplayDetectedError):
        replay.check_and_mark_nonce("peer", "nonce", now=T0)
    replay.check_and_mark_nonce("peer", "nonce-2", now=T0)
    with pytest.raises(ReplayCapacityError):
        replay.check_and_mark_nonce("peer", "nonce-3", now=T0)

    replay.check_and_mark_nonce("peer", "nonce", now=T0 + timedelta(seconds=2))


# --- D7 ---------------------------------------------------------------------


def test_agent_sequence_starts_at_unix_microseconds() -> None:
    before = now_us()
    alice = Agent(Identity.generate())
    bob = Identity.generate()
    first = alice.sign_message(_context_envelope(), recipient_did=bob.did).sequence
    after = now_us()
    assert before <= first <= after
    assert alice.sign_message(_context_envelope(), recipient_did=bob.did).sequence == first + 1


def test_agent_honours_explicit_initial_sequence() -> None:
    alice = Agent(Identity.generate(), initial_sequence=42)
    assert alice.sign_message(_context_envelope(), recipient_did=Identity.generate().did).sequence == 42
    with pytest.raises(InvalidEnvelopeError):
        Agent(Identity.generate(), initial_sequence=0)


def test_low_level_signing_never_signs_sequence_zero() -> None:
    alice = Identity.generate()
    bob = Identity.generate()
    with pytest.raises(InvalidEnvelopeError, match="sequence"):
        sign_envelope(_context_envelope(), alice, recipient_did=bob.did)
    with pytest.raises(InvalidEnvelopeError, match="sequence"):
        sign_envelope(_context_envelope(), alice, recipient_did=bob.did, sequence=0)
    preset = _context_envelope()
    preset.sequence = 9
    assert sign_envelope(preset, alice, recipient_did=bob.did).sequence == 9


def test_restarted_sender_is_accepted_with_time_based_sequences() -> None:
    identity = Identity.generate()
    bob = Agent(Identity.generate())
    first_run = Agent(identity)
    for _ in range(3):
        bob.verify_bytes(encode_envelope(first_run.sign_message(_context_envelope(), recipient_did=bob.identity.did)))
    time.sleep(0.002)
    restarted = Agent(identity)
    bob.verify_bytes(encode_envelope(restarted.sign_message(_context_envelope(), recipient_did=bob.identity.did)))

    legacy = Agent(identity, initial_sequence=1)  # the old counter that restarted at 1
    with pytest.raises(SequenceTooOldError):
        bob.verify_bytes(encode_envelope(legacy.sign_message(_context_envelope(), recipient_did=bob.identity.did)))
