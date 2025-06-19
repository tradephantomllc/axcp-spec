"""Basic Python round-trip test for AxcpEnvelope serialization."""
from praibridge.bridge import wrap_envelope, unwrap_envelope
from axcp_pb2 import AxcpEnvelope  # type: ignore


def test_trace_id_roundtrip() -> None:
    original = AxcpEnvelope(trace_id="test123")
    data = wrap_envelope(original)
    result = unwrap_envelope(data)
    assert result.trace_id == "test123"
