// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// Legacy AES transport is retained solely to migrate existing backend sessions.
import { cbc } from '@noble/ciphers/aes.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex } from '@noble/hashes/utils.js';
const key = new TextEncoder().encode('7E@u*has$d*@s5YX');
const toBase64 = (data: Uint8Array) => {
  let s = '';
  for (const b of data) s += String.fromCharCode(b);
  return btoa(s);
};
const fromBase64 = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
export function AesEncrypt(text: string) {
  const iv = crypto.getRandomValues(new Uint8Array(16));
  const encrypted = cbc(key, iv).encrypt(new TextEncoder().encode(text));
  const data = new Uint8Array(iv.length + encrypted.length);
  data.set(iv);
  data.set(encrypted, iv.length);
  return toBase64(data);
}
export function AesDecrypt(text: string) {
  const data = fromBase64(text);
  if (data.length < 32 || data.length % 16) throw new Error('Invalid legacy ciphertext');
  return new TextDecoder('utf-8', { fatal: true }).decode(cbc(key, data.slice(0, 16)).decrypt(data.slice(16)));
}
export const hashBytes = (data: ArrayBuffer) => bytesToHex(sha256(new Uint8Array(data)));
