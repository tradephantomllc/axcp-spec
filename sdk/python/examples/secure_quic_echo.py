import argparse
import asyncio

from axcp import Agent, Identity, QuicClient, QuicClientConfig, QuicServer, QuicServerConfig
from axcp.pb import axcp_pb2
from axcp.transport import QuicConnection


def build_envelope(agent: Agent, recipient_did: str, trace_id: str) -> axcp_pb2.AxcpEnvelope:
    env = agent.new_envelope(trace_id=trace_id)
    env.context_patch.CopyFrom(
        axcp_pb2.ContextPatch(
            context_id="demo",
            base_version=1,
            ops=[
                axcp_pb2.DeltaOp(
                    op=axcp_pb2.DeltaOp.ADD,
                    path="/message",
                    data=b"hello axcp over quic",
                    ts=1,
                )
            ],
        )
    )
    agent.sign_message(env, recipient_did=recipient_did)
    return env


async def run_server(args: argparse.Namespace) -> None:
    server_agent = Agent(Identity.generate())

    async def handler(conn: QuicConnection) -> None:
        incoming = await conn.receive_envelope()
        server_agent.verify(incoming)
        reply = build_envelope(server_agent, incoming.sender_did, incoming.trace_id)
        await conn.send_envelope(reply)

    server = await QuicServer.listen(
        QuicServerConfig(
            host=args.host,
            port=args.port,
            cert_file=args.cert,
            key_file=args.key,
        ),
        handler,
    )
    host, port = server.address
    print(f"AXCP Python QUIC echo server listening on {host}:{port}")
    print(f"Server DID: {server_agent.identity.did}")
    try:
        await asyncio.Event().wait()
    finally:
        await server.close()


async def run_client(args: argparse.Namespace) -> None:
    client_agent = Agent(Identity.generate())
    server_did = args.server_did or ""
    client = await QuicClient.connect(
        QuicClientConfig(
            host=args.host,
            port=args.port,
            server_name=args.server_name,
            ca_file=args.ca_file,
            insecure_skip_verify=args.insecure_skip_verify,
        )
    )
    try:
        await client.send_envelope(build_envelope(client_agent, server_did, "python-quic-echo"))
        reply = await client.receive_envelope()
        client_agent.verify(reply)
        print(f"verified reply from {reply.sender_did}: {reply.context_patch.context_id}")
    finally:
        await client.close()


def main() -> None:
    parser = argparse.ArgumentParser(description="AXCP Python secure QUIC echo example")
    sub = parser.add_subparsers(dest="mode", required=True)

    server = sub.add_parser("server")
    server.add_argument("--host", default="127.0.0.1")
    server.add_argument("--port", type=int, default=61300)
    server.add_argument("--cert", required=True)
    server.add_argument("--key", required=True)

    client = sub.add_parser("client")
    client.add_argument("--host", default="127.0.0.1")
    client.add_argument("--port", type=int, default=61300)
    client.add_argument("--server-name", default="localhost")
    client.add_argument("--server-did")
    client.add_argument("--ca-file")
    client.add_argument("--insecure-skip-verify", action="store_true")

    args = parser.parse_args()
    if args.mode == "server":
        asyncio.run(run_server(args))
    else:
        asyncio.run(run_client(args))


if __name__ == "__main__":
    main()
