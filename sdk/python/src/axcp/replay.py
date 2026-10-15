"""Thread-safe per-sender replay protection (bitmap sliding window, D6)."""

from __future__ import annotations

import threading
from collections import OrderedDict
from datetime import datetime, timedelta, timezone

from axcp.errors import (
    InvalidReplayConfigError,
    ReplayCapacityError,
    ReplayDetectedError,
    SequenceTooOldError,
)

DEFAULT_REPLAY_WINDOW = 1000
DEFAULT_PEER_TTL = timedelta(minutes=10)
DEFAULT_MAX_PEERS = 100_000
DEFAULT_MAX_NONCES = 1_000_000
MAX_SEQUENCE = (1 << 64) - 1
_MAX_WINDOW = 1 << 16
_CLEANUP_BATCH = 16


class _PeerState:
    __slots__ = ("highest", "bitmap", "last_seen")

    def __init__(self, highest: int, last_seen: datetime) -> None:
        self.highest = highest
        # bit i set <=> sequence (highest - i) has been seen, for 0 <= i < window
        self.bitmap = 1
        self.last_seen = last_seen


class ReplayProtector:
    """Per-sender replay guard with a bitmap sliding window (D6).

    * first message from a sender is accepted and becomes the highest sequence;
    * a higher sequence advances the window;
    * a sequence inside the window is rejected if already seen;
    * a sequence at or below ``highest - window`` is rejected as too old.

    Seen sequences never expire while a sender is active. Idle senders are evicted after
    ``peer_ttl`` (amortized, oldest first). With ``max_peers`` active senders, new
    senders are rejected with :class:`ReplayCapacityError` (fail closed).

    Call :meth:`check_and_mark` only after the envelope signature has been verified.
    """

    def __init__(
        self,
        ttl: timedelta | None = None,
        window: int = DEFAULT_REPLAY_WINDOW,
        *,
        peer_ttl: timedelta | None = None,
        max_peers: int = DEFAULT_MAX_PEERS,
        max_nonces: int = DEFAULT_MAX_NONCES,
        timestamp_policy: object | None = None,
    ) -> None:
        """Create a replay protector.

        ``ttl`` is a deprecated alias of ``peer_ttl``. ``timestamp_policy`` is any object
        with ``max_age`` and ``clock_skew`` timedeltas (e.g. :class:`TimestampValidator`);
        the peer TTL is raised to at least ``max_age + clock_skew``.
        """

        effective_ttl = peer_ttl if peer_ttl is not None else ttl
        if effective_ttl is None:
            effective_ttl = DEFAULT_PEER_TTL
        if not isinstance(effective_ttl, timedelta) or effective_ttl <= timedelta(0):
            raise InvalidReplayConfigError("peer_ttl must be a positive timedelta")
        if isinstance(window, bool) or not isinstance(window, int) or not 1 <= window <= _MAX_WINDOW:
            raise InvalidReplayConfigError(f"window must be an integer between 1 and {_MAX_WINDOW}")
        if isinstance(max_peers, bool) or not isinstance(max_peers, int) or max_peers < 1:
            raise InvalidReplayConfigError("max_peers must be a positive integer")
        if isinstance(max_nonces, bool) or not isinstance(max_nonces, int) or max_nonces < 1:
            raise InvalidReplayConfigError("max_nonces must be a positive integer")
        self._peer_ttl = effective_ttl
        self._window = window
        self._mask = (1 << window) - 1
        self._max_peers = max_peers
        self._max_nonces = max_nonces
        # Ordered by last activity: touched peers move to the end.
        self._peers: OrderedDict[str, _PeerState] = OrderedDict()
        # Ordered by first-seen time; entries are never refreshed.
        self._nonces: OrderedDict[tuple[str, str], datetime] = OrderedDict()
        self._lock = threading.Lock()
        if timestamp_policy is not None:
            self.apply_timestamp_policy(timestamp_policy)

    @property
    def peer_ttl(self) -> timedelta:
        return self._peer_ttl

    @property
    def window(self) -> int:
        return self._window

    @property
    def max_peers(self) -> int:
        return self._max_peers

    @property
    def tracked_peers(self) -> int:
        with self._lock:
            return len(self._peers)

    def apply_timestamp_policy(self, policy: object) -> None:
        """Raise the peer TTL to at least ``policy.max_age + policy.clock_skew``."""

        max_age = getattr(policy, "max_age", None)
        clock_skew = getattr(policy, "clock_skew", None)
        if not isinstance(max_age, timedelta) or not isinstance(clock_skew, timedelta):
            raise InvalidReplayConfigError("timestamp policy needs max_age and clock_skew timedeltas")
        minimum = max_age + clock_skew
        if minimum <= timedelta(0):
            raise InvalidReplayConfigError("timestamp policy must have a positive max age")
        with self._lock:
            if self._peer_ttl < minimum:
                self._peer_ttl = minimum

    def check_and_mark(self, peer_id: str, sequence: int, now: datetime | None = None) -> None:
        if not isinstance(peer_id, str) or not peer_id:
            raise InvalidReplayConfigError("peer_id cannot be empty")
        if isinstance(sequence, bool) or not isinstance(sequence, int) or not 1 <= sequence <= MAX_SEQUENCE:
            raise InvalidReplayConfigError("sequence must be an integer between 1 and 2^64-1")

        current = now or datetime.now(tz=timezone.utc)
        with self._lock:
            self._evict_idle_peers(current, _CLEANUP_BATCH)
            state = self._peers.get(peer_id)
            if state is not None and current - state.last_seen > self._peer_ttl:
                del self._peers[peer_id]
                state = None

            if state is None:
                if len(self._peers) >= self._max_peers:
                    self._evict_idle_peers(current, None)
                    if len(self._peers) >= self._max_peers:
                        raise ReplayCapacityError("replay capacity exceeded")
                self._peers[peer_id] = _PeerState(sequence, current)
                return

            if sequence > state.highest:
                shift = sequence - state.highest
                state.bitmap = 1 if shift >= self._window else ((state.bitmap << shift) | 1) & self._mask
                state.highest = sequence
            elif sequence > state.highest - self._window:
                bit = 1 << (state.highest - sequence)
                if state.bitmap & bit:
                    raise ReplayDetectedError("sequence already seen")
                state.bitmap |= bit
            else:
                raise SequenceTooOldError("sequence outside replay window")

            if current > state.last_seen:
                state.last_seen = current
            self._peers.move_to_end(peer_id)

    def check_and_mark_nonce(self, peer_id: str, nonce: str, now: datetime | None = None) -> None:
        if not isinstance(peer_id, str) or not peer_id:
            raise InvalidReplayConfigError("peer_id cannot be empty")
        if not isinstance(nonce, str) or not nonce:
            raise InvalidReplayConfigError("nonce cannot be empty")

        current = now or datetime.now(tz=timezone.utc)
        key = (peer_id, nonce)
        with self._lock:
            self._evict_expired_nonces(current, _CLEANUP_BATCH)
            seen_at = self._nonces.get(key)
            if seen_at is not None:
                if current - seen_at <= self._peer_ttl:
                    raise ReplayDetectedError("nonce already seen")
                del self._nonces[key]
            if len(self._nonces) >= self._max_nonces:
                self._evict_expired_nonces(current, None)
                if len(self._nonces) >= self._max_nonces:
                    raise ReplayCapacityError("nonce replay capacity exceeded")
            self._nonces[key] = current

    def _evict_idle_peers(self, now: datetime, limit: int | None) -> None:
        evicted = 0
        while self._peers and (limit is None or evicted < limit):
            peer_id, state = next(iter(self._peers.items()))
            if now - state.last_seen <= self._peer_ttl:
                return
            del self._peers[peer_id]
            evicted += 1

    def _evict_expired_nonces(self, now: datetime, limit: int | None) -> None:
        evicted = 0
        while self._nonces and (limit is None or evicted < limit):
            key, seen_at = next(iter(self._nonces.items()))
            if now - seen_at <= self._peer_ttl:
                return
            del self._nonces[key]
            evicted += 1
