import { InvalidDIDFormat } from "./errors.js";

const BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
const BASE58_INDEX = new Map<string, number>(
  [...BASE58_ALPHABET].map((char, index) => [char, index]),
);

export function base58Encode(data: Uint8Array): string {
  let value = 0n;
  for (const byte of data) {
    value = (value << 8n) + BigInt(byte);
  }

  let encoded = "";
  while (value > 0n) {
    const remainder = Number(value % 58n);
    value /= 58n;
    encoded = BASE58_ALPHABET[remainder] + encoded;
  }

  let leadingZeroes = 0;
  for (const byte of data) {
    if (byte !== 0) {
      break;
    }
    leadingZeroes += 1;
  }

  return "1".repeat(leadingZeroes) + encoded;
}

export function base58Decode(value: string): Uint8Array {
  let decoded = 0n;
  for (const char of value) {
    const digit = BASE58_INDEX.get(char);
    if (digit === undefined) {
      throw new InvalidDIDFormat(`invalid base58btc character: ${JSON.stringify(char)}`);
    }
    decoded = decoded * 58n + BigInt(digit);
  }

  const bytes: number[] = [];
  while (decoded > 0n) {
    bytes.unshift(Number(decoded & 0xffn));
    decoded >>= 8n;
  }

  let leadingZeroes = 0;
  for (const char of value) {
    if (char !== "1") {
      break;
    }
    leadingZeroes += 1;
  }

  return Uint8Array.from([...new Array<number>(leadingZeroes).fill(0), ...bytes]);
}
