"""Generated AXCP protobuf bindings.

When running from the repository, reuse the root-level generated module so
legacy bridge tests and SDK tests share one descriptor registration. Installed
packages fall back to the packaged generated module.
"""

try:
    from proto import axcp_pb2 as axcp_pb2
except Exception:  # pragma: no cover - exercised after package installation
    from . import axcp_pb2 as axcp_pb2
