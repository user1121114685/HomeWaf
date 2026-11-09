// Optional end-to-end check: pass two HomeWaf binaries with increasing versions.
// Everything runs in a fresh .cache directory with loopback ports and synthetic data.
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import net from 'node:net';
import { createHash } from 'node:crypto';
import { createGzip } from 'node:zlib';
import { pipeline } from 'node:stream/promises';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const [oldInput, newInput, nextVersion = 'v2.0.1-graceful-test'] = process.argv.slice(2);
if (!oldInput || !newInput) throw new Error('Usage: node scripts/verify-graceful-update.mjs old-binary new-binary next-version');
const dir = fs.mkdtempSync(path.join(root, '.cache', 'graceful-smoke-'));
const exe = path.join(dir, process.platform === 'win32' ? 'HomeWaf.exe' : 'HomeWaf');
fs.copyFileSync(path.resolve(oldInput), exe);
fs.mkdirSync(path.join(dir, 'conf'));
const report = { requests: 0, failures: [], failureSamples: {}, versions: [], checks: [], directory: dir };
const listen = (server) => new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', () => resolve(server.address().port)); });
const freePort = async () => { const s = net.createServer(); const p = await listen(s); await new Promise((r) => s.close(r)); return p; };
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const waitFor = async (fn, timeout = 60000) => { const end = Date.now() + timeout; let last; while (Date.now() < end) { try { const value = await fn(); if (value) return value; } catch (err) { last = err; } await delay(100); } throw new Error('Timed out waiting for smoke check: ' + (last?.message || 'condition not satisfied')); };
const hash = createHash('sha256');
for await (const chunk of fs.createReadStream(path.resolve(newInput))) hash.update(chunk);
const checksum = hash.digest('base64');
const archive = path.join(dir, 'new.gz');
await pipeline(fs.createReadStream(path.resolve(newInput)), createGzip({ level: 1 }), fs.createWriteStream(archive));

const publisher = http.createServer((req, res) => {
  if (req.url.split('?')[0].endsWith('.json')) {
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({ Version: nextVersion, Sha256: checksum, Desc: 'Isolated smooth update verification' }));
  } else if (req.url.endsWith('.gz')) {
    res.setHeader('Content-Length', fs.statSync(archive).size);
    fs.createReadStream(archive).pipe(res);
  } else { res.writeHead(404); res.end(); }
});
const publisherPort = await listen(publisher);
const origin = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': req.url === '/slow' ? 'text/event-stream' : 'text/plain' });
  if (req.url === '/slow') { res.write('begin\n'); setTimeout(() => res.end('end\n'), 6500); }
  else res.end('origin-ok');
});
origin.on('upgrade', (_req, socket) => {
  socket.write('HTTP/1.1 101 Switching Protocols\r\nConnection: upgrade\r\nUpgrade: test-echo\r\n\r\n');
  socket.on('data', (data) => socket.write(data));
});
const originPort = await listen(origin), managerPort = await freePort(), proxyPort = await freePort();
fs.writeFileSync(path.join(dir, 'conf', 'config.yml'), `local_port: ${managerPort}\ndrain_time_out: 15\ndebug_worker_header: true\nupdate_restart_timeout: 90\nnotice:\n  isenable: false\n`);
const env = { ...process.env, HOMEWAF_DEV: '1', HOMEWAF_UPDATE_URL: `http://127.0.0.1:${publisherPort}/` };
const log = fs.openSync(path.join(dir, 'process.log'), 'a');
const child = spawn(exe, [], { cwd: dir, env, windowsHide: true, stdio: ['ignore', log, log] });
let key, upgraded, stopped = false;
let monitor = [];
const readState = () => JSON.parse(fs.readFileSync(path.join(dir, 'data', 'supervisor.state'), 'utf8'));
const api = async (name) => {
  const response = await fetch(`http://127.0.0.1:${managerPort}/api/v1/sysinfo/${name}`, { headers: { 'X-API-Key': key }, signal: AbortSignal.timeout(20000) });
  const body = await response.json();
  if (response.status !== 200 || body.code !== 0) throw new Error(`${name}: ${body.msg || response.status}`);
  return body.data;
};
const proxyRequest = (urlPath = '/', firstChunk) => new Promise((resolve, reject) => {
  const request = http.get({ hostname: '127.0.0.1', port: proxyPort, path: urlPath, headers: { Host: `graceful.test:${proxyPort}`, Connection: 'close' }, agent: false }, (res) => {
    let body = '';
    res.on('data', (chunk) => { body += chunk; firstChunk?.(); });
    res.on('error', reject);
    res.on('end', () => resolve({ status: res.statusCode, body, worker: res.headers['x-samwaf-worker'] }));
  });
  request.setTimeout(20000, () => request.destroy(new Error('proxy request timed out')));
  request.on('error', reject);
});
try {
  await waitFor(async () => (await fetch(`http://127.0.0.1:${managerPort}/`, { signal: AbortSignal.timeout(2000) })).status === 200);
  await new Promise((resolve, reject) => {
    const cli = spawn(exe, ['apikey', 'init', '-o', 'json'], { cwd: dir, env, windowsHide: true, stdio: ['ignore', log, log] });
    cli.on('error', reject);
    cli.on('exit', (code) => code === 0 ? resolve() : reject(new Error('Test API key initialization failed')));
  });
  await waitFor(async () => {
    const keyPath = path.join(dir, 'data', '.keys', 'cli_api_key');
    if (!fs.existsSync(keyPath)) return false;
    key = fs.readFileSync(keyPath, 'utf8').trim();
    return (await api('version')).version;
  });
  report.versions.push((await api('version')).version);
  const siteFile = path.join(dir, 'site.json');
  // This disposable site isolates transport continuity from WAF challenge rules.
  fs.writeFileSync(siteFile, JSON.stringify({ host: 'graceful.test', port: proxyPort, remote_host: 'http://127.0.0.1', remote_port: originPort, is_enable_response_buffering: 0, log_only_mode: 1 }));
  const created = await new Promise((resolve, reject) => {
    const cli = spawn(exe, ['site', 'apply', '-f', siteFile, '--wait', '60s', '-o', 'json'], { cwd: dir, env, windowsHide: true, stdio: ['ignore', 'pipe', log] });
    let out = ''; cli.stdout.on('data', (s) => { out += s; });
    cli.on('error', reject);
    cli.on('exit', (code) => code === 0 ? resolve(JSON.parse(out)) : reject(new Error('Site fixture creation failed: ' + out.slice(-500))));
  });
  const siteCode = created.results?.[0]?.code;
  if (!siteCode) throw new Error('Missing test site code');
  const guard = await fetch(`http://127.0.0.1:${managerPort}/api/v1/wafhost/host/guardstatus?code=${encodeURIComponent(siteCode)}&guard_status=0`, { headers: { 'X-API-Key': key }, signal: AbortSignal.timeout(5000) });
  if ((await guard.json()).code !== 0) throw new Error('Could not disable challenges on disposable test site');
  await waitFor(async () => { const r = await proxyRequest(); if (r.status !== 200 || r.body !== 'origin-ok') throw new Error(`Proxy fixture HTTP ${r.status}: ${r.body.slice(0, 300)}`); return true; }, 10000);
  const before = readState(); report.oldWorker = before.active_pid;
  const available = await api('checkversion');
  if (!available.need_update || available.version_new !== nextVersion) throw new Error('Update was not detected');
  monitor = Array.from({ length: 4 }, async () => {
    while (!stopped) {
      try { const r = await proxyRequest(); if (r.status !== 200 || r.body !== 'origin-ok') { report.failures.push(`HTTP ${r.status}: ${r.body.slice(0, 80)}`); report.failureSamples[r.worker || 'unknown'] ||= { request: report.requests, status: r.status, body: r.body.slice(0, 8000) }; } }
      catch (err) { report.failures.push(err.message); }
      report.requests++; await delay(10);
    }
  });
  let began;
  const slowStarted = new Promise((r) => { began = r; });
  const slow = proxyRequest('/slow', began);
  await slowStarted;
  upgraded = net.createConnection({ host: '127.0.0.1', port: proxyPort });
  upgraded.setTimeout(20000, () => upgraded.destroy(new Error('upgrade socket timed out')));
  await new Promise((resolve, reject) => {
    upgraded.once('error', reject);
    upgraded.once('connect', () => upgraded.write(`GET /upgrade HTTP/1.1\r\nHost: graceful.test:${proxyPort}\r\nConnection: upgrade\r\nUpgrade: test-echo\r\n\r\n`));
    upgraded.once('data', (data) => String(data).includes('101') ? resolve() : reject(new Error('Upgrade rejected')));
  });
  await api('update');
  const after = await waitFor(() => { const s = readState(); return s.active_pid && s.active_pid !== before.active_pid ? s : false; });
  report.newWorker = after.active_pid;
  if (!after.pids.includes(before.active_pid)) throw new Error('Old worker exited before the long connection was released');
  const echo = new Promise((resolve, reject) => { upgraded.once('data', (data) => resolve(String(data))); upgraded.once('error', reject); });
  upgraded.write('alive-after-takeover\n');
  if (await echo !== 'alive-after-takeover\n') throw new Error('Long connection did not survive takeover');
  upgraded.end();
  const result = await slow;
  if (result.status !== 200 || result.body !== 'begin\nend\n') throw new Error('Slow request was interrupted');
  await waitFor(() => !readState().pids.includes(before.active_pid));
  report.versions.push((await api('version')).version);
  if (report.versions[1] !== nextVersion) throw new Error('New binary did not take over');
  stopped = true; await Promise.all(monitor);
  if (report.failures.length) throw new Error(`${report.failures.length} proxy requests failed`);
  report.checks.push('Real update API downloaded, SHA256-verified and replaced the executable', 'New worker accepted traffic before old worker exited', 'Slow response and upgraded connection survived takeover', 'All continuous proxy requests succeeded');
  fs.writeFileSync(path.join(root, 'artifacts', 'graceful-update-verification.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  fs.writeFileSync(path.join(root, 'artifacts', 'graceful-update-verification.json'), JSON.stringify({ ...report, error: error.message }, null, 2));
  console.error(JSON.stringify({ requests: report.requests, versions: report.versions, samples: Object.values(report.failureSamples).map((s) => ({ request: s.request, status: s.status, text: s.body.replace(/<[^>]*>/g, ' ').slice(-1200) })), directory: dir }));
  throw error;
} finally {
  stopped = true;
  upgraded?.destroy();
  await Promise.allSettled(monitor);
  // These PIDs come only from the fresh test instance's state file.
  try { for (const pid of readState().pids) { try { process.kill(pid); } catch {} } } catch {}
  child.kill();
  publisher.closeAllConnections(); publisher.close();
  origin.closeAllConnections(); origin.close();
  fs.closeSync(log);
}
