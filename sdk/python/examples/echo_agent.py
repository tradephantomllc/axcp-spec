from axcp import Agent, Identity
from axcp.pb import axcp_pb2


def main() -> None:
    alice = Agent(Identity.generate())
    bob = Agent(Identity.generate())

    env = alice.new_envelope(trace_id="echo-001")
    env.context_patch.CopyFrom(
        axcp_pb2.ContextPatch(
            context_id="demo",
            base_version=1,
            ops=[
                axcp_pb2.DeltaOp(
                    op=axcp_pb2.DeltaOp.ADD,
                    path="/message",
                    data=b"hello axcp",
                    ts=1,
                )
            ],
        )
    )

    alice.sign_message(env, recipient_did=bob.identity.did)
    bob.verify(env)
    print(f"verified envelope from {env.sender_did} to {env.recipient_did}")


if __name__ == "__main__":
    main()
