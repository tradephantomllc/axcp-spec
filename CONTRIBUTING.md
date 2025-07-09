# Contributing to AXCP

Thank you for considering a contribution to AXCP!

## ⚠️ Legal Requirement: Contributor License Agreement (CLA)

Before your contribution can be accepted, you **must read and agree to the CLA** located in [CLA.md](./CLA.md). By submitting a pull request, you confirm your agreement to these terms.

**If you do not agree, do not submit a PR.**

## How to Contribute

1. Fork this repository.
2. Create a feature branch.
3. Write your code or documentation.
4. Ensure all tests pass (`go test ./...`, `pytest`, etc.).
5. Submit a Pull Request (PR) with a clear description.

All contributions are subject to review and approval by the maintainers. Submissions not aligned with the roadmap or project principles may be declined.

## Development Guidelines

### Regenerating Protobuf Stubs

If you modify the protocol definition in `proto/axcp.proto`, you **must** regenerate the language-specific stubs:

#### Go Stubs
```bash
cd sdk/go
mkdir -p internal/pb
protoc -I ../../proto \
  --go_out=internal/pb --go_opt=paths=source_relative \
  ../../proto/axcp.proto
```

#### Python Stubs
```bash
python -m grpc_tools.protoc -I=proto --python_out=proto proto/axcp.proto
```

#### Rust Stubs
Rust protobuf generation is handled automatically by the build system.

**Important Notes:**
- Do **not** commit generated stub files unless absolutely necessary
- The CI pipeline automatically generates stubs during testing
- Local development should use the commands above for testing changes
- Always run `go mod tidy` after regenerating Go stubs

Thanks again!
