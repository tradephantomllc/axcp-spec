import { InvalidPrivateKeyError } from "./errors.js";

const BASE64_RE = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;

export function toUint8Array(value: Uint8Array | readonly number[] | undefined): Uint8Array {
  if (value === undefined) {
    return new Uint8Array();
  }
  if (value instanceof Uint8Array) {
    return new Uint8Array(value);
  }
  return Uint8Array.from(value);
}

export function bytesEqual(left: Uint8Array, right: Uint8Array): boolean {
  if (left.length !== right.length) {
    return false;
  }
  let diff = 0;
  for (let index = 0; index < left.length; index += 1) {
    diff |= left[index]! ^ right[index]!;
  }
  return diff === 0;
}

export function encodeBase64(data: Uint8Array): string {
  return Buffer.from(data).toString("base64");
}

export function decodeBase64Strict(value: string, fieldName: string): Uint8Array {
  if (!value || value.length % 4 !== 0 || !BASE64_RE.test(value)) {
    throw new InvalidPrivateKeyError(`${fieldName} must be valid standard base64`);
  }
  return new Uint8Array(Buffer.from(value, "base64"));
}

export function utf8Bytes(value: string): Uint8Array {
  return new Uint8Array(Buffer.from(value, "utf8"));
}
