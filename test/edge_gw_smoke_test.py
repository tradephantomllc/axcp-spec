#!/usr/bin/env python3
import asyncio, base64, os, uuid
import paho.mqtt.client as mqtt
from aioquic.asyncio import connect
import proto.axcp_pb2 as pb
import pytest
pytest.importorskip("paho.mqtt.client")

async def send():
    env = pb.AxcpEnvelope(version=1, trace_id=str(uuid.uuid4()), profile=0)
    async with connect("localhost", 7143, configuration=None) as client:
        stream = await client.create_stream()
        raw = env.SerializeToString()
        stream.write(len(raw).to_bytes(4, "little") + raw)
        await stream.drain()

def on_msg(_, __, msg):
    env = pb.AxcpEnvelope()
    env.ParseFromString(base64.b64decode(msg.payload))
    print("received via MQTT:", env.trace_id)
    os._exit(0)

def run():
    cli = mqtt.Client()
    cli.on_message = on_msg
    cli.connect("localhost", 1883)
    cli.subscribe("axcp/#")
    cli.loop_start()
    asyncio.run(send())

if __name__ == "__main__":
    run()
