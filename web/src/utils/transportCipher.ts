// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

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
export default { randomBytes, box: { keyPair: () => x25519.keygen(), before }, secretbox: seal };
