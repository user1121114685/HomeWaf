// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

// Wire-compatible NaCl box/secretbox adapter using maintained noble primitives.
// The existing backend's swt2 protocol uses X25519 -> HSalsa20 -> XSalsa20-Poly1305.
import { x25519 } from '@noble/curves/ed25519.js';
import { hsalsa, secretbox } from '@noble/ciphers/salsa.js';
const randomBytes = (length: number) => crypto.getRandomValues(new Uint8Array(length));
function before(publicKey: Uint8Array, secretKey: Uint8Array) {
  const shared = x25519.getSharedSecret(secretKey, publicKey);
  const input = new DataView(shared.buffer, shared.byteOffset, shared.byteLength);
  const words = new Uint32Array(8);
  for (let i = 0; i < 8; i++) words[i] = input.getUint32(i * 4, true);
  const output = new Uint32Array(8);
  hsalsa(new Uint32Array([0x61707865, 0x3320646e, 0x79622d32, 0x6b206574]), words, new Uint32Array(4), output);
  const result = new Uint8Array(32);
  const view = new DataView(result.buffer);
  for (let i = 0; i < 8; i++) view.setUint32(i * 4, output[i], true);
  shared.fill(0);
  words.fill(0);
  output.fill(0);
  return result;
}
const seal = Object.assign(
  (plain: Uint8Array, nonce: Uint8Array, key: Uint8Array) => secretbox(key, nonce).seal(plain),
  {
    open: (cipher: Uint8Array, nonce: Uint8Array, key: Uint8Array) => {
      try {
        return secretbox(key, nonce).open(cipher);
      } catch {
        return null;
      }
    },
  },
);
export const transportKeyPair = () => x25519.keygen();

export function toBase64(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

export function fromBase64(value: string): Uint8Array {
  return Uint8Array.from(atob(value), (byte) => byte.charCodeAt(0));
}

export class TransportSession {
  readonly id: string;
  #key: Uint8Array;

  constructor(id: string, peerPublicKey: Uint8Array, secretKey: Uint8Array) {
    if (!/^[A-Za-z0-9_-]+$/.test(id) || peerPublicKey.length !== 32) throw new Error('无效的管理会话');
    this.id = id;
    this.#key = before(peerPublicKey, secretKey);
  }

  encrypt(plain: string): string {
    const nonce = randomBytes(24);
    const cipher = seal(new TextEncoder().encode(plain), nonce, this.#key);
    const payload = new Uint8Array(nonce.length + cipher.length);
    payload.set(nonce);
    payload.set(cipher, nonce.length);
    return `swt2:${this.id}:${toBase64(payload)}`;
  }

  decrypt(encrypted: string): string {
    const prefix = `swt2:${this.id}:`;
    if (typeof encrypted !== 'string' || !encrypted.startsWith(prefix)) throw new Error('管理响应的会话标识无效');
    const payload = fromBase64(encrypted.slice(prefix.length));
    const plain = seal.open(payload.subarray(24), payload.subarray(0, 24), this.#key);
    if (!plain) throw new Error('管理响应校验失败');
    return new TextDecoder('utf-8', { fatal: true }).decode(plain);
  }
}
