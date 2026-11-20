// Development only: watch -> staged build -> existing rolling-restart.
import fs from "node:fs";
import { resolveBuildVersion, buildMetadataFlags } from './build-version.mjs';
import path from "node:path";
import net from "node:net";
import { spawn, spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";
import { BuildInputs, excludedRoots as excluded, ignoredInput } from "./dev-inputs.mjs";

const root = fileURLToPath(new URL("..", import.meta.url));
const webDir = path.join(root, "web");
const viteCli = path.join(webDir, "node_modules", "vite", "bin", "vite.js");
const args = process.argv.slice(2);
const windows = process.platform === "win32";
const devDir = path.join(root, ".cache", "dev");
const runtimeDir = path.join(devDir, "runtime");
const binary = path.join(runtimeDir, `homewaf${windows ? ".exe" : ""}`);
const candidate = path.join(devDir, `candidate${windows ? ".exe" : ""}`);
const statePath = path.join(runtimeDir, "data", "supervisor.state");
const lockPath = path.join(devDir, "session.lock");
const adoptExisting = args.includes("--adopt-existing");
const watchLog = path.join(devDir, "watch-events.log");
const watchers = new Map();
const children = new Set();
let supervisor, frontend, stopping = false, busy = false, dirty = false, timer, lock;
let generation = 0;
let inputs, attached = false;
const pendingEvents = new Set();

function trace(event, details = {}) {
  fs.appendFileSync(watchLog, JSON.stringify({ time: new Date().toISOString(), watcher_pid: process.pid, event, ...details }) + "\n");
}

function log(message) { console.log(`[dev] ${message}`); }
function alive(child) { return child && child.exitCode === null && child.signalCode === null; }
function readState() {
  try { return JSON.parse(fs.readFileSync(statePath, "utf8")); } catch { return null; }
}
function start(command, commandArgs, options = {}) {
  if (stopping) throw new Error("Development session stopped");
  const child = spawn(command, commandArgs, {
    cwd: root, stdio: "inherit", windowsHide: true, detached: !windows, ...options,
  });
  children.add(child);
  child.done = new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => resolve({ code, signal }));
  }).finally(() => children.delete(child));
  child.done.catch(() => {});
  return child;
}
async function run(command, commandArgs, options) {
  const result = await start(command, commandArgs, options).done;
  if (result.code !== 0) throw new Error(`${path.basename(command)} failed (${result.code ?? result.signal})`);
}
async function waitFor(check, timeout, description) {
  const end = Date.now() + timeout;
  while (!stopping && Date.now() < end) {
    const result = check();
    if (result) return result;
    if (supervisor && !alive(supervisor)) throw new Error("Development Supervisor exited");
    await delay(100);
  }
  throw new Error(stopping ? "Development session stopped" : `Timed out: ${description}; inspect ${runtimeDir}/logs/supervisor.log`);
}
function npm(args, options) {
  if (!windows) return start("npm", args, options);
  const cli = path.join(path.dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js");
  if (!fs.existsSync(cli)) throw new Error("npm-cli.js must be installed alongside Node.js");
  return start(process.execPath, [cli, ...args], options);
}
function backendPort() {
  const config = path.join(runtimeDir, "conf", "config.yml");
  const text = fs.existsSync(config) ? fs.readFileSync(config, "utf8") : "";
  return Number(text.match(/^local_port:\s*["']?(\d+)/m)?.[1] || 26666);
}
async function checkPort(port) {
  const server = net.createServer();
  await new Promise((resolve, reject) => {
    server.once("error", () => reject(new Error(`Port ${port} is in use. Stop the other instance or change local_port in ${runtimeDir}/conf/config.yml.`)));
    server.listen(port, "127.0.0.1", () => server.close(resolve));
  });
}

// Windows permits moving a running image, but not overwriting it. Keep unique backups.
function promote() {
  const backup = path.join(devDir, `previous-${Date.now()}-${generation}${windows ? ".exe" : ""}`);
  const previous = fs.existsSync(binary);
  if (previous) fs.renameSync(binary, backup);
  try { fs.renameSync(candidate, binary); }
  catch (error) {
    if (previous) fs.renameSync(backup, binary);
    throw error;
  }
  return previous ? backup : null;
}
function cleanBinaries() {
  for (const name of fs.readdirSync(devDir)) {
    if (!/^previous-\d+-\d+(\.exe)?$/.test(name)) continue;
    // The original Supervisor image stays locked until session exit on Windows.
    try { fs.unlinkSync(path.join(devDir, name)); } catch {}
  }
}
async function rebuild() {
  await run(process.execPath, ["scripts/prepare-ip-databases.mjs", "--check"]);
  const build = resolveBuildVersion({ root });
  const version = build.version;
  ++generation;
  log(`Building ${version}...`);
  await run("go", ["build", `-ldflags=-X HomeWaf/internal/instance/global.GWAF_RELEASE=false -X HomeWaf/internal/instance/global.GWAF_RELEASE_VERSION=${version} ${buildMetadataFlags(build)}`, "-o", candidate, "./cmd/homewaf"]);
  if (stopping) return;
  const before = readState();
  if ((supervisor || attached) && before?.upgrading) throw new Error("Supervisor is already switching Workers; save again after it finishes");
  const backup = promote();
  try {
    if (!supervisor && !attached) {
      await checkPort(backendPort());
      supervisor = start(binary, [], { cwd: runtimeDir });
      supervisor.done.then(() => { if (!stopping) void shutdown(1); }, () => { if (!stopping) void shutdown(1); });
      await waitFor(() => {
        const state = readState();
        if (state?.active_pid && !state.pids?.includes(state.active_pid)) {
          throw new Error("Initial Worker exited before READY; check the startup error above");
        }
        const activeVersion = state?.versions?.[state.pids?.indexOf(state.active_pid)];
        return state && activeVersion === version && ["active", "active-candidate"].includes(state.active_state) && state;
      }, 150000, "initial Worker READY");
    } else {
      if (!before) throw new Error("Cannot read Supervisor state; save again to retry");
      log("Build succeeded; requesting rolling-restart...");
      await run(binary, ["rolling-restart"], { cwd: runtimeDir });
      const after = await waitFor(() => {
        const state = readState();
        return state && state.upgrade_sequence > before.upgrade_sequence && !state.upgrading && state;
      }, 360000, "rolling restart completion");
      const activeVersion = after.versions?.[after.pids?.indexOf(after.active_pid)];
      if (activeVersion !== version) throw new Error("New Worker did not become ready; old Worker remains active");
    }
  } catch (error) {
    // Preserve the old launch path on readiness failure, not just its live process.
    // A timeout during an ongoing handoff must not replace its binary underneath it.
    if (backup && !readState()?.upgrading && !stopping) {
      fs.renameSync(binary, candidate);
      fs.renameSync(backup, binary);
    }
    throw error;
  }
  cleanBinaries();
  trace("ready", { worker_pid: readState()?.active_pid, version });
  log(`Ready: Worker ${readState()?.active_pid}, ${version}`);
  if (!attached && !args.includes("--backend-only") && !frontend && !stopping) {
    // Run Vite directly so Windows has no extra npm/cmd process intercepting Ctrl+C.
    frontend = start(process.execPath, [viteCli, "--host", "127.0.0.1", "--strictPort", "--clearScreen", "false"], {
      cwd: webDir,
      // Vite's terminal shortcuts use raw stdin, which would swallow Ctrl+C
      // before Windows can broadcast it to the Node/Go development processes.
      stdio: ["ignore", "inherit", "inherit"],
      env: { ...process.env, HOMEWAF_BACKEND: `http://127.0.0.1:${backendPort()}` },
    });
    frontend.done.then(() => { if (!stopping) void shutdown(1); }, () => { if (!stopping) void shutdown(1); });
    log("Frontend: http://127.0.0.1:3001");
  }
}
async function drainBuilds() {
  if (busy || stopping) return;
  busy = true;
  try {
    while (dirty && !stopping) {
      dirty = false;
      try { await rebuild(); }
      catch (error) { if (!stopping) console.error(`[dev] ${error.message}\n[dev] Fix the problem and save a watched file to retry.`); }
    }
  } finally { busy = false; }
}
function changed(relative) {
  if (stopping) return;
  const name = relative.replaceAll("\\", "/");
  if (ignoredInput(name)) return;
  pendingEvents.add(name);
  // Do not postpone forever if unrelated filesystem notifications keep arriving.
  timer ||= setTimeout(() => {
    timer = undefined;
    const events = [...pendingEvents];
    pendingEvents.clear();
    try {
      const changes = inputs.scan();
      trace(changes.length ? "inputs-changed" : "unchanged", { notifications: events, inputs: changes });
      if (!changes.length) return;
      log(`Content changed: ${changes.join(", ")}`);
      dirty = true;
      void drainBuilds();
    } catch (error) { console.error(`[dev] Input check failed; no reload: ${error.message}`); }
  }, 600);
}
function refreshWatchers() {
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    if (!entry.isDirectory() || entry.name.startsWith(".") || excluded.has(entry.name) || watchers.has(entry.name)) continue;
    const watcher = fs.watch(path.join(root, entry.name), { recursive: true }, (_event, file) => {
      changed(file ? path.join(entry.name, String(file)) : entry.name);
    });
    watcher.on("error", (error) => { console.error(`[dev] Watch error: ${error.message}`); void shutdown(1); });
    watchers.set(entry.name, watcher);
  }
  for (const [name, watcher] of watchers) {
    if (name && !fs.existsSync(path.join(root, name))) { watcher.close(); watchers.delete(name); }
  }
}
function signalGroup(child, signal) {
  try { process.kill(-child.pid, signal); } catch (error) { if (error.code !== "ESRCH") throw error; }
}
async function stopTree(child, grace = false) {
  if (!alive(child) || !child.pid) return;
  if (grace) {
    if (!windows) child.kill("SIGINT");
    // Windows console Ctrl+C is broadcast to the Go Supervisor too.
    await Promise.race([child.done.catch(() => {}), delay(35000)]);
  }
  if (!windows) {
    signalGroup(child, "SIGTERM");
    await Promise.race([child.done.catch(() => {}), delay(3000)]);
    signalGroup(child, "SIGKILL");
  } else if (alive(child)) {
    await new Promise(resolve => {
      const killer = spawn("taskkill", ["/PID", String(child.pid), "/T", "/F"], { windowsHide: true, stdio: "ignore" });
      killer.on("error", resolve); killer.on("exit", resolve);
    });
  }
}
async function shutdown(code = 0, consoleInterrupt = false) {
  if (stopping) return;
  stopping = true;
  clearTimeout(timer);
  for (const watcher of watchers.values()) watcher.close();
  log("Stopping development processes...");
  // On Windows only console Ctrl+C has already signalled the Supervisor.
  // An internal startup failure must not wait for a signal that was never sent.
  await Promise.allSettled([...children].map(child => stopTree(child, child === supervisor && (!windows || consoleInterrupt))));
  if (lock !== undefined) {
    fs.closeSync(lock);
    if (fs.existsSync(lockPath) && fs.readFileSync(lockPath, "utf8") === String(process.pid)) fs.unlinkSync(lockPath);
    lock = undefined;
  }
  cleanBinaries();
  process.exit(code);
}
async function main() {
  if (args.includes("--help") || args.includes("-h")) {
    console.log("HomeWaf development: node scripts/dev.mjs [--backend-only] [--adopt-existing]\nChanged build input contents trigger rolling reloads; Ctrl+C stops owned processes.\n--adopt-existing attaches a new watcher after the old watcher exits, without owning the running services.\nRuntime data: .cache/dev/runtime (preserved). Trigger log: .cache/dev/watch-events.log.\nFrontend: http://127.0.0.1:3001; backend defaults to :26666.");
    return;
  }
  if (args.some(arg => !["--backend-only", "--adopt-existing"].includes(arg))) throw new Error("Unknown option; use --help");
  fs.mkdirSync(runtimeDir, { recursive: true });
  if (fs.existsSync(lockPath)) {
    const pid = Number(fs.readFileSync(lockPath, "utf8"));
    if (!Number.isInteger(pid) || pid <= 0) throw new Error(`Invalid lock: ${lockPath}`);
    try { process.kill(pid, 0); throw new Error("A development session is already running"); }
    catch (error) { if (error.code !== "ESRCH") throw error; }
    fs.unlinkSync(lockPath);
  }
  lock = fs.openSync(lockPath, "wx");
  fs.writeFileSync(lock, String(process.pid));
  process.on("SIGINT", () => void shutdown(0, true));
  process.on("SIGTERM", () => void shutdown());
  const host = spawnSync("go", ["env", "GOHOSTOS", "GOHOSTARCH"], { cwd: root, encoding: "utf8", windowsHide: true });
  if (host.error || host.status !== 0) throw host.error || new Error(host.stderr);
  const [goos, goarch] = host.stdout.trim().split(/\s+/);
  if (!goos || !goarch) throw new Error("Cannot determine native Go target");
  Object.assign(process.env, { GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0", HOMEWAF_DEV: "1", SAMWAF_WORKER_HEADER: "1" });
  if (adoptExisting) {
    const state = readState();
    if (!state?.active_pid || !state.pids?.includes(state.active_pid)) throw new Error("No active development Worker to attach to");
    process.kill(state.active_pid, 0);
    attached = true;
    log(`Attached to running Worker ${state.active_pid}; existing services are retained on watcher exit.`);
  } else await checkPort(backendPort());
  if (!attached && !args.includes("--backend-only")) {
    await checkPort(3001);
    // Interrupted installs can leave the package directory without its CLI.
    if (!fs.existsSync(viteCli)) {
      log("Installing frontend dependencies...");
      const result = await npm(["ci", "--include=dev", "--prefix", "web"]).done;
      if (result.code !== 0) throw new Error("Frontend dependency installation failed");
    }
    if (!fs.existsSync(viteCli)) throw new Error(`Vite CLI is missing after installation: ${viteCli}`);
    // Detect an incomplete dependency tree before starting the Go Supervisor.
    try { await run(process.execPath, [viteCli, "--version"], { cwd: webDir, stdio: ["ignore", "inherit", "inherit"] }); }
    catch (error) { throw new Error(`Frontend dependency check failed: ${error.message}. Run npm ci --include=dev --prefix web and retry.`); }
  }
  log(`Runtime data: ${runtimeDir}`);
  inputs = new BuildInputs(root);
  const watcher = fs.watch(root, (_event, file) => {
    refreshWatchers(); changed(file ? String(file) : "");
  });
  watcher.on("error", (error) => { console.error(error.message); void shutdown(1); });
  watchers.set("", watcher);
  refreshWatchers();
  trace("watching", { attached, worker_pid: readState()?.active_pid });
  dirty = true;
  await drainBuilds();
}

main().catch(async error => {
  console.error(`[dev] ${error.message}`);
  if (lock !== undefined) await shutdown(1);
  else process.exitCode = 1;
});
