export class AXCPError extends Error {
  readonly reasonCode: string;

  constructor(message: string, reasonCode = "axcp_error") {
    super(message);
    this.name = new.target.name;
    this.reasonCode = reasonCode;
  }
}

export class DIDError extends AXCPError {
  constructor(message: string, reasonCode = "did_error") {
    super(message, reasonCode);
  }
}

export class InvalidDIDFormat extends DIDError {
  constructor(message: string) {
    super(message, "invalid_did_format");
  }
}

export class DIDResolutionError extends DIDError {
  constructor(message: string) {
    super(message, "did_resolution_failed");
  }
}

export class KeyMismatchError extends DIDError {
  constructor(message: string) {
    super(message, "did_key_mismatch");
  }
}

export class SignatureError extends AXCPError {
  constructor(message: string, reasonCode = "signature_error") {
    super(message, reasonCode);
  }
}

export class InvalidPrivateKeyError extends SignatureError {
  constructor(message: string) {
    super(message, "invalid_private_key");
  }
}

export class SignatureMissingError extends SignatureError {
  constructor(message: string) {
    super(message, "signature_missing");
  }
}

export class SignatureVerificationError extends SignatureError {
  constructor(message: string) {
    super(message, "signature_verification_failed");
  }
}

export class TimestampError extends AXCPError {
  constructor(message: string, reasonCode = "timestamp_error") {
    super(message, reasonCode);
  }
}

export class TimestampMissingError extends TimestampError {
  constructor(message: string) {
    super(message, "timestamp_missing");
  }
}

export class TimestampExpiredError extends TimestampError {
  constructor(message: string) {
    super(message, "timestamp_expired");
  }
}

export class TimestampFutureError extends TimestampError {
  constructor(message: string) {
    super(message, "timestamp_future");
  }
}

export class ReplayError extends AXCPError {
  constructor(message: string, reasonCode = "replay_error") {
    super(message, reasonCode);
  }
}

export class ReplayDetectedError extends ReplayError {
  constructor(message: string) {
    super(message, "replay_detected");
  }
}

export class SequenceTooOldError extends ReplayError {
  constructor(message: string) {
    super(message, "sequence_too_old");
  }
}

export class InvalidReplayConfigError extends ReplayError {
  constructor(message: string) {
    super(message, "invalid_replay_config");
  }
}

export class NegotiationError extends AXCPError {
  constructor(message: string, reasonCode = "negotiation_failed") {
    super(message, reasonCode);
  }
}

export class NoProfileOverlapError extends NegotiationError {
  constructor(message: string) {
    super(message, "no_profile_overlap");
  }
}

export class DeprecatedProfileError extends NegotiationError {
  constructor(message: string) {
    super(message, "deprecated_profile");
  }
}

export class InvalidCapabilitiesError extends NegotiationError {
  constructor(message: string) {
    super(message, "invalid_capabilities");
  }
}

export class NoSignatureAlgorithmOverlapError extends NegotiationError {
  constructor(message: string) {
    super(message, "no_signature_algorithm_overlap");
  }
}

export class InvalidEnvelopeError extends AXCPError {
  constructor(message: string) {
    super(message, "invalid_envelope");
  }
}
