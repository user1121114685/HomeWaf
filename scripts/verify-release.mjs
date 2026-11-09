// Install a real release archive into a disposable directory and verify startup
// plus the same checkversion API used by the update prompt in the web console.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import http from 'node:http';
import net from 'node:net';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const [archiveInput, version] = process.argv.slice(2);
if (!archiveInput || !/^v\d+\.\d+\.\d+$/.test(version || '')) throw new Error('Usage: node scripts/verify-release.mjs archive-directory vX.Y.Z [--online]');
const online = process.argv.includes('--online');
const platforms = {
  'win32-x64': ['Windows_x86_64', 'HomeWaf64.exe', '.zip'],
  'linux-x64': ['Linux_x86_64', 'HomeWafLinux64', '.tar.gz'],
  'linux-arm64': ['Linux_arm64', 'HomeWafLinuxArm64', '.tar.gz'],
  'darwin-x64': ['Darwin_x86_64', 'HomeWafDarwin64', '.tar.gz'],
  'darwin-arm64': ['Darwin_arm64', 'HomeWafDarwinArm64', '.tar.gz'],
};
const platform = platforms[`${process.platform}-${process.arch}`];
if (!platform) throw new Error('Unsupported smoke test platform');
const [assetPlatform, binaryName, extension] = platform;
const dir = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), 'homewaf-release-'));
const archiveDir = path.resolve(archiveInput);
const archive = path.join(archiveDir, `HomeWaf_${assetPlatform}_${version.slice(1)}${extension}`);
const checksums = path.join(archiveDir, `HomeWaf_${version.slice(1)}_checksums.txt`);
const run = (command, args, options = {}) => {
  const r = spawnSync(command, args, { stdio: 'inherit', windowsHide: true, ...options });
  if (r.error) throw r.error;
  if (r.status !== 0) throw new Error(`${command} failed (${r.status})`);
};
if (process.platform === 'win32') {
  run('powershell.exe', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', path.join(root, 'install_homewaf.ps1'), '-Version', version, '-Directory', dir, '-NoService', ...(online ? [] : ['-ArchivePath', archive, '-ChecksumsPath', checksums])]);
} else {
  run('bash', [path.join(root, 'install_homewaf.sh'), '--version', version, '--dir', dir, '--no-service', ...(online ? [] : ['--archive', archive, '--checksums', checksums])]);
}
const listen = server => new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', () => resolve(server.address().port)); });
const reserve = net.createServer();
const port = await listen(reserve);
await new Promise(r => reserve.close(r));
const nextVersion = version.replace(/\d+$/, n => String(Number(n) + 1));
let publishedVersion = nextVersion;
const publisher = http.createServer((_req, res) => {
  const name = `HomeWaf_${assetPlatform}_${publishedVersion.slice(1)}${extension}`;
  res.setHeader('Content-Type', 'application/json');
  res.end(JSON.stringify({ tag_name: publishedVersion, body: 'Release notification smoke test', assets: [{ name, browser_download_url: `https://github.com/user1121114685/HomeWaf/releases/download/${publishedVersion}/${name}`, digest: `sha256:${'0'.repeat(64)}` }] }));
});
const publisherPort = await listen(publisher);
fs.mkdirSync(path.join(dir, 'conf'), { recursive: true });
fs.writeFileSync(path.join(dir, 'conf/config.yml'), `local_port: ${port}\nnotice:\n  isenable: false\n`);
const env = { ...process.env, HOMEWAF_DEV: '1', HOMEWAF_UPDATE_URL: '', HOMEWAF_RELEASE_URL: `http://127.0.0.1:${publisherPort}/release` };
const exe = path.join(dir, binaryName);
const log = fs.openSync(path.join(dir, 'smoke.log'), 'a');
const child = spawn(exe, [], { cwd: dir, env, windowsHide: true, stdio: ['ignore', log, log] });
let spawnError;
child.on('error', err => { spawnError = err; });
const wait = async fn => {
  const end = Date.now() + 90000;
  let last;
  while (Date.now() < end) {
    if (spawnError) throw spawnError;
    try { const result = await fn(); if (result) return result; } catch (error) { last = error; }
    await new Promise(r => setTimeout(r, 200));
  }
  throw new Error(`Startup timed out (${last?.message || 'no response'}); log: ${dir}`);
};
try {
  await wait(async () => (await fetch(`http://127.0.0.1:${port}/`, { signal: AbortSignal.timeout(2000) })).status === 200);
  run(exe, ['apikey', 'init', '-o', 'json'], { cwd: dir, env, stdio: ['ignore', log, log] });
  const keyFile = path.join(dir, 'data/.keys/cli_api_key');
  await wait(() => fs.existsSync(keyFile));
  const key = fs.readFileSync(keyFile, 'utf8').trim();
  const api = async name => {
    const res = await fetch(`http://127.0.0.1:${port}/api/v1/sysinfo/${name}`, { headers: { 'X-API-Key': key }, signal: AbortSignal.timeout(15000) });
    const body = await res.json();
    if (res.status !== 200 || body.code !== 0) throw new Error(`${name}: ${body.msg}`);
    return body.data;
  };
  const running = await wait(() => api('version'));
  if (running.version !== version) throw new Error(`Wrong binary version: ${running.version}`);
  if (!fs.existsSync(path.join(dir, 'data/initial_password.txt'))) throw new Error('Initial password file was not created');
  if (!fs.existsSync(path.join(dir, '.homewaf-keys/data_key'))) throw new Error('Instance encryption key was not created');
  const update = await api('checkversion');
  if (!update.need_update || update.version_new !== nextVersion || update.version_desc !== 'Release notification smoke test') throw new Error(`Update prompt API failed: ${JSON.stringify(update)}`);
  publishedVersion = version;
  if ((await api('checkversion')).need_update) throw new Error('Current version incorrectly offered as an update');
  console.log(JSON.stringify({ platform: assetPlatform, version, online, checks: ['SHA256-verified installer', 'fresh database startup', 'embedded web UI', 'version API', 'random initial password', 'instance key', 'new-version notification', 'no redundant update'] }));
} finally {
  try {
    const state = JSON.parse(fs.readFileSync(path.join(dir, 'data/supervisor.state'), 'utf8'));
    for (const pid of state.pids || []) { if (Number.isInteger(pid) && pid > 1 && pid !== process.pid) { try { process.kill(pid); } catch {} } }
  } catch {}
  child.kill();
  publisher.closeAllConnections(); publisher.close();
  fs.closeSync(log);
}
