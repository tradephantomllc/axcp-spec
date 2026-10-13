# Go module policy

`sdk/go` is one installable SDK module, including `axcp`, `auth`, `negotiate`, `netquic` and its examples. Do not introduce nested `go.mod` files under it. The repository root, gateway, Raspberry Pi collector and integration tests remain separate modules joined by `go.work` for repository development.

Go's `internal` visibility is based on the import-path tree rooted above the `internal` directory. Public consumers use `github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb` re-exports, not `sdk/go/internal/pb`. There is no importable package at the SDK module root itself.

## Migration from the old nested netquic module

Package import paths are unchanged. Remove an explicit requirement or replacement for the old `github.com/tradephantomllc/axcp-spec/sdk/go/netquic` module when upgrading to the consolidated SDK; otherwise Go may report an ambiguous import. Remove any separate telemetry-example module requirement too. Keep only the parent SDK module dependency at the selected reviewed revision.

```sh
go mod edit -droprequire=github.com/tradephantomllc/axcp-spec/sdk/go/netquic
go mod edit -dropreplace=github.com/tradephantomllc/axcp-spec/sdk/go/netquic
go mod tidy
```

For an unreleased local checkout, a consumer may replace the parent SDK with its absolute path. No dependency-side replacement is required. Repository application modules use local replacements because they are built from this checkout; that does not imply the SDK needs unpublished `v0.0.0` dependencies.

Release acceptance must include a `GOWORK=off` consumer importing both `axcp` and `netquic`, followed by installation from the actual published revision. Local tests cannot certify a not-yet-published tag. CI runs the full SDK including netquic, and the release-readiness script checks that no nested SDK modules reappear.
