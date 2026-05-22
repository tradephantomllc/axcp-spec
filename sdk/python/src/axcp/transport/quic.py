"""Async QUIC transport for AXCP envelopes."""

from __future__ import annotations

import asyncio
import inspect
import ssl
import struct
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Literal

from aioquic.asyncio import QuicConnectionProtocol
from aioquic.asyncio import connect as aioquic_connect
from aioquic.asyncio import serve as aioquic_serve
from aioquic.quic.configuration import QuicConfiguration
from aioquic.quic.events import DatagramFrameReceived, QuicEvent

from axcp.envelope import decode_envelope, encode_envelope
from axcp.pb import axcp_pb2

DEFAULT_ALPN = "axcp/1"
MAX_FRAME_SIZE = 10 * 1024 * 1024
MAX_DATAGRAM_SIZE = 1200

FrameEndian = Literal["big", "little"]
StreamHandler = Callable[["QuicConnection"], Awaitable[None] | None]
DatagramHandler = Callable[["_AXCPQuicProtocol", bytes], Awaitable[None] | None]


class TransportError(Exception):
    """Base class for QUIC transport errors."""


class TransportConfigError(TransportError):
    """Transport configuration is invalid."""


class TransportClosedError(TransportError):
    """Connection or stream was closed before the operation completed."""


class TransportFrameError(TransportError):
    """A length-prefixed QUIC frame is malformed."""


class TransportTimeoutError(TransportError):
    """A transport operation timed out."""


class _AXCPQuicProtocol(QuicConnectionProtocol):
    def __init__(
        self,
        *args,
        datagram_handler: DatagramHandler | None = None,
        **kwargs,
    ) -> None:
        super().__init__(*args, **kwargs)
        self._datagram_handler = datagram_handler
        self._datagrams: asyncio.Queue[bytes] = asyncio.Queue()

    def quic_event_received(self, event: QuicEvent) -> None:
        super().quic_event_received(event)
        if isinstance(event, DatagramFrameReceived):
            self._datagrams.put_nowait(event.data)
            if self._datagram_handler is not None:
                result = self._datagram_handler(self, event.data)
                if inspect.isawaitable(result):
                    asyncio.create_task(result)

    def send_datagram(self, data: bytes) -> None:
        self._quic.send_datagram_frame(data)
        self.transmit()

    async def receive_datagram(self, timeout: float | None = None) -> bytes:
        try:
            if timeout is None:
                return await self._datagrams.get()
            return await asyncio.wait_for(self._datagrams.get(), timeout=timeout)
        except TimeoutError as exc:
            raise TransportTimeoutError("timed out waiting for QUIC datagram") from exc


@dataclass(frozen=True)
class QuicClientConfig:
    host: str
    port: int
    server_name: str | None = None
    ca_file: str | None = None
    insecure_skip_verify: bool = False
    alpn_protocols: tuple[str, ...] = (DEFAULT_ALPN,)
    idle_timeout: float = 60.0
    max_frame_size: int = MAX_FRAME_SIZE
    max_datagram_size: int = MAX_DATAGRAM_SIZE

    def validate(self) -> None:
        if not self.host:
            raise TransportConfigError("host is required")
        if not (0 < self.port <= 65535):
            raise TransportConfigError("port must be between 1 and 65535")
        if not self.alpn_protocols:
            raise TransportConfigError("at least one ALPN protocol is required")
        if self.idle_timeout <= 0:
            raise TransportConfigError("idle_timeout must be positive")
        if self.max_frame_size <= 0:
            raise TransportConfigError("max_frame_size must be positive")
        if self.max_datagram_size <= 0:
            raise TransportConfigError("max_datagram_size must be positive")


@dataclass(frozen=True)
class QuicServerConfig:
    host: str
    port: int
    cert_file: str
    key_file: str
    alpn_protocols: tuple[str, ...] = (DEFAULT_ALPN,)
    idle_timeout: float = 60.0
    max_frame_size: int = MAX_FRAME_SIZE
    max_datagram_size: int = MAX_DATAGRAM_SIZE

    def validate(self) -> None:
        if not self.host:
            raise TransportConfigError("host is required")
        if not (0 <= self.port <= 65535):
            raise TransportConfigError("port must be between 0 and 65535")
        if not self.cert_file:
            raise TransportConfigError("cert_file is required")
        if not self.key_file:
            raise TransportConfigError("key_file is required")
        if not self.alpn_protocols:
            raise TransportConfigError("at least one ALPN protocol is required")
        if self.idle_timeout <= 0:
            raise TransportConfigError("idle_timeout must be positive")
        if self.max_frame_size <= 0:
            raise TransportConfigError("max_frame_size must be positive")
        if self.max_datagram_size <= 0:
            raise TransportConfigError("max_datagram_size must be positive")


def _client_configuration(config: QuicClientConfig) -> QuicConfiguration:
    quic_config = QuicConfiguration(
        is_client=True,
        alpn_protocols=list(config.alpn_protocols),
        idle_timeout=config.idle_timeout,
        max_datagram_frame_size=config.max_datagram_size,
    )
    quic_config.server_name = config.server_name or config.host
    if config.insecure_skip_verify:
        quic_config.verify_mode = ssl.CERT_NONE
    elif config.ca_file is not None:
        quic_config.load_verify_locations(cafile=config.ca_file)
    return quic_config


def _server_configuration(config: QuicServerConfig) -> QuicConfiguration:
    quic_config = QuicConfiguration(
        is_client=False,
        alpn_protocols=list(config.alpn_protocols),
        idle_timeout=config.idle_timeout,
        max_datagram_frame_size=config.max_datagram_size,
    )
    quic_config.load_cert_chain(config.cert_file, config.key_file)
    return quic_config


async def _read_frame(
    reader: asyncio.StreamReader,
    *,
    endian: FrameEndian,
    max_frame_size: int,
    timeout: float | None = None,
) -> bytes:
    try:
        if timeout is None:
            header = await reader.readexactly(4)
        else:
            header = await asyncio.wait_for(reader.readexactly(4), timeout=timeout)
    except asyncio.IncompleteReadError as exc:
        raise TransportClosedError("stream closed while reading frame header") from exc
    except TimeoutError as exc:
        raise TransportTimeoutError("timed out reading frame header") from exc

    frame_len = struct.unpack(">I" if endian == "big" else "<I", header)[0]
    if frame_len > max_frame_size:
        raise TransportFrameError(f"frame length {frame_len} exceeds max_frame_size {max_frame_size}")

    try:
        if timeout is None:
            return await reader.readexactly(frame_len)
        return await asyncio.wait_for(reader.readexactly(frame_len), timeout=timeout)
    except asyncio.IncompleteReadError as exc:
        raise TransportClosedError("stream closed while reading frame payload") from exc
    except TimeoutError as exc:
        raise TransportTimeoutError("timed out reading frame payload") from exc


async def _write_frame(
    writer: asyncio.StreamWriter,
    data: bytes,
    *,
    endian: FrameEndian,
    max_frame_size: int,
) -> None:
    if len(data) > max_frame_size:
        raise TransportFrameError(f"frame length {len(data)} exceeds max_frame_size {max_frame_size}")
    writer.write(struct.pack(">I" if endian == "big" else "<I", len(data)) + data)
    await asyncio.sleep(0)


class QuicConnection:
    """AXCP stream connection over QUIC."""

    def __init__(
        self,
        reader: asyncio.StreamReader,
        writer: asyncio.StreamWriter,
        *,
        max_frame_size: int = MAX_FRAME_SIZE,
    ) -> None:
        self._reader = reader
        self._writer = writer
        self._max_frame_size = max_frame_size
        self._send_lock = asyncio.Lock()
        self._recv_lock = asyncio.Lock()
        self._closed = False

    async def send_message(self, data: bytes) -> None:
        """Send raw bytes using Go `SendMessage` big-endian framing."""

        async with self._send_lock:
            self._ensure_open()
            await _write_frame(self._writer, data, endian="big", max_frame_size=self._max_frame_size)

    async def receive_message(self, timeout: float | None = None) -> bytes:
        """Receive raw bytes using Go `ReceiveMessage` big-endian framing."""

        async with self._recv_lock:
            self._ensure_open()
            return await _read_frame(
                self._reader,
                endian="big",
                max_frame_size=self._max_frame_size,
                timeout=timeout,
            )

    async def send_envelope(self, envelope: axcp_pb2.AxcpEnvelope) -> None:
        """Send an envelope using Go `SendEnvelope` little-endian framing."""

        async with self._send_lock:
            self._ensure_open()
            await _write_frame(
                self._writer,
                encode_envelope(envelope),
                endian="little",
                max_frame_size=self._max_frame_size,
            )

    async def receive_envelope(self, timeout: float | None = None) -> axcp_pb2.AxcpEnvelope:
        """Receive an envelope using Go `RecvEnvelope` little-endian framing."""

        async with self._recv_lock:
            self._ensure_open()
            data = await _read_frame(
                self._reader,
                endian="little",
                max_frame_size=self._max_frame_size,
                timeout=timeout,
            )
            return decode_envelope(data)

    async def close(self) -> None:
        if self._closed:
            return
        self._closed = True
        self._writer.close()
        await asyncio.sleep(0)

    def _ensure_open(self) -> None:
        if self._closed or self._writer.is_closing():
            raise TransportClosedError("QUIC stream is closed")


class QuicClient(QuicConnection):
    """AXCP QUIC client with one bidirectional control stream."""

    def __init__(
        self,
        config: QuicClientConfig,
        context_manager,
        protocol: _AXCPQuicProtocol,
        reader: asyncio.StreamReader,
        writer: asyncio.StreamWriter,
    ) -> None:
        super().__init__(reader, writer, max_frame_size=config.max_frame_size)
        self._config = config
        self._context_manager = context_manager
        self._protocol = protocol

    @classmethod
    async def connect(cls, config: QuicClientConfig) -> "QuicClient":
        config.validate()
        context_manager = aioquic_connect(
            config.host,
            config.port,
            configuration=_client_configuration(config),
            create_protocol=_AXCPQuicProtocol,
            wait_connected=True,
        )
        protocol = await context_manager.__aenter__()
        reader, writer = await protocol.create_stream()
        return cls(config, context_manager, protocol, reader, writer)

    async def __aenter__(self) -> "QuicClient":
        return self

    async def __aexit__(self, exc_type, exc, tb) -> None:
        await self.close()

    async def send_datagram(self, data: bytes) -> None:
        if len(data) > self._config.max_datagram_size:
            raise TransportFrameError(
                f"datagram length {len(data)} exceeds max_datagram_size {self._config.max_datagram_size}"
            )
        self._protocol.send_datagram(data)
        await asyncio.sleep(0)

    async def receive_datagram(self, timeout: float | None = None) -> bytes:
        return await self._protocol.receive_datagram(timeout=timeout)

    async def close(self) -> None:
        if self._closed:
            return
        await super().close()
        await self._context_manager.__aexit__(None, None, None)


class QuicServer:
    """AXCP QUIC server."""

    def __init__(
        self,
        config: QuicServerConfig,
        server,
        tasks: set[asyncio.Task],
    ) -> None:
        self._config = config
        self._server = server
        self._tasks = tasks
        self._closed = False

    @classmethod
    async def listen(
        cls,
        config: QuicServerConfig,
        handler: StreamHandler,
        *,
        datagram_handler: DatagramHandler | None = None,
    ) -> "QuicServer":
        config.validate()
        tasks: set[asyncio.Task] = set()

        def stream_handler(reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
            connection = QuicConnection(reader, writer, max_frame_size=config.max_frame_size)
            task = asyncio.create_task(_run_stream_handler(handler, connection))
            tasks.add(task)
            task.add_done_callback(tasks.discard)

        def create_protocol(*args, **kwargs):
            return _AXCPQuicProtocol(*args, datagram_handler=datagram_handler, **kwargs)

        server = await aioquic_serve(
            config.host,
            config.port,
            configuration=_server_configuration(config),
            create_protocol=create_protocol,
            stream_handler=stream_handler,
        )
        return cls(config, server, tasks)

    @property
    def address(self) -> tuple[str, int]:
        sockname = self._server._transport.get_extra_info("sockname")
        return sockname[0], sockname[1]

    async def close(self) -> None:
        if self._closed:
            return
        self._closed = True
        self._server.close()
        if self._tasks:
            done, pending = await asyncio.wait(self._tasks, timeout=2)
            for task in pending:
                task.cancel()
            for task in done:
                task.result()

    async def __aenter__(self) -> "QuicServer":
        return self

    async def __aexit__(self, exc_type, exc, tb) -> None:
        await self.close()


async def _run_stream_handler(handler: StreamHandler, connection: QuicConnection) -> None:
    try:
        result = handler(connection)
        if inspect.isawaitable(result):
            await result
    finally:
        await connection.close()
