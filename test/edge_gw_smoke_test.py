#!/usr/bin/env python3
import asyncio, base64, os, uuid, struct
import paho.mqtt.client as mqtt
import proto.axcp_pb2 as pb
import pytest
pytest.importorskip("paho.mqtt.client")

async def send():
    env = pb.AxcpEnvelope(version=1, trace_id=str(uuid.uuid4()), profile=0)
    reader, writer = await asyncio.open_connection("localhost", 7143)
    try:
        raw = env.SerializeToString()
        # Send 4-byte length prefix followed by serialized protobuf, same as before
        writer.write(len(raw).to_bytes(4, "little") + raw)
        await writer.drain()
        print(f"Sent envelope with trace_id: {env.trace_id}")
    except Exception as e:
        print(f"Error sending message: {e}")
    finally:
        writer.close()
        await writer.wait_closed()

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
