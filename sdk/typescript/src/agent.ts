import {
  type NewEnvelopeOptions,
  decodeEnvelope,
  newEnvelope,
  signEnvelope,
  verifyEnvelope,
} from "./envelope.js";
import { InvalidEnvelopeError } from "./errors.js";
import { type DIDResolver, Identity } from "./identity.js";
import { type AxcpEnvelope, toSafeNumber } from "./pb/schema.js";
import { ReplayProtector } from "./replay.js";
import { TimestampValidator, nowMicros } from "./timestamp.js";

export interface AgentOptions {
  /** Resolver used for every sender DID, including did:key (allowlist, D3). */
  readonly resolver?: DIDResolver;
  /** Replay protector; its peer TTL is raised to at least maxAge + clockSkew. */
  readonly replay?: ReplayProtector;
  readonly timestamps?: TimestampValidator;
  /**
   * First sequence number used by {@link Agent.signMessage}. Defaults to the current
   * Unix time in microseconds so sequences stay monotonic across restarts (D7).
   */
  readonly initialSequence?: number;
}

export interface AgentSignOptions {
  /** Recipient DID (required, D4). */
  readonly recipientDid: string;
  readonly timestampMs?: number;
}

export class Agent {
  readonly identity: Identity;
  readonly resolver: DIDResolver | undefined;
  readonly replay: ReplayProtector;
  readonly timestamps: TimestampValidator;
  private nextSequence: number;

  constructor(identity: Identity, options: AgentOptions = {}) {
    this.identity = identity;
    this.resolver = options.resolver;
    this.timestamps = options.timestamps ?? new TimestampValidator();
    this.replay = options.replay ?? new ReplayProtector();
    this.replay.applyTimestampPolicy(this.timestamps);
    const initial = options.initialSequence ?? nowMicros();
    if (!Number.isSafeInteger(initial) || initial < 1) {
      throw new InvalidEnvelopeError("initialSequence must be a positive safe integer");
    }
    this.nextSequence = initial;
  }

  newEnvelope(options: NewEnvelopeOptions = {}): AxcpEnvelope {
    return newEnvelope(options);
  }

  /** Sign with the next sequence number. `recipientDid` is required. */
  signMessage(envelope: AxcpEnvelope, options: AgentSignOptions): AxcpEnvelope {
    const sequence = this.nextSequence;
    if (!Number.isSafeInteger(sequence + 1)) {
      throw new InvalidEnvelopeError("sequence space exhausted; create a new agent");
    }
    const signed = signEnvelope(envelope, this.identity, {
      recipientDid: options?.recipientDid,
      sequence,
      ...(options?.timestampMs === undefined ? {} : { timestampMs: options.timestampMs }),
    });
    this.nextSequence = sequence + 1;
    return signed;
  }

  /**
   * Verify a decoded envelope: structure (D5), recipient == this agent (D4), signature,
   * timestamp, then replay (state mutated only after the signature is valid). Prefer
   * {@link Agent.verifyBytes} for network input so unknown fields and duplicate payloads
   * are detected on the wire.
   */
  async verify(envelope: AxcpEnvelope): Promise<void> {
    await verifyEnvelope(envelope, {
      expectedRecipientDid: this.identity.did,
      ...(this.resolver === undefined ? {} : { resolver: this.resolver }),
    });
    this.timestamps.validate(toSafeNumber(envelope.timestampMs, "timestampMs"));
    this.replay.checkAndMark(envelope.senderDid as string, toSafeNumber(envelope.sequence, "sequence"));
  }

  /** Strictly decode and verify wire bytes; returns the verified envelope. */
  async verifyBytes(data: Uint8Array): Promise<AxcpEnvelope> {
    const envelope = decodeEnvelope(data, { strict: true });
    await this.verify(envelope);
    return envelope;
  }
}
