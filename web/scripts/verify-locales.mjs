import fs from 'node:fs';
import { pathToFileURL, fileURLToPath } from 'node:url';
import { baseCompile } from '@intlify/message-compiler';

let checked = 0;
const errors = [];
function visit(value, key) {
  if (typeof value === 'string') {
    checked++;
    baseCompile(value, { onError: (error) => errors.push({ key, message: error.message }) });
  } else if (value && typeof value === 'object') {
    for (const [name, child] of Object.entries(value)) visit(child, `${key}.${name}`);
  }
}
for (const locale of ['zh_CN', 'en_US']) {
  const source = new URL(`../src/i18n/${locale}.js`, import.meta.url);
  const dependency = pathToFileURL(fileURLToPath(new URL('../src/i18n/logExport.js', import.meta.url))).href;
  const code = fs.readFileSync(source, 'utf8').replace("'./logExport'", JSON.stringify(dependency));
  const module = await import('data:text/javascript;base64,' + Buffer.from(code).toString('base64'));
  visit(module.default, locale);
}
console.log(JSON.stringify({ checked, errors }, null, 2));
if (errors.length) process.exitCode = 1;
