// Narrow, version-checked adapters for upstream consumers of newer dependency APIs.
// Upstream copyright and license notices remain in every patched file.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const modulePath = (...names) => path.join(root, 'node_modules', ...names);
const walkerPath = modulePath('estree-walker', 'package.json');
const walker = JSON.parse(fs.readFileSync(walkerPath, 'utf8'));
if (walker.version !== '3.0.3') throw new Error('Review the estree-walker adapter for this version');
walker.exports['.'].default = walker.exports['.'].import;
fs.writeFileSync(walkerPath, JSON.stringify(walker, null, 2) + '\n');
const core = JSON.parse(fs.readFileSync(modulePath('@vue/compiler-core/package.json'), 'utf8'));
if (core.version !== '3.5.45') throw new Error('Review the Vue entity decoder adapter for this version');
for (const name of fs.readdirSync(modulePath('@vue/compiler-core/dist'))) {
  if (!name.endsWith('.js')) continue;
  const file = modulePath('@vue/compiler-core/dist', name);
  const source = fs.readFileSync(file, 'utf8');
  let updated = source.replace(
    /new decode\.EntityDecoder\(\s*decode\.htmlDecodeTree,\s*/g,
    'new decode.HtmlEntityDecoder(',
  );
  updated = updated.replace(/new EntityDecoder\(\s*htmlDecodeTree,\s*/g, 'new HtmlEntityDecoder(');
  updated = updated.replace(/EntityDecoder,\s*htmlDecodeTree/g, 'HtmlEntityDecoder');
  updated = updated.replace(/decode\.fromCodePoint/g, 'String.fromCodePoint');
  updated = updated.replace(/,\s*fromCodePoint(?=\s*\})/g, '');
  updated = updated.replace(/(?<![.\w])fromCodePoint\(/g, 'String.fromCodePoint(');
  if (updated !== source)
    fs.writeFileSync(file, '// HomeWaf modification: adapt to entities 9 HtmlEntityDecoder.\n' + updated);
}
const sfcPath = modulePath('@vue/compiler-sfc/dist/compiler-sfc.cjs.js');
const sfc = fs.readFileSync(sfcPath, 'utf8');
const sfcUpdated = sfc.replace(
  "var MagicString = require('magic-string');",
  "var MagicStringModule = require('magic-string'); var MagicString = MagicStringModule.default || MagicStringModule;",
);
if (sfcUpdated !== sfc)
  fs.writeFileSync(sfcPath, '// HomeWaf modification: Node 26 ESM interoperability.\n' + sfcUpdated);

// Rolldown 1.3 appends a stable hash to native preload markers. Vite 8.3's
// replacement must consume that suffix too, or the generated JS is invalid.
const vite = JSON.parse(fs.readFileSync(modulePath('vite/package.json'), 'utf8'));
if (vite.version !== '8.3.6') throw new Error('Review the Rolldown preload adapter for this Vite version');
const vitePath = modulePath('vite/dist/node/chunks/node.js');
const viteSource = fs.readFileSync(vitePath, 'utf8');
const oldMarkerEnd = 's.update(markerStartPos, markerStartPos + preloadMarker.length,';
const newMarkerEnd =
  's.update(markerStartPos, markerStartPos + (/^__VITE_PRELOAD__[a-f0-9]{32}/.exec(code.slice(markerStartPos))?.[0].length ?? preloadMarker.length),';
if (viteSource.includes(oldMarkerEnd)) {
  if (viteSource.split(oldMarkerEnd).length !== 3) throw new Error('Unexpected Vite preload replacement count');
  fs.writeFileSync(
    vitePath,
    '// HomeWaf modification: consume Rolldown 1.3 preload marker hashes.\n' +
      viteSource.replaceAll(oldMarkerEnd, newMarkerEnd),
  );
} else if (!viteSource.includes(newMarkerEnd)) throw new Error('Vite preload adapter target not found');
