#!/usr/bin/env python3
import json, base64
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
    decoded = pb.AxcpEnvelope()
    decoded.ParseFromString(base64.b64decode(back["context_delta"]))
    assert decoded.capability_msg.offer.desc.id == claude["tool"]["name"]

def test_profile_downgrade():
    claude = load(f"{SAMPLES}/mcp_claude_search.json")
    env = mcp_to_axcp(claude)
    env.Profile = 3
    back = axcp_to_mcp(env)
    assert back["id"] == claude["id"]
