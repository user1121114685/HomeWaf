import { createHash } from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Only upstream data is downloaded. Never execute programs from the corpus.
// CI uses a reviewed immutable revision; the daily job also checks current main.
const repository = 'coreruleset/coreruleset';
const pinnedCommit = 'e03a4f6dabc7a30ebd8c52c97d28a154f590a48f'; // CRS v4.30.0, matching the bundled rules.
const sources = [
  ['sqli', 'REQUEST-942-APPLICATION-ATTACK-SQLI/942100.yaml'],
  ['xss', 'REQUEST-941-APPLICATION-ATTACK-XSS/941100.yaml'],
  ['lfi', 'REQUEST-930-APPLICATION-ATTACK-LFI/930120.yaml'],
];
const root = fileURLToPath(new URL('..', import.meta.url));
const output = path.join(root, '.cache', process.argv.includes('--latest') ? 'waf-corpus-latest' : 'waf-corpus');

async function download(url) {
  const response = await fetch(url, { signal: AbortSignal.timeout(30000), redirect: 'error', headers: { 'User-Agent': 'HomeWaf-security-regression' } });
  if (!response.ok) throw new Error(`Corpus download failed (${response.status}): ${url}`);
  const data = Buffer.from(await response.arrayBuffer());
  if (data.length === 0 || data.length > 2 * 1024 * 1024) throw new Error(`Unexpected corpus size: ${url}`);
  return data;
}

let commit = pinnedCommit;
if (process.argv.includes('--latest')) {
  const info = JSON.parse(await download(`https://api.github.com/repos/${repository}/commits/main`));
  if (!/^[0-9a-f]{40}$/.test(info.sha)) throw new Error('Invalid upstream commit');
  commit = info.sha;
}
await fs.mkdir(output, { recursive: true });
// Retain the source license next to downloaded tests, including their author metadata.
await fs.writeFile(path.join(output, 'LICENSE'), await download(`https://raw.githubusercontent.com/${repository}/${commit}/LICENSE`));
const files = [];
for (const [category, source] of sources) {
  const data = await download(`https://raw.githubusercontent.com/${repository}/${commit}/tests/regression/tests/${source}`);
  const name = path.basename(source);
  await fs.writeFile(path.join(output, name), data);
  files.push({ category, name, source, sha256: createHash('sha256').update(data).digest('hex') });
}
await fs.writeFile(path.join(output, 'manifest.json'), JSON.stringify({ repository, commit, files }, null, 2) + '\n');
console.log(`Downloaded OWASP CRS attack data at ${commit} into ${path.relative(root, output)}; upstream code is not executed.`);
