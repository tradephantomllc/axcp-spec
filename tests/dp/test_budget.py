import subprocess
import os, pathlib

ROOT = pathlib.Path(__file__).resolve().parents[2]

def test_budget_go():
    # Run the test in the dp package directory
    dp_dir = os.path.join(ROOT, "sdk", "go", "axcp", "dp")
    subprocess.check_call(
        ["go", "test", "-run", "TestBudget"],
        cwd=dp_dir
    )
