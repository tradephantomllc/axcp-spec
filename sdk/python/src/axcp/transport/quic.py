"""Async QUIC transport for AXCP envelopes."""

from __future__ import annotations

import asyncio
import inspect
import logging
import ssl
import struct
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Literal

from aioquic.asyncio import QuicConnectionProtocol
from aioquic.asyncio import connect as aioquic_connect
from aioquic.asyncio import serve as aioquic_serve
from aioquic.asyncio.protocol import QuicStreamAdapter
from aioquic.quic.configuration import QuicConfiguration
from aioquic.quic.events import (
    ConnectionTerminated,
    DatagramFrameReceived,
    QuicEvent,
    StopSendingReceived,
    StreamDataReceived,
    StreamReset,
)

from axcp.envelope import decode_envelope, encode_envelope
from axcp.errors import (
    TransportClosedError,
    TransportConfigError,
    TransportError,
    TransportFrameError,
    TransportTimeoutError,
)
from axcp.pb import axcp_pb2

DEFAULT_ALPN = "axcp/1"
MAX_FRAME_SIZE = 10 * 1024 * 1024
MAX_DATAGRAM_SIZE = 1200
DEFAULT_MAX_DATAGRAM_QUEUE = 1024
DEFAULT_READ_TIMEOUT = 300.0
MAX_RETAINED_HANDLER_ERRORS = 100
FRAME_HEADER_SIZE = 4
_APPLICATION_ERROR_BUFFER_LIMIT = 0x1

FrameEndian = Literal["big", "little"]
StreamHandler = Callable[["QuicConnection"], Awaitable[None] | None]
DatagramHandler = Callable[["_AXCPQuicProtocol", bytes], Awaitable[None] | None]

logger = logging.getLogger("axcp.transport.quic")

__all__ = [
    "DEFAULT_ALPN",
    "DEFAULT_MAX_DATAGRAM_QUEUE",
    "DEFAULT_READ_TIMEOUT",
    "MAX_DATAGRAM_SIZE",
    "MAX_FRAME_SIZE",
    "HandlerErrors",
    "QuicClient",
    "QuicClientConfig",
    "QuicConnection",
    "QuicServer",
    "QuicServerConfig",
    "TransportClosedError",
    "TransportConfigError",
    "TransportError",
    "TransportFrameError",
    "TransportTimeoutError",
]


class HandlerErrors:
    """Bounded record of background handler failures (never silently dropped, D11)."""

    def __init__(self, limit: int = MAX_RETAINED_HANDLER_ERRORS) -> None:
        self._limit = limit
        self._errors: list[BaseException] = []
        self.dropped = 0

    def record(self, error: BaseException) -> None:
        logger.error("AXCP QUIC handler failed", exc_info=(type(error), error, error.__traceback__))
        if len(self._errors) < self._limit:
            self._errors.append(error)
        else:
            self.dropped += 1

    @property
    def errors(self) -> tuple[BaseException, ...]:
        return tuple(self._errors)

    def track(self, tasks: set[asyncio.Task], task: asyncio.Task) -> None:
        """Keep a strong reference to ``task`` and record its exception when it finishes."""

        tasks.add(task)

        def _done(finished: asyncio.Task) -> None:
            tasks.discard(finished)
            if finished.cancelled():
                return
            error = finished.exception()
            if error is not None:
                self.record(error)

        task.add_done_callback(_done)


class _AXCPStreamAdapter(QuicStreamAdapter):
    """Honor QUIC half-close events before writing or sending a final FIN."""

    def write(self, data: bytes) -> None:
        error_code = self.protocol._stopped_sending.get(self.stream_id)
        if error_code is not None:
            raise TransportClosedError(
                f"peer stopped receiving stream {self.stream_id} (error {error_code})"
            )
        if self.is_closing():
            raise TransportClosedError("QUIC stream send direction is closed")
        super().write(data)

    def write_eof(self) -> None:
        if self.is_closing():
            self._closing = True
            return
        super().write_eof()

    def is_closing(self) -> bool:
        return (
            self._closing
            or self.protocol._terminated
            or self.stream_id in self.protocol._stopped_sending
        )


class _AXCPQuicProtocol(QuicConnectionProtocol):
    def __init__(
        self,
        *args,
        datagram_handler: DatagramHandler | None = None,
        max_stream_buffer: int = MAX_FRAME_SIZE + FRAME_HEADER_SIZE,
        max_datagram_queue: int = DEFAULT_MAX_DATAGRAM_QUEUE,
        handler_errors: HandlerErrors | None = None,
        **kwargs,
    ) -> None:
        super().__init__(*args, **kwargs)
        self._datagram_handler = datagram_handler
        self._max_stream_buffer = max_stream_buffer
        self._max_datagram_queue = max_datagram_queue
        self._datagrams: asyncio.Queue[bytes] = asyncio.Queue(maxsize=max_datagram_queue)
        self._datagram_tasks: set[asyncio.Task] = set()
        self._handler_errors = handler_errors if handler_errors is not None else HandlerErrors()
        self._stopped_sending: dict[int, int] = {}
        self._reset_streams: dict[int, int] = {}
        self._terminated = False
        self.dropped_datagrams = 0
        self.stream_buffer_overflows = 0
        self._closed_for_overflow = False

    def _create_stream(self, stream_id: int) -> tuple[asyncio.StreamReader, asyncio.StreamWriter]:
        # aioquic's default adapter does not consume STOP_SENDING / RESET_STREAM.
        # Keep its reader registration contract while handling both half-closes.
        adapter = _AXCPStreamAdapter(self, stream_id)
        reader = asyncio.StreamReader()
        protocol = asyncio.StreamReaderProtocol(reader)
        writer = asyncio.StreamWriter(adapter, protocol, reader, self._loop)
        self._stream_readers[stream_id] = reader
        if stream_id in self._reset_streams:
            reader.set_exception(self._reset_error(stream_id, self._reset_streams[stream_id]))
        return reader, writer

    @staticmethod
    def _reset_error(stream_id: int, error_code: int) -> TransportClosedError:
        return TransportClosedError(f"peer reset stream {stream_id} (error {error_code})")

    @property
    def handler_errors(self) -> tuple[BaseException, ...]:
        return self._handler_errors.errors

    def _stream_buffer_exceeded(self, event: StreamDataReceived) -> bool:
        if self._closed_for_overflow:
            return True  # connection already closing: drop any further stream data
        reader = self._stream_readers.get(event.stream_id)
        buffered = len(getattr(reader, "_buffer", b"")) if reader is not None else 0
        if buffered + len(event.data) <= self._max_stream_buffer:
            return False
        # Unread data would exceed max frame size + 4 bytes: close the connection (D11).
        self.stream_buffer_overflows += 1
        self._closed_for_overflow = True
        error = TransportFrameError(
            f"buffered stream data exceeds max frame size + 4 ({self._max_stream_buffer} bytes)"
        )
        if reader is not None:
            reader.set_exception(error)
        self.close(error_code=_APPLICATION_ERROR_BUFFER_LIMIT, reason_phrase="stream buffer limit exceeded")
        return True

    def _deliver_datagram(self, data: bytes) -> None:
        if self._datagram_handler is None:
            try:
                self._datagrams.put_nowait(data)
            except asyncio.QueueFull:
                self.dropped_datagrams += 1
            return
        if len(self._datagram_tasks) >= self._max_datagram_queue:
            self.dropped_datagrams += 1
            return
        try:
            result = self._datagram_handler(self, data)
        except Exception as exc:  # surfaced through HandlerErrors, never dropped
            self._handler_errors.record(exc)
            return
        if inspect.isawaitable(result):
            self._handler_errors.track(self._datagram_tasks, asyncio.ensure_future(result))

    def quic_event_received(self, event: QuicEvent) -> None:
        if isinstance(event, StreamDataReceived) and self._stream_buffer_exceeded(event):
            return
        if isinstance(event, StopSendingReceived):
            self._stopped_sending[event.stream_id] = event.error_code
        elif isinstance(event, StreamReset):
            self._reset_streams[event.stream_id] = event.error_code
            reader = self._stream_readers.get(event.stream_id)
            if reader is not None:
                reader.set_exception(self._reset_error(event.stream_id, event.error_code))
        elif isinstance(event, ConnectionTerminated):
            self._terminated = True
        super().quic_event_received(event)
        if isinstance(event, DatagramFrameReceived):
            self._deliver_datagram(event.data)

    def send_datagram(self, data: bytes) -> None:
        self._quic.send_datagram_frame(data)
        self.transmit()

    async def receive_datagram(self, timeout: float | None = DEFAULT_READ_TIMEOUT) -> bytes:
        try:
            if timeout is None:
                return await self._datagrams.get()
            return await asyncio.wait_for(self._datagrams.get(), timeout=timeout)
        except asyncio.TimeoutError as exc:
            raise TransportTimeoutError("timed out waiting for QUIC datagram") from exc


def _validate_common(
    alpn_protocols: tuple[str, ...],
    idle_timeout: float,
    max_frame_size: int,
    max_datagram_size: int,
    max_datagram_queue: int,
    read_timeout: float,
) -> None:
    if not alpn_protocols:
        raise TransportConfigError("at least one ALPN protocol is required")
    if idle_timeout <= 0:
        raise TransportConfigError("idle_timeout must be positive")
    if max_frame_size <= 0 or max_frame_size > 0xFFFFFFFF:
        raise TransportConfigError("max_frame_size must be between 1 and 2^32-1")
    if max_datagram_size <= 0:
        raise TransportConfigError("max_datagram_size must be positive")
    if max_datagram_queue <= 0:
        raise TransportConfigError("max_datagram_queue must be positive")
    if read_timeout <= 0:
        raise TransportConfigError("read_timeout must be positive")


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
    max_datagram_queue: int = DEFAULT_MAX_DATAGRAM_QUEUE
    read_timeout: float = DEFAULT_READ_TIMEOUT

    def validate(self) -> None:
        if not self.host:
            raise TransportConfigError("host is required")
        if not (0 < self.port <= 65535):
            raise TransportConfigError("port must be between 1 and 65535")
        _validate_common(
            self.alpn_protocols,
            self.idle_timeout,
            self.max_frame_size,
            self.max_datagram_size,
            self.max_datagram_queue,
            self.read_timeout,
        )


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
    max_datagram_queue: int = DEFAULT_MAX_DATAGRAM_QUEUE
    read_timeout: float = DEFAULT_READ_TIMEOUT

    def validate(self) -> None:
        if not self.host:
            raise TransportConfigError("host is required")
        if not (0 <= self.port <= 65535):
            raise TransportConfigError("port must be between 0 and 65535")
        if not self.cert_file:
            raise TransportConfigError("cert_file is required")
        if not self.key_file:
            raise TransportConfigError("key_file is required")
        _validate_common(
            self.alpn_protocols,
            self.idle_timeout,
            self.max_frame_size,
            self.max_datagram_size,
            self.max_datagram_queue,
            self.read_timeout,
        )


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
    except asyncio.TimeoutError as exc:
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
    except asyncio.TimeoutError as exc:
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
    """AXCP stream connection over QUIC.

    Receive calls without an explicit ``timeout`` use ``read_timeout`` (D11).
    """

    def __init__(
        self,
        reader: asyncio.StreamReader,
        writer: asyncio.StreamWriter,
        *,
        max_frame_size: int = MAX_FRAME_SIZE,
        read_timeout: float = DEFAULT_READ_TIMEOUT,
    ) -> None:
        self._reader = reader
        self._writer = writer
        self._max_frame_size = max_frame_size
        self._read_timeout = read_timeout
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
                timeout=self._read_timeout if timeout is None else timeout,
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

    async def receive_envelope(
        self, timeout: float | None = None, *, strict: bool = True
    ) -> axcp_pb2.AxcpEnvelope:
        """Receive an envelope using Go `RecvEnvelope` little-endian framing.

        Decoding is strict by default (see :func:`axcp.envelope.decode_envelope`).
        """

        async with self._recv_lock:
            self._ensure_open()
            data = await _read_frame(
                self._reader,
                endian="little",
                max_frame_size=self._max_frame_size,
                timeout=self._read_timeout if timeout is None else timeout,
            )
            return decode_envelope(data, strict=strict)

    async def close(self) -> None:
        if self._closed:
            return
        self._closed = True
        self._writer.close()
        await asyncio.sleep(0)

    def _ensure_open(self) -> None:
        if self._closed:
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
        super().__init__(
            reader, writer, max_frame_size=config.max_frame_size, read_timeout=config.read_timeout
        )
        self._config = config
        self._context_manager = context_manager
        self._protocol = protocol

    @classmethod
    async def connect(cls, config: QuicClientConfig) -> "QuicClient":
        config.validate()

        def create_protocol(*args, **kwargs) -> _AXCPQuicProtocol:
            return _AXCPQuicProtocol(
                *args,
                max_stream_buffer=config.max_frame_size + FRAME_HEADER_SIZE,
                max_datagram_queue=config.max_datagram_queue,
                **kwargs,
            )

        context_manager = aioquic_connect(
            config.host,
            config.port,
            configuration=_client_configuration(config),
            create_protocol=create_protocol,
            wait_connected=True,
        )
        protocol = await context_manager.__aenter__()
        reader, writer = await protocol.create_stream()
        return cls(config, context_manager, protocol, reader, writer)

    async def __aenter__(self) -> "QuicClient":
        return self

    async def __aexit__(self, exc_type, exc, tb) -> None:
        await self.close()

    @property
    def dropped_datagrams(self) -> int:
        return self._protocol.dropped_datagrams

    async def send_datagram(self, data: bytes) -> None:
        if len(data) > self._config.max_datagram_size:
            raise TransportFrameError(
                f"datagram length {len(data)} exceeds max_datagram_size {self._config.max_datagram_size}"
            )
        self._protocol.send_datagram(data)
        await asyncio.sleep(0)

    async def receive_datagram(self, timeout: float | None = None) -> bytes:
        return await self._protocol.receive_datagram(
            timeout=self._config.read_timeout if timeout is None else timeout
        )

    async def close(self) -> None:
        if self._closed:
            return
        try:
            await super().close()
        finally:
            await self._context_manager.__aexit__(None, None, None)


class QuicServer:
    """AXCP QUIC server.

    Stream and datagram handler tasks are tracked; their exceptions are logged, exposed
    through :attr:`handler_errors` and the first one is re-raised by :meth:`close`.
    """

    def __init__(
        self,
        config: QuicServerConfig,
        server,
        tasks: set[asyncio.Task],
        errors: HandlerErrors,
    ) -> None:
        self._config = config
        self._server = server
        self._tasks = tasks
        self._errors = errors
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
        errors = HandlerErrors()

        def stream_handler(reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
            connection = QuicConnection(
                reader, writer, max_frame_size=config.max_frame_size, read_timeout=config.read_timeout
            )
            errors.track(tasks, asyncio.ensure_future(_run_stream_handler(handler, connection)))

        def create_protocol(*args, **kwargs):
            return _AXCPQuicProtocol(
                *args,
                datagram_handler=datagram_handler,
                max_stream_buffer=config.max_frame_size + FRAME_HEADER_SIZE,
                max_datagram_queue=config.max_datagram_queue,
                handler_errors=errors,
                **kwargs,
            )

        server = await aioquic_serve(
            config.host,
            config.port,
            configuration=_server_configuration(config),
            create_protocol=create_protocol,
            stream_handler=stream_handler,
        )
        return cls(config, server, tasks, errors)

    @property
    def address(self) -> tuple[str, int]:
        sockname = self._server._transport.get_extra_info("sockname")
        return sockname[0], sockname[1]

    @property
    def handler_errors(self) -> tuple[BaseException, ...]:
        """Exceptions raised by stream or datagram handlers so far."""

        return self._errors.errors

    async def close(self) -> None:
        """Stop the server; re-raises the first handler exception, if any."""

        if self._closed:
            return
        self._closed = True
        self._server.close()
        pending_tasks = set(self._tasks)
        if pending_tasks:
            _, pending = await asyncio.wait(pending_tasks, timeout=2)
            for task in pending:
                task.cancel()
            if pending:
                await asyncio.wait(pending, timeout=1)
        errors = self._errors.errors
        if errors:
            raise errors[0]

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
