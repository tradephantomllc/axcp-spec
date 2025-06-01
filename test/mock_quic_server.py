#!/usr/bin/env python3
import asyncio
import base64
import struct
import sys
import paho.mqtt.client as mqtt
from proto import axcp_pb2 as pb

class MockQuicServer:
    def __init__(self, host='localhost', port=7143, mqtt_host='localhost', mqtt_port=1883):
        self.host = host
        self.port = port
        self.mqtt_host = mqtt_host
        self.mqtt_port = mqtt_port
        self.mqtt_client = mqtt.Client()
        
    async def handle_client(self, reader, writer):
        """Handle a client connection."""
        print(f"Client connected from {writer.get_extra_info('peername')}")
        
        try:
            # Read the 4-byte length prefix
            length_bytes = await reader.read(4)
            if not length_bytes or len(length_bytes) < 4:
                print("Incomplete length prefix, closing connection")
                return
                
            # Unpack the message length
            message_length = struct.unpack("<I", length_bytes)[0]
            print(f"Received message length: {message_length} bytes")
            
            # Read the serialized AxcpEnvelope
            data = await reader.read(message_length)
            if len(data) != message_length:
                print(f"Expected {message_length} bytes, got {len(data)} bytes")
                return
                
            # Parse the AxcpEnvelope
            envelope = pb.AxcpEnvelope()
            envelope.ParseFromString(data)
            print(f"Received envelope: version={envelope.version}, trace_id={envelope.trace_id}")
            
            # Forward to MQTT broker
            mqtt_payload = base64.b64encode(data)
            self.mqtt_client.publish(f"axcp/{envelope.trace_id}", mqtt_payload)
            print(f"Published to MQTT topic: axcp/{envelope.trace_id}")
            
        except Exception as e:
            print(f"Error handling client: {e}")
        finally:
            writer.close()
            await writer.wait_closed()
            print("Connection closed")
    
    async def start_server(self):
        """Start the mock QUIC server."""
        # Connect to MQTT broker
        self.mqtt_client.connect(self.mqtt_host, self.mqtt_port)
        self.mqtt_client.loop_start()
        print(f"Connected to MQTT broker at {self.mqtt_host}:{self.mqtt_port}")
        
        # Start TCP server
        server = await asyncio.start_server(
            self.handle_client, self.host, self.port)
        
        addr = server.sockets[0].getsockname()
        print(f"Serving on {addr}")
        
        async with server:
            await server.serve_forever()
    
if __name__ == "__main__":
    server = MockQuicServer()
    try:
        print("Starting mock QUIC server on port 7143")
        asyncio.run(server.start_server())
    except KeyboardInterrupt:
        print("Server stopped by user")

