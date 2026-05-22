"""Thread-safe sequence and nonce replay protection."""

from __future__ import annotations

import dataclasses
import threading
from collections import defaultdict
from datetime import datetime, timedelta, timezone

from axcp.errors import InvalidReplayConfigError, ReplayDetectedError, SequenceTooOldError


@dataclasses.dataclass
class _PeerWindow:
    highest: int = 0
    seen: dict[int, datetime] = dataclasses.field(default_factory=dict)


class ReplayProtector:
    """Sequence and nonce replay guard with TTL cleanup."""

    def __init__(self, ttl: timedelta = timedelta(minutes=5), window: int = 64) -> None:
        if ttl <= timedelta(0):
            raise InvalidReplayConfigError("ttl must be positive")
        if window < 1:
            raise InvalidReplayConfigError("window must be at least 1")
        self._ttl = ttl
        self._window = window
        self._peers: dict[str, _PeerWindow] = {}
        self._nonces: dict[str, dict[str, datetime]] = defaultdict(dict)
        self._lock = threading.Lock()

    def check_and_mark(self, peer_id: str, sequence: int, now: datetime | None = None) -> None:
        if not peer_id:
            raise InvalidReplayConfigError("peer_id cannot be empty")
        if sequence < 0:
            raise InvalidReplayConfigError("sequence cannot be negative")

        current = now or datetime.now(tz=timezone.utc)
        with self._lock:
            window = self._peers.setdefault(peer_id, _PeerWindow())
            self._cleanup_sequences(window, current)

            if sequence in window.seen:
                raise ReplayDetectedError("sequence already seen")

            if window.highest > 0:
                min_acceptable = (
                    window.highest - self._window + 1
                    if window.highest >= self._window
                    else 0
                )
                if sequence < min_acceptable:
                    raise SequenceTooOldError("sequence outside replay window")

            window.seen[sequence] = current
            if sequence > window.highest:
                window.highest = sequence

    def check_and_mark_nonce(self, peer_id: str, nonce: str, now: datetime | None = None) -> None:
        if not peer_id:
            raise InvalidReplayConfigError("peer_id cannot be empty")
        if not nonce:
            raise InvalidReplayConfigError("nonce cannot be empty")

        current = now or datetime.now(tz=timezone.utc)
        with self._lock:
            peer_nonces = self._nonces[peer_id]
            self._cleanup_nonces(peer_nonces, current)
            if nonce in peer_nonces:
                raise ReplayDetectedError("nonce already seen")
            peer_nonces[nonce] = current

    def _cleanup_sequences(self, window: _PeerWindow, now: datetime) -> None:
        cutoff = now - self._ttl
        expired = [sequence for sequence, seen_at in window.seen.items() if seen_at < cutoff]
        for sequence in expired:
            del window.seen[sequence]

    def _cleanup_nonces(self, nonces: dict[str, datetime], now: datetime) -> None:
        cutoff = now - self._ttl
        expired = [nonce for nonce, seen_at in nonces.items() if seen_at < cutoff]
        for nonce in expired:
            del nonces[nonce]
