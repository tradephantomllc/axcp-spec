#!/usr/bin/env python3
"""
Sanity-check: validate envelope-vs-session profile rules.
Run:  python scripts/test_profile.py
"""
from dataclasses import dataclass

@dataclass
class Envelope:
    profile: int

def validate(envelope: Envelope, session_profile: int, supported_mask: int) -> int:
    """
    Return error code:
    0 = ok
    12 = PROFILE_MISMATCH
    13 = PROFILE_UNSUPPORTED
    """
    if envelope.profile > session_profile:
        return 12  # PROFILE_MISMATCH
    if not (1 << envelope.profile) & supported_mask:
        return 13  # PROFILE_UNSUPPORTED
    return 0

def test():
    supported_mask = 0b0111  # supports profiles 0–2
    assert validate(Envelope(1), 2, supported_mask) == 0
    assert validate(Envelope(3), 2, supported_mask) == 12  # mismatch
    assert validate(Envelope(2), 2, 0b0011) == 13  # unsupported
    print("✅ All profile validation tests passed.")

if __name__ == "__main__":
    test()
