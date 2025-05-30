import subprocess
import os, pathlib

ROOT = pathlib.Path(__file__).resolve().parents[2]

def test_budget_go():
    # Usa il percorso completo del modulo Go
    subprocess.check_call(["go", "test", "-run", "TestBudget", "github.com/tradephantom/axcp-spec/sdk/go/dp"], cwd=ROOT)
