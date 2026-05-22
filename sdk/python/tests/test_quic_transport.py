import asyncio
import ipaddress
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

from axcp import Agent, Identity, QuicClient, QuicClientConfig, QuicServer, QuicServerConfig
from axcp.pb import axcp_pb2
from axcp.transport import QuicConnection, TransportFrameError, TransportTimeoutError


def _write_self_signed_cert(tmp_path: Path) -> tuple[str, str]:
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = issuer = x509.Name(
        [
            x509.NameAttribute(NameOID.ORGANIZATION_NAME, "AXCP Tests"),
            x509.NameAttribute(NameOID.COMMON_NAME, "localhost"),
        ]
    )
    now = datetime.now(timezone.utc)
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1))
        .not_valid_after(now + timedelta(days=1))
        .add_extension(
            x509.SubjectAlternativeName(
                [
                    x509.DNSName("localhost"),
                    x509.IPAddress(ipaddress.ip_address("127.0.0.1")),
                ]
            ),
            critical=False,
        )
        .sign(key, hashes.SHA256())
    )

    cert_file = tmp_path / "cert.pem"
    key_file = tmp_path / "key.pem"
    cert_file.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    key_file.write_bytes(
        key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.TraditionalOpenSSL,
            serialization.NoEncryption(),
        )
    )
    return str(cert_file), str(key_file)


def _context_envelope(trace_id: str = "quic-trace"):
    env = axcp_pb2.AxcpEnvelope(version=1, trace_id=trace_id, profile=1)
    env.context_patch.CopyFrom(
        axcp_pb2.ContextPatch(
            context_id="ctx",
            base_version=1,
            ops=[axcp_pb2.DeltaOp(op=axcp_pb2.DeltaOp.ADD, path="/message", data=b"ping", ts=1)],
        )
    )
    return env


def test_quic_transport_signed_envelope_roundtrip(tmp_path: Path) -> None:
    asyncio.run(_signed_envelope_roundtrip(tmp_path))


async def _signed_envelope_roundtrip(tmp_path: Path) -> None:
    cert_file, key_file = _write_self_signed_cert(tmp_path)
    alice = Agent(Identity.generate())
    server_agent = Agent(Identity.generate())

    async def handler(conn: QuicConnection) -> None:
        incoming = await conn.receive_envelope(timeout=2)
        server_agent.verify(incoming)

        reply = server_agent.new_envelope(trace_id=incoming.trace_id)
        reply.context_patch.CopyFrom(
            axcp_pb2.ContextPatch(context_id="reply", base_version=2)
        )
        server_agent.sign_message(reply, recipient_did=incoming.sender_did)
        await conn.send_envelope(reply)

    server = await QuicServer.listen(
        QuicServerConfig(host="127.0.0.1", port=0, cert_file=cert_file, key_file=key_file),
        handler,
    )
    try:
        host, port = server.address
        client = await QuicClient.connect(
            QuicClientConfig(host=host, port=port, server_name="localhost", insecure_skip_verify=True)
        )
        try:
            env = _context_envelope()
            alice.sign_message(env, recipient_did=server_agent.identity.did)
            await client.send_envelope(env)

            reply = await client.receive_envelope(timeout=2)
            alice.verify(reply)
            assert reply.context_patch.context_id == "reply"
            assert reply.recipient_did == alice.identity.did
        finally:
            await client.close()
    finally:
        await server.close()


def test_quic_transport_raw_message_big_endian_roundtrip(tmp_path: Path) -> None:
    asyncio.run(_raw_message_big_endian_roundtrip(tmp_path))


async def _raw_message_big_endian_roundtrip(tmp_path: Path) -> None:
    cert_file, key_file = _write_self_signed_cert(tmp_path)

    async def handler(conn: QuicConnection) -> None:
        message = await conn.receive_message(timeout=2)
        assert message == b"hello"
        await conn.send_message(b"world")

    server = await QuicServer.listen(
        QuicServerConfig(host="127.0.0.1", port=0, cert_file=cert_file, key_file=key_file),
        handler,
    )
    try:
        host, port = server.address
        client = await QuicClient.connect(
            QuicClientConfig(host=host, port=port, server_name="localhost", insecure_skip_verify=True)
        )
        try:
            await client.send_message(b"hello")
            assert await client.receive_message(timeout=2) == b"world"
        finally:
            await client.close()
    finally:
        await server.close()


def test_quic_transport_datagram_echo(tmp_path: Path) -> None:
    asyncio.run(_datagram_echo(tmp_path))


async def _datagram_echo(tmp_path: Path) -> None:
    cert_file, key_file = _write_self_signed_cert(tmp_path)

    def datagram_handler(protocol, data: bytes) -> None:
        protocol.send_datagram(data)

    async def handler(conn: QuicConnection) -> None:
        await asyncio.sleep(0.2)

    server = await QuicServer.listen(
        QuicServerConfig(host="127.0.0.1", port=0, cert_file=cert_file, key_file=key_file),
        handler,
        datagram_handler=datagram_handler,
    )
    try:
        host, port = server.address
        client = await QuicClient.connect(
            QuicClientConfig(host=host, port=port, server_name="localhost", insecure_skip_verify=True)
        )
        try:
            await client.send_datagram(b"telemetry")
            assert await client.receive_datagram(timeout=2) == b"telemetry"
        finally:
            await client.close()
    finally:
        await server.close()


def test_quic_transport_rejects_oversized_message(tmp_path: Path) -> None:
    asyncio.run(_rejects_oversized_message(tmp_path))


async def _rejects_oversized_message(tmp_path: Path) -> None:
    cert_file, key_file = _write_self_signed_cert(tmp_path)

    async def handler(conn: QuicConnection) -> None:
        with pytest.raises(TransportTimeoutError):
            await conn.receive_message(timeout=0.2)

    server = await QuicServer.listen(
        QuicServerConfig(host="127.0.0.1", port=0, cert_file=cert_file, key_file=key_file),
        handler,
    )
    try:
        host, port = server.address
        client = await QuicClient.connect(
            QuicClientConfig(
                host=host,
                port=port,
                server_name="localhost",
                insecure_skip_verify=True,
                max_frame_size=4,
            )
        )
        try:
            with pytest.raises(TransportFrameError):
                await client.send_message(b"too-large")
        finally:
            await client.close()
    finally:
        await server.close()
