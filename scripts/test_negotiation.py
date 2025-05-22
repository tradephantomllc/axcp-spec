#!/usr/bin/env python3
from dataclasses import dataclass

@dataclass
class Neg:
    supported: int  # bitmask
    minimum: int

def negotiate(a: Neg, b: Neg) -> int | None:
    intersection = a.supported & b.supported
    required = max(a.minimum, b.minimum)
    highest = max([i for i in range(4) if intersection & (1 << i)], default=-1)
    return highest if highest >= required else None

def test():
    assert negotiate(Neg(0b1111, 1), Neg(0b1011, 0)) == 2
    assert negotiate(Neg(0b0001, 0), Neg(0b0100, 0)) is None
    assert negotiate(Neg(0b0011, 2), Neg(0b0110, 1)) == 2
    print("negotiation tests passed")

if __name__ == "__main__":
    test()
