import json, os
import pytest

ROOT = os.path.abspath(os.path.join(__file__, "..", ".."))
GOLD = os.path.join(ROOT, "tests", "dp", "golden")

def load(name):
    return json.load(open(os.path.join(GOLD, name)))

def test_laplace_stats():
    g = load("laplace_mean_var.json")
    assert abs(g["Mean"]) < 0.03
    assert abs(g["Var"] - 2.0) < 0.06

def test_gauss_stats():
    g = load("gaussian_mean_var.json")
    assert abs(g["Mean"]) < 0.03
    assert abs(g["Var"] - 1.0) < 0.05
