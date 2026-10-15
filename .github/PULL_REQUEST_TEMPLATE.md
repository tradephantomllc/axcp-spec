## Summary

Describe the change and the problem it solves.

## Validation

List the local commands or CI jobs used to verify the change.

## Risk

Call out compatibility, security, release, or migration impact. Changes to the
specification, `proto/axcp.proto`, the canonical signing payload or the
transcript are protocol changes. Write `None` if there is no material risk.

## Checklist

- [ ] Title follows `type(scope): subject`
- [ ] I have signed the [Contributor License Agreement](https://cla-assistant.io/tradephantomllc/axcp-spec) (the `license/cla` check passes)
- [ ] Every commit includes a DCO `Signed-off-by` line (`git commit -s`)
- [ ] Public docs or examples were updated when behavior changed
- [ ] Relevant CI jobs pass
- [ ] This PR does not disclose an unfixed vulnerability (report those privately; see `SECURITY.md`)
