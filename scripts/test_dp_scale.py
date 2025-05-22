#!/usr/bin/env python3
import math

def laplace_scale(epsilon, clip):
    return clip / epsilon

def gaussian_sigma(epsilon, delta, clip):
    return math.sqrt(2*math.log(1.25/delta)) * clip / epsilon

def test():
    assert abs(laplace_scale(1.0, 2.0) - 2.0) < 1e-6
    σ = gaussian_sigma(1.0, 1e-5, 2.0)
    assert σ > 2.0            # should be bigger than Laplace scale
    print("DP scale tests OK")

if __name__ == "__main__":
    test()
