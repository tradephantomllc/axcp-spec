# AXCP Core quickstart

Prerequisites: Git and Go 1.26.10 or newer on a supported Go release. Commands start at the repository root.

```sh
git clone https://github.com/tradephantomllc/axcp-spec.git
cd axcp-spec
go test ./sdk/go/...
go build -o /tmp/axcp-authenticated-chat ./examples/go/authenticated_chat
```

In terminal 1:

```sh
/tmp/axcp-authenticated-chat -server
```

In terminal 2:

```sh
/tmp/axcp-authenticated-chat
/tmp/axcp-authenticated-chat -replay
```

The client verifies the signed response. The replay run demonstrates rejection of a duplicated sequence. Stop the server with Ctrl+C.

This is an offline, localhost demonstration with fixed test identities and self-signed TLS. Both peers are configured for Secure Baseline; the example does not run the SDK's wire profile handshake. Its stream format and ALPN are specific to the demonstration, so it is not a client for the MQTT gateway. Its identities are standard Ed25519 `did:key` values derived from fixed demo seeds, both peers verify every envelope with the SDK `Validator` (structure, recipient, timestamp, signature, then replay), and `-replay` exercises the SDK `ReplayProtector` locally. Never reuse its deterministic keys in a deployment.

For the actual QUIC-to-MQTT gateway, follow [Gateway setup](gateway-setup.md). For a local signed-envelope roundtrip without a server, see the [Python and TypeScript examples](getting-started.md); the Python SDK ships it as a script, `sdk/python/examples/echo_agent.py`, which signs and verifies one envelope in-process without any network. There are no `axcpctl` or `make gateway` commands in this repository.
