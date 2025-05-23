#!/usr/bin/env python3
import json, base64
from google.protobuf import json_format
import proto.axcp_pb2 as axcp
from gateway.mcp_bridge import mcp_to_axcp, axcp_to_mcp

def test():
    mcp = {"id": "req-123", "tool": {"name": "search", "version": "1.0"}}
    env = mcp_to_axcp(mcp)
    assert env.capability_msg.offer.desc.id == "search"
    back = axcp_to_mcp(env)
    assert back["id"] == "req-123"
    print("bridge round-trip OK")

if __name__ == "__main__":
    test()
