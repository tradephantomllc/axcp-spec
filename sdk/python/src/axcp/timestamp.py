"""Timestamp helpers for AXCP replay protection."""

from __future__ import annotations

import dataclasses
from datetime import datetime, timedelta, timezone

from axcp.errors import TimestampExpiredError, TimestampFutureError, TimestampMissingError

DEFAULT_MAX_TIMESTAMP_AGE = timedelta(minutes=5)
DEFAULT_CLOCK_SKEW = timedelta(seconds=30)


def now_ms() -> int:
    return int(datetime.now(tz=timezone.utc).timestamp() * 1000)


def datetime_from_ms(timestamp_ms: int) -> datetime:
    return datetime.fromtimestamp(timestamp_ms / 1000, tz=timezone.utc)


def rfc3339_seconds_from_ms(timestamp_ms: int) -> str:
    return datetime_from_ms(timestamp_ms).strftime("%Y-%m-%dT%H:%M:%SZ")


@dataclasses.dataclass
class TimestampValidator:
    max_age: timedelta = DEFAULT_MAX_TIMESTAMP_AGE
    clock_skew: timedelta = DEFAULT_CLOCK_SKEW

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
