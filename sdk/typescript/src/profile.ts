import {
  DeprecatedProfileError,
  InvalidCapabilitiesError,
  NegotiationError,
  NoProfileOverlapError,
  NoSignatureAlgorithmOverlapError,
} from "./errors.js";

export enum Profile {
  SECURE_BASELINE = "secure-baseline-v1",
  TRANSPORT_ONLY = "transport-only-v0",
}

export const PROTOCOL_VERSION_V1 = "1";
export const SIG_ALGO_ED25519 = "ed25519";

export interface Capabilities {
  readonly requireAuth: boolean;
  readonly requireReplay: boolean;
  readonly sigAlgos: readonly string[];
}

export interface ClientHello {
  readonly version: string;
  readonly profiles: readonly Profile[];
  readonly capabilities: Capabilities;
}

export interface ServerHello {
  readonly version: string;
  readonly profile: Profile;
  readonly capabilities: Capabilities;
}

export interface NegotiateOptions {
  readonly serverProfiles?: readonly Profile[];
  readonly serverCapabilities?: Capabilities;
  readonly allowDeprecated?: boolean;
}

export const DEFAULT_CAPABILITIES: Capabilities = Object.freeze({
  requireAuth: true,
  requireReplay: true,
  sigAlgos: Object.freeze([SIG_ALGO_ED25519]),
});

export const DEFAULT_CLIENT_HELLO: ClientHello = Object.freeze({
  version: PROTOCOL_VERSION_V1,
  profiles: Object.freeze([Profile.SECURE_BASELINE]),
  capabilities: DEFAULT_CAPABILITIES,
});

export function negotiate(client: ClientHello = DEFAULT_CLIENT_HELLO, options: NegotiateOptions = {}): ServerHello {
  const serverProfiles = options.serverProfiles ?? [Profile.SECURE_BASELINE];
  const serverCapabilities = options.serverCapabilities ?? DEFAULT_CAPABILITIES;
  const allowDeprecated = options.allowDeprecated ?? false;

  if (client.version !== PROTOCOL_VERSION_V1) {
    throw new NegotiationError("unsupported protocol version");
  }
  if (client.profiles.length === 0) {
    throw new NoProfileOverlapError("client did not provide profiles");
  }

  const selected = selectProfile(client.profiles, serverProfiles, allowDeprecated);
  validateCapabilities(selected, client.capabilities);
  const selectedAlgo = resolveSignatureAlgorithm(client.capabilities.sigAlgos, serverCapabilities.sigAlgos);

  return {
    version: PROTOCOL_VERSION_V1,
    profile: selected,
    capabilities: {
      requireAuth: client.capabilities.requireAuth || profileRequiresAuth(selected),
      requireReplay: client.capabilities.requireReplay || profileRequiresReplay(selected),
      sigAlgos: [selectedAlgo],
    },
  };
}

function selectProfile(
  clientProfiles: readonly Profile[],
  serverProfiles: readonly Profile[],
  allowDeprecated: boolean,
): Profile {
  const serverProfileSet = new Set(serverProfiles);
  if (clientProfiles.includes(Profile.SECURE_BASELINE) && serverProfileSet.has(Profile.SECURE_BASELINE)) {
    return Profile.SECURE_BASELINE;
  }
  if (clientProfiles.includes(Profile.TRANSPORT_ONLY) && serverProfileSet.has(Profile.TRANSPORT_ONLY)) {
    if (!allowDeprecated) {
      throw new DeprecatedProfileError("transport-only-v0 is deprecated");
    }
    return Profile.TRANSPORT_ONLY;
  }
  throw new NoProfileOverlapError("no common AXCP profile");
}

function validateCapabilities(profile: Profile, capabilities: Capabilities): void {
  switch (profile) {
    case Profile.SECURE_BASELINE:
      if (capabilities.sigAlgos.length === 0) {
        throw new InvalidCapabilitiesError("secure-baseline-v1 requires a signature algorithm");
      }
      return;
    case Profile.TRANSPORT_ONLY:
      return;
    default:
      throw new NoProfileOverlapError(`unknown AXCP profile: ${String(profile)}`);
  }
}

function resolveSignatureAlgorithm(clientAlgos: readonly string[], serverAlgos: readonly string[]): string {
  if (clientAlgos.includes(SIG_ALGO_ED25519) && serverAlgos.includes(SIG_ALGO_ED25519)) {
    return SIG_ALGO_ED25519;
  }
  throw new NoSignatureAlgorithmOverlapError("no common signature algorithm");
}

function profileRequiresAuth(profile: Profile): boolean {
  switch (profile) {
    case Profile.SECURE_BASELINE:
      return true;
    case Profile.TRANSPORT_ONLY:
      return false;
    default:
      throw new NoProfileOverlapError(`unknown AXCP profile: ${String(profile)}`);
  }
}

function profileRequiresReplay(profile: Profile): boolean {
  switch (profile) {
    case Profile.SECURE_BASELINE:
      return true;
    case Profile.TRANSPORT_ONLY:
      return false;
    default:
      throw new NoProfileOverlapError(`unknown AXCP profile: ${String(profile)}`);
  }
}
