"""AXCP SDK exception hierarchy."""

from __future__ import annotations


class AXCPError(Exception):
    """Base class for all AXCP SDK errors."""

    reason_code = "axcp_error"


class DIDError(AXCPError):
    """DID parsing or resolution failed."""

    reason_code = "did_error"


class InvalidDIDFormat(DIDError):
    reason_code = "invalid_did_format"


class DIDResolutionError(DIDError):
    reason_code = "did_resolution_failed"


class KeyMismatchError(DIDError):
    reason_code = "did_key_mismatch"


class SignatureError(AXCPError):
    """Envelope signing or verification failed."""

    reason_code = "signature_error"


class InvalidPrivateKeyError(SignatureError):
    reason_code = "invalid_private_key"


class SignatureMissingError(SignatureError):
    reason_code = "signature_missing"


class SignatureVerificationError(SignatureError):
    reason_code = "signature_verification_failed"


class TimestampError(AXCPError):
    """Timestamp validation failed."""

    reason_code = "timestamp_error"


class TimestampMissingError(TimestampError):
    reason_code = "timestamp_missing"


class TimestampExpiredError(TimestampError):
    reason_code = "timestamp_expired"


class TimestampFutureError(TimestampError):
    reason_code = "timestamp_future"


class ReplayError(AXCPError):
    """Replay protection rejected an envelope."""

    reason_code = "replay_error"


class ReplayDetectedError(ReplayError):
    reason_code = "replay_detected"


class SequenceTooOldError(ReplayError):
    reason_code = "sequence_too_old"


class InvalidReplayConfigError(ReplayError):
    reason_code = "invalid_replay_config"


class NegotiationError(AXCPError):
    """Profile negotiation failed."""

    reason_code = "negotiation_failed"


class NoProfileOverlapError(NegotiationError):
    reason_code = "no_profile_overlap"


class DeprecatedProfileError(NegotiationError):
    reason_code = "deprecated_profile"


class InvalidCapabilitiesError(NegotiationError):
    reason_code = "invalid_capabilities"


class NoSignatureAlgorithmOverlapError(NegotiationError):
    reason_code = "no_signature_algorithm_overlap"


class InvalidEnvelopeError(AXCPError):
    """Envelope serialization or payload validation failed."""

    reason_code = "invalid_envelope"
