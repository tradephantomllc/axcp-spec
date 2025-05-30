import subprocess
import os, pathlib

ROOT = pathlib.Path(__file__).resolve().parents[2]

def test_budget_go():
    # run the nested Go module under sdk/go
    subprocess.check_call(
        ["go", "test", "-run", "TestBudget", "./dp"],
        cwd=os.path.join(ROOT, "sdk", "go")
    )
