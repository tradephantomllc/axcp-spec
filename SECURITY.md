# Security Policy

## Supported Scope

This repository contains AXCP Core. Security issues affecting the public Core
specification, SDKs, gateway, examples, CI, or release artifacts are in scope.

Commercial components are maintained separately. Report issues in them through
your commercial support channel or to the address below.

## Reporting Vulnerabilities

**Do not open public GitHub issues, pull requests or discussions for security
vulnerabilities.** Report them privately through one of these channels:

1. **GitHub private vulnerability reporting:** use
   [Report a vulnerability](https://github.com/tradephantomllc/axcp-spec/security/advisories/new)
   on this repository.
2. **Email:** support@getaxcp.com, with "AXCP security" in the subject line.

Please include the affected component and version or commit, a description of
the issue and its impact, and reproduction steps. Do not include real private
keys, tokens or production data; use test identities.

We will acknowledge your report, keep you informed while we investigate, and
coordinate disclosure with you once a fix or mitigation is available.

## Branch Protection

All changes to `main` must go through a pull request. Branch protection is
enforced for administrators and requires the branch to be up to date before
merge. Force pushes and branch deletions are disabled, and conversations must be
resolved before merging.

Required status checks are defined in the branch protection settings, which are
authoritative. At the time of writing they cover the Core CI jobs (Go, Python,
TypeScript, gateway telemetry, Raspberry Pi agent and examples), the Rust CI
jobs and the `license/cla` Contributor License Agreement check.

## Dependency Handling

Scheduled Dependabot version updates are intentionally low-noise. Security
alerts and security updates should remain enabled in repository settings, while
dependency changes that raise runtime or language baselines are handled through
explicit maintainer PRs.
