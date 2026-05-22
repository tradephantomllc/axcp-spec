import Long from "long";
import protobuf from "protobufjs";

protobuf.util.Long = Long;
protobuf.configure();

export type UInt64Value = number | string | Long;

export enum DeltaOpType {
  ADD = 0,
  REPLACE = 1,
  REMOVE = 2,
  MERGE = 3,
}

export enum ErrorCode {
  UNKNOWN = 0,
  INVALID_CONTEXT = 1,
  UNAUTHORIZED = 2,
  TOOL_NOT_FOUND = 3,
  TIMEOUT = 4,
  UNSUPPORTED_VERSION = 5,
  BAD_DELTA = 6,
  PAYLOAD_TOO_LARGE = 7,
  MALFORMED_REQUEST = 8,
  TOO_MANY_REQUESTS = 9,
  AUTH_SIGNATURE_INVALID = 10,
  AUTH_REPLAY_DETECTED = 11,
  PROFILE_MISMATCH = 12,
  PROFILE_UNSUPPORTED = 13,
  PROFILE_NEGOTIATION_FAILED = 14,
  MISSING_PATCH_RANGE = 15,
  AUTH_DID_INVALID = 17,
  AUTH_TIMESTAMP_EXPIRED = 18,
}

export interface DeltaOp {
  op?: DeltaOpType | number;
  path?: string;
  data?: Uint8Array;
  ts?: UInt64Value;
}

export interface ContextPatch {
  contextId?: string;
  baseVersion?: UInt64Value;
  ops?: DeltaOp[];
}

export interface ContextGraphVersion {
  contextId?: string;
  version?: UInt64Value;
}

export interface SyncSubscribe {
  from?: ContextGraphVersion;
}

export interface SyncRequest {
  missingFrom?: ContextGraphVersion;
  toVersion?: UInt64Value;
}

export interface RetryEnvelope {
  bufferedPatches?: ContextPatch[];
  ttlMs?: number;
}

export interface CapabilityOffer {
  desc?: CapabilityDescriptor;
}

export interface CapabilityRequest {
  ids?: string[];
}

export interface CapabilityAck {
  accepted?: string[];
}

export interface CapabilityMessage {
  offer?: CapabilityOffer;
  request?: CapabilityRequest;
  ack?: CapabilityAck;
}

export interface CapabilityDescriptor {
  toolId?: string;
  inputSchema?: string;
  outputSchema?: string;
  timeoutMs?: number;
  resourceHint?: string;
  authScope?: string[];
  descriptorVersion?: string;
}

export interface ProfileNegotiate {
  supportedMask?: number;
  minRequired?: number;
}

export interface ProfileAck {
  agreedProfile?: number;
}

export interface RoutePolicyMessage {
  policyId?: string;
  wasmBlob?: Uint8Array;
  ttlMs?: number;
}

export interface SystemStats {
  cpuPercent?: number;
  memBytes?: UInt64Value;
  temperatureC?: number;
}

export interface TokenUsage {
  promptTokens?: number;
  completionTokens?: number;
}

export interface TelemetryDatagram {
  timestampMs?: UInt64Value;
  system?: SystemStats;
  tokens?: TokenUsage;
}

export interface ErrorMessage {
  code?: ErrorCode | number;
  reason?: string;
  diagnostics?: Uint8Array;
}

export interface McpJsonBlob {
  json?: Uint8Array;
}

export interface A2AJsonBlob {
  json?: Uint8Array;
}

export interface AxcpEnvelope {
  version?: number;
  traceId?: string;
  profile?: number;
  contextPatch?: ContextPatch;
  capabilityMsg?: CapabilityMessage;
  routeMsg?: RoutePolicyMessage;
  error?: ErrorMessage;
  profileNeg?: ProfileNegotiate;
  profileAck?: ProfileAck;
  retryEnv?: RetryEnvelope;
  telemetry?: TelemetryDatagram;
  senderDid?: string;
  timestampMs?: UInt64Value;
  sequence?: UInt64Value;
  recipientDid?: string;
  signature?: Uint8Array;
  attestationProof?: Uint8Array;
}

export const root = new protobuf.Root();
const axcp = root.define("axcp").define("v0_1");

const deltaOp = new protobuf.Type("DeltaOp")
  .add(new protobuf.Enum("OpType", { ADD: 0, REPLACE: 1, REMOVE: 2, MERGE: 3 }))
  .add(new protobuf.Field("op", 1, "OpType"))
  .add(new protobuf.Field("path", 2, "string"))
  .add(new protobuf.Field("data", 3, "bytes"))
  .add(new protobuf.Field("ts", 4, "uint64"));

const contextPatch = new protobuf.Type("ContextPatch")
  .add(new protobuf.Field("contextId", 1, "string"))
  .add(new protobuf.Field("baseVersion", 2, "uint64"))
  .add(new protobuf.Field("ops", 3, "DeltaOp", "repeated"));

const contextGraphVersion = new protobuf.Type("ContextGraphVersion")
  .add(new protobuf.Field("contextId", 1, "string"))
  .add(new protobuf.Field("version", 2, "uint64"));

const syncSubscribe = new protobuf.Type("SyncSubscribe").add(
  new protobuf.Field("from", 1, "ContextGraphVersion"),
);

const syncRequest = new protobuf.Type("SyncRequest")
  .add(new protobuf.Field("missingFrom", 1, "ContextGraphVersion"))
  .add(new protobuf.Field("toVersion", 2, "uint64"));

const retryEnvelope = new protobuf.Type("RetryEnvelope")
  .add(new protobuf.Field("bufferedPatches", 1, "ContextPatch", "repeated"))
  .add(new protobuf.Field("ttlMs", 2, "uint32"));

const capabilityDescriptor = new protobuf.Type("CapabilityDescriptor")
  .add(new protobuf.Field("toolId", 1, "string"))
  .add(new protobuf.Field("inputSchema", 2, "string"))
  .add(new protobuf.Field("outputSchema", 3, "string"))
  .add(new protobuf.Field("timeoutMs", 4, "uint32"))
  .add(new protobuf.Field("resourceHint", 5, "string"))
  .add(new protobuf.Field("authScope", 6, "string", "repeated"))
  .add(new protobuf.Field("descriptorVersion", 7, "string"));

const capabilityOffer = new protobuf.Type("CapabilityOffer").add(
  new protobuf.Field("desc", 1, "CapabilityDescriptor"),
);

const capabilityRequest = new protobuf.Type("CapabilityRequest").add(
  new protobuf.Field("ids", 1, "string", "repeated"),
);

const capabilityAck = new protobuf.Type("CapabilityAck").add(
  new protobuf.Field("accepted", 1, "string", "repeated"),
);

const capabilityMessage = new protobuf.Type("CapabilityMessage")
  .add(new protobuf.OneOf("kind", ["offer", "request", "ack"]))
  .add(new protobuf.Field("offer", 1, "CapabilityOffer"))
  .add(new protobuf.Field("request", 2, "CapabilityRequest"))
  .add(new protobuf.Field("ack", 3, "CapabilityAck"));

const profileNegotiate = new protobuf.Type("ProfileNegotiate")
  .add(new protobuf.Field("supportedMask", 1, "uint32"))
  .add(new protobuf.Field("minRequired", 2, "uint32"));

const profileAck = new protobuf.Type("ProfileAck").add(new protobuf.Field("agreedProfile", 1, "uint32"));

const routePolicyMessage = new protobuf.Type("RoutePolicyMessage")
  .add(new protobuf.Field("policyId", 1, "string"))
  .add(new protobuf.Field("wasmBlob", 2, "bytes"))
  .add(new protobuf.Field("ttlMs", 3, "uint32"));

const systemStats = new protobuf.Type("SystemStats")
  .add(new protobuf.Field("cpuPercent", 1, "uint32"))
  .add(new protobuf.Field("memBytes", 2, "uint64"))
  .add(new protobuf.Field("temperatureC", 3, "uint32"));

const tokenUsage = new protobuf.Type("TokenUsage")
  .add(new protobuf.Field("promptTokens", 1, "uint32"))
  .add(new protobuf.Field("completionTokens", 2, "uint32"));

const telemetryDatagram = new protobuf.Type("TelemetryDatagram")
  .add(new protobuf.OneOf("payload", ["system", "tokens"]))
  .add(new protobuf.Field("timestampMs", 1, "uint64"))
  .add(new protobuf.Field("system", 10, "SystemStats"))
  .add(new protobuf.Field("tokens", 11, "TokenUsage"));

const errorMessage = new protobuf.Type("ErrorMessage")
  .add(new protobuf.Field("code", 1, "uint32"))
  .add(new protobuf.Field("reason", 2, "string"))
  .add(new protobuf.Field("diagnostics", 3, "bytes"));

const mcpJsonBlob = new protobuf.Type("McpJsonBlob").add(new protobuf.Field("json", 1, "bytes"));
const a2aJsonBlob = new protobuf.Type("A2AJsonBlob").add(new protobuf.Field("json", 1, "bytes"));

const axcpEnvelope = new protobuf.Type("AxcpEnvelope")
  .add(
    new protobuf.OneOf("payload", [
      "contextPatch",
      "capabilityMsg",
      "routeMsg",
      "error",
      "profileNeg",
      "profileAck",
      "retryEnv",
      "telemetry",
    ]),
  )
  .add(new protobuf.Field("version", 1, "uint32"))
  .add(new protobuf.Field("traceId", 2, "string"))
  .add(new protobuf.Field("profile", 3, "uint32"))
  .add(new protobuf.Field("contextPatch", 4, "ContextPatch"))
  .add(new protobuf.Field("capabilityMsg", 5, "CapabilityMessage"))
  .add(new protobuf.Field("routeMsg", 6, "RoutePolicyMessage"))
  .add(new protobuf.Field("error", 7, "ErrorMessage"))
  .add(new protobuf.Field("profileNeg", 8, "ProfileNegotiate"))
  .add(new protobuf.Field("profileAck", 9, "ProfileAck"))
  .add(new protobuf.Field("retryEnv", 10, "RetryEnvelope"))
  .add(new protobuf.Field("telemetry", 11, "TelemetryDatagram"))
  .add(new protobuf.Field("senderDid", 20, "string"))
  .add(new protobuf.Field("timestampMs", 21, "uint64"))
  .add(new protobuf.Field("sequence", 22, "uint64"))
  .add(new protobuf.Field("recipientDid", 23, "string"))
  .add(new protobuf.Field("signature", 100, "bytes"))
  .add(new protobuf.Field("attestationProof", 101, "bytes"));

axcp
  .add(deltaOp)
  .add(contextPatch)
  .add(contextGraphVersion)
  .add(syncSubscribe)
  .add(syncRequest)
  .add(retryEnvelope)
  .add(capabilityDescriptor)
  .add(capabilityOffer)
  .add(capabilityRequest)
  .add(capabilityAck)
  .add(capabilityMessage)
  .add(profileNegotiate)
  .add(profileAck)
  .add(routePolicyMessage)
  .add(systemStats)
  .add(tokenUsage)
  .add(telemetryDatagram)
  .add(errorMessage)
  .add(mcpJsonBlob)
  .add(a2aJsonBlob)
  .add(axcpEnvelope);

root.resolveAll();

export const AxcpEnvelopeType = root.lookupType("axcp.v0_1.AxcpEnvelope");
export const ContextPatchType = root.lookupType("axcp.v0_1.ContextPatch");
export const DeltaOpTypeMessage = root.lookupType("axcp.v0_1.DeltaOp");

export function toSafeNumber(value: UInt64Value | undefined, fieldName: string): number {
  if (value === undefined) {
    return 0;
  }
  if (Long.isLong(value)) {
    const asNumber = value.toNumber();
    if (!Number.isSafeInteger(asNumber)) {
      throw new RangeError(`${fieldName} exceeds Number.MAX_SAFE_INTEGER`);
    }
    return asNumber;
  }
  if (typeof value === "bigint") {
    if (value > BigInt(Number.MAX_SAFE_INTEGER) || value < BigInt(Number.MIN_SAFE_INTEGER)) {
      throw new RangeError(`${fieldName} exceeds Number.MAX_SAFE_INTEGER`);
    }
    return Number(value);
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (!Number.isSafeInteger(parsed)) {
      throw new RangeError(`${fieldName} must be a safe integer string`);
    }
    return parsed;
  }
  if (!Number.isSafeInteger(value)) {
    throw new RangeError(`${fieldName} must be a safe integer`);
  }
  return value;
}
