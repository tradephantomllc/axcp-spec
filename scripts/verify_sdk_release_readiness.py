#!/usr/bin/env python3
"""Release-readiness checks for public AXCP SDK packages."""

from __future__ import annotations

import json
import sys
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
VECTOR_PATH = ROOT / "testdata" / "sdk" / "secure_baseline_vector.json"
PYPROJECT_PATH = ROOT / "sdk" / "python" / "pyproject.toml"
NPM_PACKAGE_PATH = ROOT / "sdk" / "typescript" / "package.json"
DEPENDABOT_PATH = ROOT / ".github" / "dependabot.yml"


def main() -> int:
    errors: list[str] = []
    errors.extend(check_vector())
    errors.extend(check_python_package())
    errors.extend(check_typescript_package())
    errors.extend(check_dependabot())

    if errors:
        for error in errors:
            print(f"release-readiness: {error}", file=sys.stderr)
        return 1
    print("release-readiness: ok")
    return 0


def check_vector() -> list[str]:
    errors: list[str] = []
    vector = json.loads(VECTOR_PATH.read_text(encoding="utf-8"))
    required_top_level = {
        "name",
        "protocol_version",
        "profile",
        "trace_id",
        "recipient_did",
        "timestamp_ms",
        "sequence",
        "identity",
        "context_patch",
        "expected",
    }
    missing = required_top_level - vector.keys()
    if missing:
        errors.append(f"{VECTOR_PATH} missing keys: {sorted(missing)}")

    identity = vector.get("identity", {})
    seed = identity.get("private_seed_hex", "")
    if len(seed) != 64:
        errors.append("secure baseline vector private_seed_hex must be 32 bytes in hex")
    if not identity.get("did", "").startswith("did:key:z"):
        errors.append("secure baseline vector identity.did must be did:key base58btc")

    expected = vector.get("expected", {})
    for key in ("signing_payload_b64", "auth_transcript_b64", "signature_b64", "envelope_b64"):
        if not expected.get(key):
            errors.append(f"secure baseline vector expected.{key} is required")
    return errors


def check_python_package() -> list[str]:
    errors: list[str] = []
    pyproject = tomllib.loads(PYPROJECT_PATH.read_text(encoding="utf-8"))
    project = pyproject.get("project", {})
    if project.get("name") != "axcp":
        errors.append("Python package name must remain axcp")
    if project.get("license") != "Apache-2.0":
        errors.append("Python SDK license must be Apache-2.0")
    if not project.get("readme"):
        errors.append("Python SDK must declare a README")
    dependencies = set(project.get("dependencies", []))
    for required in ("protobuf>=6.31.1", "PyNaCl>=1.5.0"):
        if required not in dependencies:
            errors.append(f"Python SDK missing dependency {required}")
    package_data = pyproject.get("tool", {}).get("setuptools", {}).get("package-data", {})
    if "py.typed" not in package_data.get("axcp", []):
        errors.append("Python SDK must package py.typed")
    return errors


def check_typescript_package() -> list[str]:
    errors: list[str] = []
    package = json.loads(NPM_PACKAGE_PATH.read_text(encoding="utf-8"))
    if package.get("name") != "@tradephantom/axcp":
        errors.append("TypeScript package name must remain @tradephantom/axcp")
    if package.get("license") != "Apache-2.0":
        errors.append("TypeScript SDK license must be Apache-2.0")
    exports = package.get("exports", {})
    for subpath in (".", "./pb", "./transport"):
        if subpath not in exports:
            errors.append(f"TypeScript SDK missing export {subpath}")
    files = set(package.get("files", []))
    if "dist/src" not in files or "README.md" not in files:
        errors.append("TypeScript SDK files must include dist/src and README.md")
    scripts = package.get("scripts", {})
    for script in ("build", "test", "typecheck", "prepack"):
        if script not in scripts:
            errors.append(f"TypeScript SDK missing npm script {script}")
    return errors


def check_dependabot() -> list[str]:
    text = DEPENDABOT_PATH.read_text(encoding="utf-8")
    errors: list[str] = []
    if 'package-ecosystem: "npm"' not in text:
        errors.append("Dependabot must track npm dependencies")
    if 'directory: "/sdk/typescript"' not in text:
        errors.append("Dependabot npm entry must target /sdk/typescript")
    return errors


if __name__ == "__main__":
    raise SystemExit(main())
