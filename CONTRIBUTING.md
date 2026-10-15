# Contributing to AXCP

Thank you for considering a contribution to AXCP!

## License

AXCP Core is licensed under the Apache License 2.0, and accepted contributions are distributed as part of AXCP Core under that license. The rights you grant when contributing are defined by the CLA below.

## Contributor License Agreement (CLA) and sign-off

Every contribution needs **both** of the following:

1. **A signed CLA.** Contributors must sign the project's Contributor License Agreement through [CLA Assistant](https://cla-assistant.io/tradephantomllc/axcp-spec). When you open your first pull request, CLA Assistant adds a comment with a link where you can read and sign the agreement. The `license/cla` status check must pass before a pull request can be merged. The agreement's terms are the ones shown by CLA Assistant.
2. **A DCO sign-off on every commit.** Add a `Signed-off-by` line to each commit to certify the [Developer Certificate of Origin](https://developercertificate.org/):

   ```
   Signed-off-by: Your Name <your.email@example.com>
   ```

   `git commit -s` adds it automatically. Use the name and email address you want recorded with your contribution.

By signing off, you certify that:

1. The contribution was created in whole or in part by you and you have the right to submit it under the Apache 2.0 license; or
2. The contribution is based upon previous work that, to the best of your knowledge, is covered under an appropriate open source license and you have the right to submit that work with modifications; or
3. The contribution was provided directly to you by some other person who certified (1) or (2) and you have not modified it.

## How to Contribute

1. Fork this repository.
2. Create a feature branch (`git checkout -b feature/my-feature`).
3. Write your code or documentation.
4. Run the relevant checks (see [SDK release readiness](docs/sdk-release-readiness.md) for the full list; for example `go test ./sdk/go/...` or `python -m pytest -q sdk/python/tests`).
5. Commit with sign-off (`git commit -s -m "type(scope): subject"`).
6. Open a pull request with a clear description and sign the CLA if CLA Assistant asks you to.

Changes to the specification (`spec/`), the Protobuf schema (`proto/`) or the canonical signing payload and transcript are protocol changes. Describe the compatibility impact and update the shared test vectors in `testdata/sdk/` intentionally.

## Security issues

Do not report vulnerabilities in public issues or pull requests. Follow the private process in [SECURITY.md](SECURITY.md).

## Code Review

All contributions are subject to review and approval by the maintainers. We aim to review PRs promptly, but please be patient during busy periods.

Submissions not aligned with the roadmap or project principles may be declined, but we'll always provide feedback.

## Questions?

For questions about contributing, open a question issue or contact dev@tradephantom.com.

Thanks again!
