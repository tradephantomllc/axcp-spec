#!/usr/bin/env python3
"""
Very small sanity-check: validate envelope-vs-session profile rules.
Run:  python scripts/test_profile.py
"""
from dataclasses import dataclass

@dataclass
class Envelope:
    profile: int

def validate(envelope: Envelope, session_profile: int) -> bool:
    """Return True if envelope allowed under session_profile."""
    return envelope.profile <= session_profile

def test():
    assert validate(Envelope(0), 2) is True
    assert validate(Envelope(2), 2) is True
    assert validate(Envelope(3), 2) is False  # should raise PROFILE_MISMATCH
    print("All profile tests passed.")

if __name__ == "__main__":
    test()
