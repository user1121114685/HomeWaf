import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { evaluateAudit } from './audit-dependencies.mjs';
import { advisory, applyMitigation, guard, hash, upstreamHash, verifyMitigation } from './picomatch-security.mjs';

const root = fileURLToPath(new URL('..', import.meta.url));
const require = createRequire(import.meta.url);
const packageDir = path.join(root, 'node_modules/picomatch');

test('installed picomatch rejects repeated wildcard ReDoS patterns within a bounded subprocess', () => {
  verifyMitigation(packageDir);
  execFileSync(process.execPath, ['--input-type=commonjs', '-e', `
    const assert = require('node:assert/strict');
    const pm = require(${JSON.stringify(require.resolve('picomatch'))});
    for (const pattern of ['+(*)b', '+(*a)', '*(*a)', '+(a*)b', '+(a*b)c', '+(**a)', '+(*?a)', '+(*[ab])c', '+(*a/b)c', 'x+(*a)y', '+(*a|b)']) {
      for (const options of [{}, { capture: true }, { dot: true }, { maxExtglobRecursion: false }]) {
        assert.equal(pm.isMatch('a'.repeat(60) + '!', pattern, options), false, pattern);
      }
    }
  `], { cwd: root, timeout: 3000, windowsHide: true, stdio: 'pipe' });
});

test('ordinary build globs, literal repeats and escaped stars keep matching', () => {
  const pm = require('picomatch');
  for (const [input, pattern, expected] of [
    ['src/pages/Home.vue', 'src/**/*.vue', true],
    ['src/pages/Home.ts', 'src/**/*.vue', false],
    ['index.ts', '*.{js,ts}', true],
    ['foobarfoo', '+(foo|bar)', true],
    ['abc', '+(foo|bar)', false],
    ['*a*a', '+(\\*a)', true],
    ['*a*a', '+("*a")', true],
  ]) assert.equal(pm.isMatch(input, pattern), expected, pattern);
});

test('mitigation is idempotent and refuses unreviewed versions or modified sources', t => {
  const cache = path.join(root, '.cache/security-tests');
  fs.mkdirSync(cache, { recursive: true });
  const fixture = fs.mkdtempSync(path.join(cache, 'picomatch-'));
  t.after(() => fs.rmSync(fixture, { recursive: true, force: true }));
  fs.cpSync(packageDir, fixture, { recursive: true });
  const file = path.join(fixture, 'lib/parse.js');
  const original = fs.readFileSync(file, 'utf8').replace(guard, '');
  assert.equal(hash(original), upstreamHash);
  fs.writeFileSync(file, original);
  assert.throws(() => verifyMitigation(fixture), /missing or changed/);
  applyMitigation(fixture);
  applyMitigation(fixture);
  verifyMitigation(fixture);
  fs.appendFileSync(file, '\n// unexpected change\n');
  assert.throws(() => verifyMitigation(fixture), /missing or changed/);
  assert.throws(() => applyMitigation(fixture), /unreviewed file/);
  fs.writeFileSync(path.join(fixture, 'package.json'), JSON.stringify({ name: 'picomatch', version: '4.0.8' }));
  assert.throws(() => applyMitigation(fixture), /changing its version/);
});

const source = { name: 'picomatch', dependency: 'picomatch', url: advisory, range: '<=4.0.7', severity: 'high' };
function report(extra = []) {
  return {
    auditReportVersion: 2,
    metadata: { vulnerabilities: { total: 2 } },
    vulnerabilities: {
      picomatch: { via: [source, ...extra], nodes: ['node_modules/picomatch'] },
      vite: { via: ['picomatch'], nodes: ['node_modules/vite'] },
    },
  };
}

test('audit accepts only this advisory after verifying every installed copy', () => {
  const nodes = [];
  const result = evaluateAudit(report(), node => nodes.push(node));
  assert.deepEqual(result.unresolved, []);
  assert.deepEqual(result.mitigated, [advisory]);
  assert.deepEqual(nodes, ['node_modules/picomatch']);
  const nested = report();
  nested.vulnerabilities.picomatch.nodes.push('node_modules/tool/node_modules/picomatch');
  assert.throws(() => evaluateAudit(nested, node => {
    if (node.includes('/tool/')) throw new Error('unpatched nested dependency');
  }), /unpatched nested dependency/);
});

test('new advisories, changed ranges and unresolved dependency edges still fail the gate', () => {
  const extra = { ...source, url: 'https://github.com/advisories/GHSA-new-advisory' };
  assert.deepEqual(evaluateAudit(report([extra]), () => {}).unresolved.sort(), ['picomatch', 'vite']);
  const changed = report();
  changed.vulnerabilities.picomatch.via = [{ ...source, range: '<=4.0.8' }];
  assert.deepEqual(evaluateAudit(changed, () => {}).unresolved.sort(), ['picomatch', 'vite']);
  const cycle = report();
  cycle.vulnerabilities.picomatch.via = ['vite'];
  assert.equal(evaluateAudit(cycle, () => {}).unresolved.length, 2);
});

test('network failures and incomplete audit responses cannot become a clean result', () => {
  for (const invalid of [null, {}, { error: { message: 'Registry unavailable' } }, { ...report(), vulnerabilities: {} }]) {
    assert.throws(() => evaluateAudit(invalid, () => {}));
  }
  assert.deepEqual(evaluateAudit({ auditReportVersion: 2, metadata: { vulnerabilities: { total: 0 } }, vulnerabilities: {} }, () => {}).unresolved, []);
});
