"""Python helper wrappers for AXCP ↔ PraisonAI bridge.

Usage:
    from praibridge.bridge import wrap_envelope, unwrap_envelope

`AxcpEnvelope` comes from the generated `axcp_pb2` module which must be
compiled from `proto/axcp.proto`, e.g.:

    python -m grpc_tools.protoc -I proto --python_out=. proto/axcp.proto

This file purposefully contains no business logic beyond basic (de)serialization.
"""
from __future__ import annotations

from axcp_pb2 import AxcpEnvelope  # type: ignore


def wrap_envelope(env: AxcpEnvelope) -> bytes:  # noqa: D401
    """Serialize an `AxcpEnvelope` into raw bytes."""
    return env.SerializeToString()


def unwrap_envelope(data: bytes) -> AxcpEnvelope:  # noqa: D401
    """Deserialize bytes into an `AxcpEnvelope`. Raises `DecodeError` on failure."""
    env = AxcpEnvelope()
    env.ParseFromString(data)
    return env
