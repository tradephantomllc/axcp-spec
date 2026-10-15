"""Timestamp helpers for AXCP replay protection."""

from __future__ import annotations

import dataclasses
import time
from datetime import datetime, timedelta, timezone

from axcp.errors import (
    TimestampExpiredError,
    TimestampFutureError,
    TimestampInvalidError,
    TimestampMissingError,
)

DEFAULT_MAX_TIMESTAMP_AGE = timedelta(minutes=5)
DEFAULT_CLOCK_SKEW = timedelta(seconds=30)
MAX_TIMESTAMP_MS = 253_402_300_799_999
"""Largest accepted timestamp: 9999-12-31T23:59:59.999Z (D8)."""

_EPOCH = datetime(1970, 1, 1, tzinfo=timezone.utc)


def now_ms() -> int:
    return time.time_ns() // 1_000_000


def now_us() -> int:
    """Current Unix time in microseconds (time-based sequence allocation, D7)."""

    return time.time_ns() // 1_000


def _check_range(timestamp_ms: int) -> int:
    if isinstance(timestamp_ms, bool) or not isinstance(timestamp_ms, int):
        raise TimestampInvalidError("timestamp must be an integer number of milliseconds")
    if timestamp_ms < 0 or timestamp_ms > MAX_TIMESTAMP_MS:
        raise TimestampInvalidError("timestamp is out of range")
    return timestamp_ms


def datetime_from_ms(timestamp_ms: int) -> datetime:
    """Convert milliseconds since the Unix epoch to an aware UTC datetime.

    Out-of-range values raise :class:`TimestampInvalidError` (never ``OverflowError``,
    ``ValueError`` or ``OSError``).
    """

    return _EPOCH + timedelta(milliseconds=_check_range(timestamp_ms))


def rfc3339_seconds_from_ms(timestamp_ms: int) -> str:
    return datetime_from_ms(timestamp_ms).strftime("%Y-%m-%dT%H:%M:%SZ")


@dataclasses.dataclass
class TimestampValidator:
    max_age: timedelta = DEFAULT_MAX_TIMESTAMP_AGE
    clock_skew: timedelta = DEFAULT_CLOCK_SKEW

    def __post_init__(self) -> None:
        if self.max_age <= timedelta(0):
            raise TimestampMissingError("max_age must be positive")
        if self.clock_skew < timedelta(0):
            raise TimestampMissingError("clock_skew cannot be negative")

    def validate(self, timestamp_ms: int, now: datetime | None = None) -> None:
        if timestamp_ms == 0:
            raise TimestampMissingError("timestamp is missing or zero")

        current = now or datetime.now(tz=timezone.utc)
        msg_time = datetime_from_ms(timestamp_ms)
        if msg_time < current - self.max_age:
            raise TimestampExpiredError("timestamp expired")
        if msg_time > current + self.clock_skew:
            raise TimestampFutureError("timestamp is too far in the future")

    def is_expired(self, timestamp_ms: int, now: datetime | None = None) -> bool:
        try:
            self.validate(timestamp_ms, now=now)
        except TimestampExpiredError:
            return True
        return False
