"""PraisonAI ↔ AXCP bridge Python helpers.

This package provides thin wrapper functions to serialize / deserialize
`AxcpEnvelope` protobuf messages for sending over QUIC channels.

Note: `axcp_pb2.py` must be generated from `proto/axcp.proto` using
`python -m grpc_tools.protoc` or similar. For CI we assume it is pre-generated
or vendored alongside the package at project root.
"""

from .bridge import wrap_envelope, unwrap_envelope  # noqa: F401
