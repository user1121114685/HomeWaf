// Release-gated end-to-end check: pass two HomeWaf binaries with increasing versions.
// Everything runs in a fresh .cache directory with loopback ports and synthetic data.
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import https from 'node:https';
import net from 'node:net';
import { createHash } from 'node:crypto';
import { execFileSync, spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { listenLoopbackForTest as listen } from './test-listener.mjs';

const root = fileURLToPath(new URL('..', import.meta.url));
const [oldInput, newInput, nextVersion = 'v3.0.1'] = process.argv.slice(2);
const viaInstaller = process.argv.includes('--via-installer');
if (!oldInput || !newInput) throw new Error('Usage: node scripts/verify-graceful-update.mjs old-binary new-binary next-version');
const dir = fs.mkdtempSync(path.join(root, '.cache', 'graceful-smoke-'));
const nativeName = process.platform === 'win32' ? 'HomeWaf64.exe' : `HomeWaf${process.platform === 'darwin' ? 'Darwin' : 'Linux'}${process.arch === 'arm64' ? 'Arm64' : '64'}`;
const exe = path.join(dir, nativeName);
fs.copyFileSync(path.resolve(oldInput), exe);
fs.mkdirSync(path.join(dir, 'conf'));
const report = { requests: 0, failures: [], failureSamples: {}, versions: [], checks: [], directory: dir };
const freePort = async () => { const s = net.createServer(); const p = await listen(s); await new Promise((r) => s.close(r)); return p; };
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const waitFor = async (fn, timeout = 60000, interval = 100) => { const end = Date.now() + timeout; let last; while (Date.now() < end) { try { const value = await fn(); if (value) return value; } catch (err) { last = err; } await delay(interval); } throw new Error('Timed out waiting for smoke check: ' + (last?.message || 'condition not satisfied') + (last?.cause?.message ? ` (${last.cause.message})` : '')); };
const keyPath = path.join(dir, 'publisher.key');
const caPath = path.join(dir, 'publisher.pem');
execFileSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
  '-subj', '/CN=HomeWaf update fixture', '-addext', 'subjectAltName=IP:127.0.0.1',
  '-keyout', keyPath, '-out', caPath], { windowsHide: true, stdio: 'ignore' });
let release;
const publisher = https.createServer({ key: fs.readFileSync(keyPath), cert: fs.readFileSync(caPath) }, (req, res) => {
  if (req.url === '/release') {
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({ tag_name: nextVersion, body: 'Isolated smooth update verification',
      assets: [{ name: release.name, browser_download_url: `https://127.0.0.1:${publisherPort}/${release.name}`, digest: `sha256:${release.digest}` }] }));
  } else if (req.url === '/' + release.name) {
    res.setHeader('Content-Length', fs.statSync(release.archivePath).size);
    fs.createReadStream(release.archivePath).pipe(res);
  } else { res.writeHead(404); res.end(); }
});
const publisherPort = await listen(publisher);
const origin = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': req.url === '/slow' ? 'text/event-stream' : 'text/plain' });
  if (req.url === '/slow') { res.write('begin\n'); setTimeout(() => res.end('end\n'), 6500); }
  else res.end('origin-ok');
});
origin.on('upgrade', (req, socket) => {
  // Exercise the supported WebSocket protocol. Arbitrary HTTP upgrades are
  // intentionally rejected by HomeWaf's protocol guard.
  const accept = createHash('sha1').update(req.headers['sec-websocket-key'] + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').digest('base64');
  socket.write(`HTTP/1.1 101 Switching Protocols\r\nConnection: upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: ${accept}\r\n\r\n`);
  let pending = Buffer.alloc(0);
  socket.on('data', data => {
    pending = Buffer.concat([pending, data]);
    if (pending.length < 2) return;
    const length = pending[1] & 0x7f;
    if (pending[0] !== 0x81 || !(pending[1] & 0x80) || length >= 126) { socket.destroy(); return; }
    if (pending.length < length + 6) return;
    const message = Buffer.from(pending.subarray(6, 6 + length));
    for (let i = 0; i < message.length; i++) message[i] ^= pending[2 + i % 4];
    socket.write(Buffer.concat([Buffer.from([0x81, message.length]), message]));
    pending = pending.subarray(6 + length);
  });
});
const originPort = await listen(origin), managerPort = await freePort(), proxyPort = await freePort();
fs.writeFileSync(path.join(dir, 'conf', 'config.yml'), `local_port: ${managerPort}\ndrain_time_out: 15\ndebug_worker_header: true\nupdate_restart_timeout: 90\nnotice:\n  isenable: false\n`);
const env = { ...process.env, HOMEWAF_UPDATE_URL: '', HOMEWAF_RELEASE_URL: `https://127.0.0.1:${publisherPort}/release`,
  HOMEWAF_SMOKE_CA: caPath };
const log = fs.openSync(path.join(dir, 'process.log'), 'a');
const helperProcesses = new Set();
const runProcess = (command, args, stdio = ['ignore', log, log]) => new Promise((resolve, reject) => {
  const processChild = spawn(command, args, { cwd: dir, env, windowsHide: true, stdio });
  helperProcesses.add(processChild);
  processChild.once('error', reject);
  processChild.once('exit', code => { helperProcesses.delete(processChild); code === 0 ? resolve() : reject(new Error(`${command} exited ${code}`)); });
});
const installerCommand = process.platform === 'win32' ? 'powershell.exe' : 'bash';
const installerArgs = (action, extras = []) => process.platform === 'win32'
  ? ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', path.join(root, 'install_homewaf.ps1'), '-Action', action, '-Directory', dir, '-NoService', ...extras]
  : [path.join(root, 'install_homewaf.sh'), action, '--dir', dir, '--no-service', ...extras];
async function prepareInstaller() {
  const stage = path.join(dir, 'installer-stage');
  fs.mkdirSync(stage);
  fs.copyFileSync(path.resolve(newInput), path.join(stage, nativeName));
  for (const name of ['LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'LICENSES']) fs.cpSync(path.join(root, name), path.join(stage, name), { recursive: true });
  const platform = `${process.platform === 'win32' ? 'Windows' : process.platform === 'darwin' ? 'Darwin' : 'Linux'}_${process.arch === 'arm64' ? 'arm64' : 'x86_64'}`;
  const asset = `HomeWaf_${platform}_${nextVersion.slice(1)}${process.platform === 'win32' ? '.zip' : '.tar.gz'}`;
  const archivePath = path.join(dir, asset);
  if (process.platform === 'win32') {
    const packer = path.join(dir, 'pack-fixture.ps1');
    fs.writeFileSync(packer, "param([string]$Source,[string]$Destination)\n$ErrorActionPreference='Stop'\n$files=@(Get-ChildItem -LiteralPath $Source | ForEach-Object { $_.FullName })\nCompress-Archive -LiteralPath $files -DestinationPath $Destination\n");
    await runProcess('powershell.exe', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', packer, stage, archivePath]);
  } else await runProcess('tar', [
    '-czf', archivePath,
    '-C', stage,
    nativeName,
    'LICENSE',
    'NOTICE',
    'THIRD_PARTY_NOTICES.md',
    'LICENSES',
  ]);
  const digest = createHash('sha256');
  for await (const data of fs.createReadStream(archivePath)) digest.update(data);
  const checksumsPath = path.join(dir, 'fixture-checksums.txt');
  release = { name: asset, archivePath, digest: digest.digest('hex') };
  fs.writeFileSync(checksumsPath, `${release.digest}  ${asset}\n`);
  fs.writeFileSync(path.join(dir, '.homewaf-installer'), 'HomeWaf');
  return process.platform === 'win32'
    ? ['-Version', nextVersion, '-ArchivePath', archivePath, '-ChecksumsPath', checksumsPath]
    : ['--version', nextVersion, '--archive', archivePath, '--checksums', checksumsPath];
}
const upgradeArgs = await prepareInstaller();
const child = spawn(exe, [], { cwd: dir, env, windowsHide: true, stdio: ['ignore', log, log] });
let key, upgraded, stopped = false;
let monitor = [];
const proxyAgent = new http.Agent({ keepAlive: true, maxSockets: 8, maxFreeSockets: 8 });
const readState = () => JSON.parse(fs.readFileSync(path.join(dir, 'data', 'supervisor.state'), 'utf8'));
const api = async (name) => {
  const response = await fetch(`http://127.0.0.1:${managerPort}/api/v1/sysinfo/${name}`, { headers: { 'X-API-Key': key }, signal: AbortSignal.timeout(20000) });
  const body = await response.json();
  if (response.status !== 200 || body.code !== 0) throw new Error(`${name}: ${body.msg || response.status}`);
  return body.data;
};
const proxyRequest = (urlPath = '/', firstChunk) => new Promise((resolve, reject) => {
  const request = http.get({ hostname: '127.0.0.1', port: proxyPort, path: urlPath, headers: { Host: `graceful.test:${proxyPort}` }, agent: proxyAgent }, (res) => {
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
  const postPort = async (action, payload) => {
    const response = await fetch(`http://127.0.0.1:${managerPort}/api/v1/pipeline/ports/${action}`, {
      method: 'POST', headers: { 'Content-Type': 'application/json', 'X-API-Key': key },
      body: JSON.stringify(payload), signal: AbortSignal.timeout(10000),
    });
    const result = await response.json();
    if (!response.ok || result.code !== 0) throw new Error(`Service ${action} failed: ${result.msg || response.status}`);
    return result.data;
  };
  const { config } = await postPort('draft', {
    port: proxyPort, name: 'Graceful update fixture', host_names: ['graceful.test'],
    upstream: `http://127.0.0.1:${originPort}`, template_id: 'observe', defense_mode: 'restrict',
  });
  const saved = await postPort('save', { config });
  await postPort('save', { config: { ...saved, listen_enabled: true } });
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
  // A later assertion can fail before this request is awaited; keep cleanup from
  // turning its aborted socket into an unhandled rejection that hides the cause.
  slow.catch(() => {});
  await slowStarted;
  upgraded = net.createConnection({ host: '127.0.0.1', port: proxyPort });
  upgraded.setTimeout(20000, () => upgraded.destroy(new Error('upgrade socket timed out')));
  await new Promise((resolve, reject) => {
    upgraded.once('error', reject);
    upgraded.once('connect', () => upgraded.write(`GET /upgrade HTTP/1.1\r\nHost: graceful.test:${proxyPort}\r\nConnection: upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n`));
    upgraded.once('data', (data) => String(data).includes('101') ? resolve() : reject(new Error('Upgrade rejected: ' + String(data).slice(0, 500))));
  });
  const updateCompletion = viaInstaller ? runProcess(installerCommand, installerArgs('upgrade', upgradeArgs)) : api('update');
  updateCompletion.catch(() => {});
  if (!viaInstaller) await updateCompletion;
  // A script waits for the upgrade to finish. Observe takeover and release long
  // connections concurrently, rather than keeping them open until drain timeout.
  const handoff = await Promise.race([
    waitFor(async () => {
      const state = readState();
      if (state.active_pid && state.active_pid !== before.active_pid) return { state };
      const progress = await api('updateprogress');
      if (progress.state === 'failed') return { error: progress.error || 'Worker handoff failed', progress };
      return false;
    }, 150000, 1000),
    updateCompletion.then(() => new Promise(() => {})),
  ]);
  if (handoff.error) throw new Error(`Worker handoff failed at ${handoff.progress.error_stage || 'unknown stage'}: ${handoff.error}`);
  const after = handoff.state;
  report.newWorker = after.active_pid;
  if (!after.pids.includes(before.active_pid)) throw new Error('Old worker exited before the long connection was released');
  const echo = new Promise((resolve, reject) => {
    let received = Buffer.alloc(0);
    const onData = data => {
      received = Buffer.concat([received, data]);
      if (received.length >= 2 && received.length >= 2 + (received[1] & 0x7f)) {
        upgraded.off('data', onData);
        resolve(received.subarray(2, 2 + (received[1] & 0x7f)).toString());
      }
    };
    upgraded.on('data', onData);
    upgraded.once('error', reject);
  });
  const message = Buffer.from('alive-after-takeover\n'), mask = Buffer.from([1, 2, 3, 4]);
  const masked = Buffer.from(message);
  for (let i = 0; i < masked.length; i++) masked[i] ^= mask[i % 4];
  upgraded.write(Buffer.concat([Buffer.from([0x81, 0x80 | masked.length]), mask, masked]));
  if (await echo !== 'alive-after-takeover\n') throw new Error('Long connection did not survive takeover');
  upgraded.end();
  const result = await slow;
  if (result.status !== 200 || result.body !== 'begin\nend\n') throw new Error('Slow request was interrupted');
  await waitFor(() => !readState().pids.includes(before.active_pid));
  await updateCompletion;
  report.versions.push((await api('version')).version);
  if (report.versions[1] !== nextVersion) throw new Error('New binary did not take over');
  stopped = true; await Promise.all(monitor);
  if (report.failures.length) throw new Error(`${report.failures.length} proxy requests failed`);
  report.checks.push('Real update API downloaded, SHA256-verified and replaced the executable', 'New worker accepted traffic before old worker exited', 'Slow response and WebSocket connection survived takeover', 'All continuous proxy requests succeeded');
  fs.writeFileSync(path.join(root, 'artifacts', 'graceful-update-verification.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  const diagnosticLog = path.join(dir, 'process.log');
  const logTail = fs.existsSync(diagnosticLog)
    ? fs.readFileSync(diagnosticLog, 'utf8').split(/\r?\n/).slice(-160).join('\n')
    : '';
  const failure = { ...report, error: error.message, logTail };
  fs.writeFileSync(path.join(root, 'artifacts', 'graceful-update-verification.json'), JSON.stringify(failure, null, 2));
  console.error(JSON.stringify({ requests: report.requests, versions: report.versions, samples: Object.values(report.failureSamples).map((s) => ({ request: s.request, status: s.status, text: s.body.replace(/<[^>]*>/g, ' ').slice(-1200) })), error: error.message, logTail, directory: dir }));
  throw error;
} finally {
  stopped = true;
  upgraded?.destroy();
  proxyAgent.destroy();
  await Promise.allSettled(monitor);
  for (const helperProcess of helperProcesses) { try { helperProcess.kill(); } catch {} }
  if (child.pid && child.exitCode === null && child.signalCode === null) {
    await new Promise(resolve => {
      const timeout = setTimeout(() => child.kill('SIGKILL'), 10000);
      child.once('exit', () => { clearTimeout(timeout); resolve(); });
      child.kill();
    });
  }
  // These PIDs come only from the fresh test instance's state file.
  try { for (const pid of readState().pids) { try { process.kill(pid); } catch {} } } catch {}
  publisher.closeAllConnections(); publisher.close();
  origin.closeAllConnections(); origin.close();
  fs.closeSync(log);
}
if (viaInstaller) {
  const retained = ['conf/config.yml', 'data/.keys/cli_api_key', '.homewaf-keys/data_key'];
  const snapshots = retained.map(name => fs.readFileSync(path.join(dir, name)));
  await runProcess(installerCommand, installerArgs('uninstall'), 'inherit');
  if (fs.existsSync(exe)) throw new Error('Uninstall left the executable in place');
  retained.forEach((name, index) => {
    if (!fs.readFileSync(path.join(dir, name)).equals(snapshots[index])) throw new Error(`Uninstall changed retained instance data: ${name}`);
  });
  fs.copyFileSync(path.resolve(newInput), exe);
  fs.chmodSync(exe, 0o755);
  fs.writeFileSync(path.join(dir, 'owner-note.txt'), 'preserve unrelated files');
  const purge = process.platform === 'win32' ? ['-PurgeData', '-Yes'] : ['--purge-data', '--yes'];
  const outside = fs.mkdtempSync(path.join(root, '.cache', 'purge-boundary-'));
  const sentinel = path.join(outside, 'keep.txt');
  fs.writeFileSync(sentinel, 'must remain outside the application');
  const link = path.join(dir, 'data', 'external-link');
  fs.symlinkSync(outside, link, process.platform === 'win32' ? 'junction' : 'dir');
  if (process.platform === 'win32') {
    let refused = false;
    try { await runProcess(installerCommand, installerArgs('uninstall', purge), 'ignore'); }
    catch { refused = true; }
    if (!refused || !fs.existsSync(sentinel) || !fs.existsSync(exe)) throw new Error('Linked-data purge was not rejected before removal');
    fs.unlinkSync(link);
  }
  await runProcess(installerCommand, installerArgs('uninstall', purge), 'inherit');
  if (!fs.existsSync(sentinel)) throw new Error('Purge followed a link outside the application');
  fs.unlinkSync(sentinel);
  fs.rmdirSync(outside);
  for (const name of ['conf', 'data', '.homewaf-keys', 'logs', 'ssl']) if (fs.existsSync(path.join(dir, name))) throw new Error(`Explicit purge left ${name}`);
  if (!fs.existsSync(path.join(dir, 'owner-note.txt'))) throw new Error('Purge removed unrelated files');
  report.checks.push('Installer upgrade works without WebUI', 'Default uninstall retains configuration, data and keys', 'Explicit purge removes only named instance data directories');
  fs.writeFileSync(path.join(root, 'artifacts/graceful-update-installer-verification.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ checks: report.checks, requests: report.requests, failures: report.failures }, null, 2));
}
