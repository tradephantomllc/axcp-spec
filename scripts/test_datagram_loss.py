#!/usr/bin/env python3
import random, statistics as st

LOST = 0
RECV = 1

def simulate(loss_rate=0.1, packets=1000):
    seq = 0
    delivered = []
    for _ in range(packets):
        seq += 1
        if random.random() < loss_rate:
            continue           # drop
        delivered.append(seq)
    gaps = sum(1 for i in range(len(delivered)-1)
               if delivered[i+1] != delivered[i]+1)
    return gaps, len(delivered)

def test():
    gaps, recv = simulate()
    assert recv > 800            # >= 20 % loss would be unusual
    print(f"received={recv}, gaps={gaps} (OK)")

if __name__ == "__main__":
    test()
