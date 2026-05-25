# Security Policy

## Supported Scope

This repository contains AXCP Core. Security issues affecting the public Core
specification, SDKs, gateway examples, CI, or release artifacts are in scope.

Advanced and Enterprise code is maintained separately and should be reported
through the private support channel used for those repositories.

## Reporting Vulnerabilities

Use GitHub private vulnerability reporting for this repository when available.
Do not open a public issue containing exploit details, private keys, tokens,
reproduction payloads, or sensitive deployment information.

If private reporting is unavailable, open a minimal public issue requesting a
security contact and omit technical details until a private channel is agreed.

## Branch Protection

All changes to `main` must go through a pull request. Branch protection is
enforced for administrators, requires the branch to be up to date, and requires
these checks before merge:

- `Test Go`
- `Test Python`
- `Test TypeScript`
- `Test Gateway Telemetry`
- `Test RPi Agent`
- `Check Examples`

Force pushes and branch deletions are disabled. Conversations must be resolved
before merging.

## Dependency Handling

Scheduled Dependabot version updates are intentionally low-noise. Security
alerts and security updates should remain enabled in repository settings, while
dependency changes that raise runtime or language baselines are handled through
explicit maintainer PRs.
