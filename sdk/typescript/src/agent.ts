import {
  type NewEnvelopeOptions,
  type SignEnvelopeOptions,
  newEnvelope,
  signEnvelope,
  verifyEnvelope,
} from "./envelope.js";
import { InvalidEnvelopeError } from "./errors.js";
import { type DIDResolver, Identity } from "./identity.js";
import { type AxcpEnvelope, toSafeNumber } from "./pb/schema.js";
import { ReplayProtector } from "./replay.js";
import { TimestampValidator } from "./timestamp.js";

export interface AgentOptions {
  readonly resolver?: DIDResolver;
  readonly replay?: ReplayProtector;
  readonly timestamps?: TimestampValidator;
}

export class Agent {
  readonly identity: Identity;
  readonly resolver: DIDResolver | undefined;
  readonly replay: ReplayProtector;
  readonly timestamps: TimestampValidator;
  private sequence = 0;

  constructor(identity: Identity, options: AgentOptions = {}) {
    this.identity = identity;
    this.resolver = options.resolver;
    this.replay = options.replay ?? new ReplayProtector();
    this.timestamps = options.timestamps ?? new TimestampValidator();
  }

  newEnvelope(options: NewEnvelopeOptions = {}): AxcpEnvelope {
    return newEnvelope(options);
  }

  signMessage(envelope: AxcpEnvelope, options: Omit<SignEnvelopeOptions, "sequence"> = {}): AxcpEnvelope {
    this.sequence += 1;
    return signEnvelope(envelope, this.identity, {
      ...options,
      sequence: this.sequence,
    });
  }

  async verify(envelope: AxcpEnvelope): Promise<void> {
    if (envelope.recipientDid && envelope.recipientDid !== this.identity.did) {
      throw new InvalidEnvelopeError("envelope recipient_did does not match this agent");
    }
    const verifyOptions = this.resolver === undefined ? {} : { resolver: this.resolver };
    await verifyEnvelope(envelope, verifyOptions);
    this.timestamps.validate(toSafeNumber(envelope.timestampMs, "timestampMs"));
    this.replay.checkAndMark(envelope.senderDid ?? "", toSafeNumber(envelope.sequence, "sequence"));
  }
}
