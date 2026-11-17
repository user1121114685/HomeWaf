import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { advisory, verifyMitigation } from './picomatch-security.mjs';

const root = fileURLToPath(new URL('..', import.meta.url));

export function evaluateAudit(report, verifyNode) {
  if (report?.error || report?.auditReportVersion !== 2 || !report.vulnerabilities ||
      typeof report.vulnerabilities !== 'object' || Array.isArray(report.vulnerabilities) ||
      !Number.isInteger(report.metadata?.vulnerabilities?.total)) {
    throw new Error('Missing or invalid npm audit response; the audit did not pass');
  }
  const entries = Object.entries(report.vulnerabilities);
  if (entries.length !== report.metadata.vulnerabilities.total) {
    throw new Error('Inconsistent npm audit vulnerability count');
  }
  const mitigated = new Set();
  const checking = new Set();
  const checked = new Map();
  function covered(name) {
    if (checked.has(name)) return checked.get(name);
    const item = report.vulnerabilities[name];
    if (!item || checking.has(name) || !Array.isArray(item.via) || !item.via.length) return false;
    checking.add(name);
    const results = item.via.map(source => {
      if (typeof source === 'string') return covered(source);
      if (name !== 'picomatch' || source?.name !== 'picomatch' || source.dependency !== 'picomatch' ||
          source.url !== advisory || source.range !== '<=4.0.7' || source.severity !== 'high' ||
          !Array.isArray(item.nodes) || !item.nodes.length) return false;
      for (const node of item.nodes) verifyNode(node);
      mitigated.add(advisory);
      return true;
    });
    checking.delete(name);
    const result = results.every(Boolean);
    checked.set(name, result);
    return result;
  }
  const unresolved = entries.filter(([name]) => !covered(name)).map(([name]) => name);
  return { unresolved, mitigated: [...mitigated], affectedPackages: entries.length };
}

export function auditDependencies() {
  // npm supplies npm_execpath when this is invoked by npm run audit:security.
  const npmCli = process.env.npm_execpath || path.join(path.dirname(process.execPath), 'node_modules/npm/bin/npm-cli.js');
  const command = fs.existsSync(npmCli) ? process.execPath : 'npm';
  const args = [...(command === process.execPath ? [npmCli] : []), 'audit', '--json', '--audit-level=low'];
  const result = spawnSync(command, args, { cwd: root, encoding: 'utf8', windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
  if (result.error) throw result.error;
  if (result.status !== 0 && result.status !== 1) throw new Error(`npm audit failed: ${result.stderr || result.stdout}`);
  const report = JSON.parse(result.stdout);
  if (result.status === 1 && report?.metadata?.vulnerabilities?.total === 0) {
    throw new Error('npm audit exited unsuccessfully without reporting vulnerabilities');
  }
  const checked = evaluateAudit(report, node => {
    if (typeof node !== 'string' || !node.startsWith('node_modules/')) throw new Error('Invalid vulnerable package path');
    const directory = fs.realpathSync(path.resolve(root, node));
    if (!directory.startsWith(path.join(fs.realpathSync(root), 'node_modules') + path.sep)) {
      throw new Error('Vulnerable package resolved outside node_modules');
    }
    verifyMitigation(directory);
  });
  if (checked.mitigated.length) {
    console.log(`npm reports ${checked.affectedPackages} affected packages. ${advisory} is locally mitigated; picomatch remains upstream-unfixed.`);
    console.log('Verified installed package version and patched source SHA-256. ReDoS regressions run before this audit.');
  }
  if (checked.unresolved.length) {
    console.error(JSON.stringify(report, null, 2));
    throw new Error(`Unmitigated npm vulnerabilities: ${checked.unresolved.join(', ')}`);
  }
  console.log('Dependency security gate passed: no unmitigated reported vulnerabilities.');
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) auditDependencies();
