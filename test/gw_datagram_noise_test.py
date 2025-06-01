import base64
import asyncio
import subprocess
import os
import time
import sys
import proto.axcp_pb2 as pb
from aioquic.asyncio import connect

# Try to find mosquitto_sub in common locations
MOSQUITTO_PATHS = [
    os.path.join("C:\\", "Program Files", "mosquitto", "mosquitto_sub.exe"),
    os.path.join("C:\\", "Program Files (x86)", "mosquitto", "mosquitto_sub.exe"),
    "mosquitto_sub"  # Fallback to PATH
]

# Find the first valid mosquitto_sub path
MOSQUITTO_SUB = None
for path in MOSQUITTO_PATHS:
    if os.path.exists(path) or path == "mosquitto_sub":
        MOSQUITTO_SUB = path
        break

if not MOSQUITTO_SUB:
    print("Error: mosquitto_sub not found. Please install Mosquitto MQTT broker.")
    sys.exit(1)

def send_td(count=1, profile=3):
    """Create a telemetry datagram with the specified profile."""
    td = pb.TelemetryDatagram(
        timestamp_ms=int(time.time() * 1000),
        profile=profile,
        payload=pb.TelemetryDatagram.System(
            system=pb.SystemStats(
                cpu_percent=50,
                mem_bytes=1000000
            )
        )
    )
    # Serialize and prepend the 0xA0 header
    raw = td.SerializeToString()
    return b'\xA0' + raw

async def run():
    """Send a test telemetry datagram to the gateway."""
    async with connect("localhost", 7143, configuration=None) as c:
        await c.send_datagram_frame(send_td())
        print("Sent telemetry datagram")

def test_noise():
    """Test the telemetry datagram with noise application."""
    # Start mosquitto_sub in the background to capture the published telemetry
    try:
        process = subprocess.Popen(
            [MOSQUITTO_SUB, "-t", "telemetry/#", "-C", "1", "-h", "localhost"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
    except FileNotFoundError:
        print(f"Error: Could not find mosquitto_sub at {MOSQUITTO_SUB}")
        print("Please ensure Mosquitto MQTT broker is installed and in your PATH")
        sys.exit(1)
    
    # Give mosquitto_sub a moment to start
    time.sleep(1)
    
    try:
        # Run the test
        asyncio.run(run())
        
        # Wait for the message to be received
        time.sleep(1)
        
        # Check if we got any output
        output, _ = process.communicate(timeout=2)
        if output:
            print("Successfully received telemetry data:")
            print(output.decode())
        else:
            print("No telemetry data received")
    finally:
        # Clean up
        process.terminate()
        process.wait()

if __name__ == "__main__":
    test_noise()
