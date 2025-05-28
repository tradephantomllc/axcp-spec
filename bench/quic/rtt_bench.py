#!/usr/bin/env python3
import asyncio, sys, time, uuid, os

# Aggiungi il percorso alla radice del progetto e alla directory proto
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../../../')))
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../../../proto')))

from aioquic.asyncio import serve, connect
import proto.axcp_pb2 as pb

COUNT = int(sys.argv[1]) if len(sys.argv) > 1 else 1000

def sample_env():
    env = pb.AxcpEnvelope(version=1, trace_id=str(uuid.uuid4()), profile=0)
    return env.SerializeToString()

async def echo_server(reader, writer):
    try:
        while True:
            size = int.from_bytes(await reader.readexactly(4), "little")
            buf  = await reader.readexactly(size)
            writer.write(size.to_bytes(4,"little") + buf)
            await writer.drain()
    except asyncio.IncompleteReadError:
        pass

async def bench():
    server = await serve("127.0.0.1", 61235, configuration=None, stream_handler=echo_server)
    async with connect("127.0.0.1", 61235, configuration=None) as client:
        stream = await client.create_stream()
        send = sample_env()
        t0 = time.perf_counter()
        for _ in range(COUNT):
            stream.write(len(send).to_bytes(4,"little") + send)
            size = int.from_bytes(await stream.readexactly(4), "little")
            await stream.readexactly(size)
        rtt = (time.perf_counter() - t0) / COUNT * 1_000_000
        print(f"{COUNT} pkts  avg RTT = {rtt:.1f} μs")
    server.close()
    await server.wait_closed()

if __name__ == "__main__":
    asyncio.run(bench())
