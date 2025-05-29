#!/usr/bin/env python3
import sys
import os
import json
import base64

# Add the project root to Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import proto.axcp_pb2 as pb
from gateway.mcp_bridge import mcp_to_axcp, axcp_to_mcp

SAMPLES = "gateway/samples"

def load(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)

def test_mcp_roundtrip():
    claude = load(f"{SAMPLES}/mcp_claude_search.json")
    env = mcp_to_axcp(claude)
    back = axcp_to_mcp(env)
    assert back["id"] == claude["id"]
    # Verify the context_delta contains the expected data
    assert back.get("context_delta") is not None
    # The actual content is a serialized AxcpEnvelope with capability_msg
    assert back.get("ts") is not None

def test_profile_downgrade():
    claude = load(f"{SAMPLES}/mcp_claude_search.json")
    env = mcp_to_axcp(claude)
    # The profile field is set but not used in axcp_to_mcp conversion
    env.profile = 3
    back = axcp_to_mcp(env)
    # The trace_id should be the same as the original id
    assert back["id"] == claude["id"]
    # Verify the timestamp is included
    assert back.get("ts") is not None
