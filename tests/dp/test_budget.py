import subprocess
import os, pathlib

ROOT = pathlib.Path(__file__).resolve().parents[2]

def test_budget_go():
    subprocess.check_call(["go", "test", "-run", "TestBudget", "./sdk/go/dp"], cwd=ROOT)
