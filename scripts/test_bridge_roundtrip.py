#!/usr/bin/env python3
import sys
import os

# Add the root directory and proto directory to sys.path
# Go up 1 level from scripts/ to reach the root of the repo
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'proto')))

import json
import base64
from google.protobuf import json_format
import proto.axcp_pb2 as axcp
from gateway.mcp_bridge import mcp_to_axcp, axcp_to_mcp

def test():
    mcp = {"id": "req-123", "tool": {"name": "search", "version": "1.0"}}
    env = mcp_to_axcp(mcp)
    assert env.capability_msg.offer.desc.tool_id == "search"
    back = axcp_to_mcp(env)
    assert back["id"] == "req-123"
    print("bridge round-trip OK")

if __name__ == "__main__":
    test()
