# Go Module Policy

This document defines the binding rules for Go module structure in AXCP Core.

## Single Module Rule

**`sdk/go` is the single Go module for AXCP Core.**

### What this means

1. **No nested modules**: Do NOT create `go.mod` files inside `sdk/go/` subdirectories
2. **Internal packages stay internal**: `internal/` packages can only be imported within `sdk/go`
3. **External access via re-exports**: Public types are exposed via `axcp/pb/export.go`

### Why this matters

Go's `internal/` package visibility is scoped to the **module**, not the directory. If we create nested modules (e.g., `sdk/go/axcp/go.mod`), then `sdk/go/axcp` becomes a separate module and **cannot** import from `sdk/go/internal/pb`.

This caused CI failures in the past and required a refactoring fix.

## Correct Structure

```
sdk/go/
├── go.mod                    # Single module: github.com/tradephantomllc/axcp-spec/sdk/go
├── internal/
│   └── pb/                   # Generated protobuf (internal only)
├── axcp/
│   ├── pb/
│   │   └── export.go         # Re-exports internal/pb types for external use
│   └── *.go                  # Public AXCP types
├── auth/
├── negotiate/
└── netquic/
```

## Import Patterns

### From within sdk/go (OK)
```go
import pb "github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
```

### From external packages (axcp-advanced, axcp-enterprise)
```go
// Use the public re-exports, NOT internal/pb directly
import pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
```

## Violations

The following are **not allowed**:

- Creating `sdk/go/*/go.mod` files
- Importing `sdk/go/internal/*` from outside `sdk/go` module
- Moving `internal/pb` to a public location

## Enforcement

- CI will fail if nested modules are detected
- Code review must reject PRs that violate this policy
