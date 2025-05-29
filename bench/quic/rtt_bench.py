#!/usr/bin/env python3
import asyncio
import socket
import time
import uuid
import sys

# Simple UDP echo server and client for RTT measurement

HOST = '127.0.0.1'
PORT = 61235
COUNT = int(sys.argv[1]) if len(sys.argv) > 1 else 1000

async def udp_echo_server():
    loop = asyncio.get_running_loop()
    
    class EchoServerProtocol:
        def connection_made(self, transport):
            self.transport = transport
            
        def datagram_received(self, data, addr):
            # Echo back the received data
            self.transport.sendto(data, addr)
            
        def connection_lost(self, exc):
            # Handle connection lost
            pass
    
    # Create datagram endpoint
    transport, _ = await loop.create_datagram_endpoint(
        lambda: EchoServerProtocol(),
        local_addr=(HOST, PORT)
    )
    
    try:
        # Keep the server running
        while True:
            await asyncio.sleep(3600)  # Sleep for 1 hour
    except asyncio.CancelledError:
        pass
    finally:
        transport.close()

async def udp_echo_client(count):
    loop = asyncio.get_running_loop()
    
    # Create a UDP socket
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setblocking(False)
    
    # Generate some test data
    test_data = b'test_packet'
    
    # Warm-up
    sock.sendto(test_data, (HOST, PORT))
    await loop.sock_recv(sock, 1024)
    
    # Measure RTT
    times = []
    for _ in range(count):
        start = time.perf_counter()
        sock.sendto(test_data, (HOST, PORT))
        await loop.sock_recv(sock, 1024)
        end = time.perf_counter()
        times.append((end - start) * 1_000_000)  # convert to microseconds
    
    sock.close()
    return times

async def main():
    # Start server task
    server_task = asyncio.create_task(udp_echo_server())
    
    # Give server time to start
    await asyncio.sleep(1)
    
    try:
        # Run client
        times = await udp_echo_client(COUNT)
        
        # Calculate statistics
        avg_rtt = sum(times) / len(times)
        min_rtt = min(times)
        max_rtt = max(times)
        
        print(f"Packets sent: {COUNT}")
        print(f"Average RTT: {avg_rtt:.2f} μs")
        print(f"Minimum RTT: {min_rtt:.2f} μs")
        print(f"Maximum RTT: {max_rtt:.2f} μs")
        
    finally:
        # Clean up
        server_task.cancel()
        try:
            await server_task
        except asyncio.CancelledError:
            pass

if __name__ == "__main__":
    asyncio.run(main())
