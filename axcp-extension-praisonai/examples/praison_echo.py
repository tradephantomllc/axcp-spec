"""Minimal echo example to validate round-trip serialization."""
from praibridge.bridge import wrap_envelope, unwrap_envelope
from axcp_pb2 import AxcpEnvelope  # type: ignore


def main() -> None:
    env = AxcpEnvelope(trace_id="echo-123")
    data = wrap_envelope(env)
    new_env = unwrap_envelope(data)
    print("Roundtrip OK:", new_env.trace_id == "echo-123")


if __name__ == "__main__":
    main()
