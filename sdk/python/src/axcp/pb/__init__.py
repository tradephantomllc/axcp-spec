"""Generated AXCP protobuf bindings bundled with the SDK.

Only the bundled module is imported: an application-level ``proto`` package can never
replace the SDK's bindings. (A repository-root ``proto/axcp_pb2.py`` generated from the
same ``axcp.proto`` may still be imported alongside; both register the identical file
descriptor and share message classes.)
"""

from . import axcp_pb2 as axcp_pb2

__all__ = ["axcp_pb2"]
