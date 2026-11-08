import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
const assets = fileURLToPath(new URL('../dist/assets/', import.meta.url));
let checked = 0;
for (const name of fs.readdirSync(assets)) {
  if (!name.endsWith('.js')) continue;
  const result = spawnSync(process.execPath, ['--check', path.join(assets, name)], { encoding: 'utf8' });
  if (result.status !== 0) {
    console.error(name, result.stderr.slice(-1500));
    process.exit(1);
  }
  checked++;
}
console.log(`Validated JavaScript syntax in ${checked} generated chunks`);
