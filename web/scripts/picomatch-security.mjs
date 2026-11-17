import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

export const advisory = 'https://github.com/advisories/GHSA-fc5q-39xc-vjpf';
export const upstreamHash = '4e1e05447668aa1e94366c5516a2bcd8f5fff800cc92b1d0cd7d77703b8618c4';
export const patchedHash = '8aacc5bdbedff88f9712160f39436430d8df0bde446d1d658e75916500c4d08a';
export const hash = source => createHash('sha256').update(source).digest('hex');

// HomeWaf's build does not need wildcard bodies inside repeated extglobs.
// Literalize those patterns using picomatch's existing risky-pattern fallback.
// This restriction also applies when a caller disables the upstream recursion guard.
// Remove this version-locked mitigation once an upstream fix is reviewed and tested.
export const guard = `  // HomeWaf mitigation for GHSA-fc5q-39xc-vjpf; retain the upstream MIT license.
  let quoted = false;
  let bracket = false;
  for (let i = 0; i < body.length; i++) {
    const ch = body[i];
    if (ch === '\\\\') { i++; continue; }
    if (ch === '"' && !bracket) { quoted = !quoted; continue; }
    if (quoted) continue;
    if (ch === '[') { bracket = true; continue; }
    if (ch === ']' && bracket) { bracket = false; continue; }
    if (ch === '*' && !bracket) return { risky: true };
  }

`;
const anchor = 'const analyzeRepeatedExtglob = (body, options) => {\n';

function readPackage(directory) {
  const manifest = JSON.parse(fs.readFileSync(path.join(directory, 'package.json'), 'utf8'));
  if (manifest.name !== 'picomatch' || manifest.version !== '4.0.7') {
    throw new Error('Review the picomatch security mitigation before changing its version');
  }
  return path.join(directory, 'lib', 'parse.js');
}

export function applyMitigation(directory) {
  const file = readPackage(directory);
  const source = fs.readFileSync(file, 'utf8');
  if (hash(source) === patchedHash) return;
  if (hash(source) !== upstreamHash || source.split(anchor).length !== 2) {
    throw new Error('Unexpected picomatch source; refusing to patch an unreviewed file');
  }
  const patched = source.replace(anchor, anchor + guard);
  if (hash(patched) !== patchedHash) throw new Error('Invalid picomatch security patch');
  fs.writeFileSync(file, patched);
}

export function verifyMitigation(directory) {
  const file = readPackage(directory);
  if (hash(fs.readFileSync(file)) !== patchedHash) {
    throw new Error('picomatch security mitigation is missing or changed; run npm ci');
  }
}
