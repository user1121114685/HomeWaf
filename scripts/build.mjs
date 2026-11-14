import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
process.chdir(root);
process.env.CGO_ENABLED ||= "0";
function run(command, args, options = {}) {
  const result = spawnSync(command, args, { stdio: "inherit", ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
function npm(args) {
  if (process.platform !== "win32") return run("npm", args);
  const cli = path.join(
    path.dirname(process.execPath),
    "node_modules",
    "npm",
    "bin",
    "npm-cli.js",
  );
  if (!fs.existsSync(cli))
    throw new Error(
      "npm-cli.js must be installed alongside the selected Node toolchain",
    );
  run(process.execPath, [cli, ...args]);
}
run(process.execPath, ["scripts/prepare-ip-databases.mjs", "--check"]);
if (!process.argv.includes("--skip-install")) npm(["ci", "--prefix", "web"]);
npm(["run", "typecheck", "--prefix", "web"]);
npm(["run", "build", "--prefix", "web"]);
const dist = path.join(root, "public", "dist");
if (path.relative(root, dist) !== path.join("public", "dist"))
  throw new Error("Invalid output directory");
fs.rmSync(dist, { recursive: true, force: true });
fs.cpSync(path.join(root, "web", "dist"), dist, { recursive: true });
fs.writeFileSync(path.join(dist, ".gitkeep"), "");
const legal = path.join(dist, "legal");
fs.mkdirSync(legal, { recursive: true });
for (const name of ["LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md"]) {
  fs.copyFileSync(path.join(root, name), path.join(legal, name));
}
fs.cpSync(path.join(root, "LICENSES"), path.join(legal, "LICENSES"), {
  recursive: true,
});
const web = JSON.parse(fs.readFileSync("web/package.json", "utf8"));
fs.writeFileSync(
  "public/web_frontend.txt",
  `repo=HomeWaf/web\nversion=${web.version}\nsource=local-package-lock\n`,
);
if (!process.argv.includes("--web-only")) {
  fs.mkdirSync("artifacts", { recursive: true });
  const os =
    process.env.GOOS ||
    { win32: "windows", darwin: "darwin" }[process.platform] ||
    "linux";
  const arch =
    process.env.GOARCH || (process.arch === "arm64" ? "arm64" : "amd64");
  const binary =
    os === "windows"
      ? "HomeWaf64.exe"
      : `HomeWaf${os === "darwin" ? "Darwin" : "Linux"}${arch === "arm64" ? "Arm64" : "64"}`;
  const version = process.env.HOMEWAF_VERSION || "v2.0.0-dev";
  if (!/^v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-zA-Z0-9.-]+)?$/.test(version))
    throw new Error("Invalid HOMEWAF_VERSION");
  let flags = `-s -w -X HomeWaf/global.GWAF_RELEASE=true -X HomeWaf/global.GWAF_RELEASE_VERSION=${version} -X HomeWaf/global.GWAF_RELEASE_VERSION_NAME=HomeWaf`;
  for (const [envName, target] of [
    [
      "HOMEWAF_LICENSE_PUBLIC_KEYS",
      "HomeWaf/internal/licensing.TrustedKeysJSON",
    ],
    ["HOMEWAF_PLUGIN_PUBLIC_KEYS", "HomeWaf/plugin/process.TrustedKeysJSON"],
  ]) {
    const file = process.env[envName];
    if (!file) continue;
    const keys = JSON.parse(fs.readFileSync(file, "utf8"));
    if (
      !keys ||
      Array.isArray(keys) ||
      typeof keys !== "object" ||
      !Object.keys(keys).length
    )
      throw new Error(`Invalid ${envName}`);
    for (const [id, key] of Object.entries(keys)) {
      if (
        !/^[A-Za-z0-9_.:-]{1,128}$/.test(id) ||
        typeof key !== "string" ||
        !/^[A-Za-z0-9_-]{43}$/.test(key) ||
        Buffer.from(key, "base64url").length !== 32
      )
        throw new Error(`Invalid public key in ${envName}`);
    }
    flags += ` -X '${target}=${JSON.stringify(keys)}'`;
  }
  run("go", [
    "build",
    "-trimpath",
    `-ldflags=${flags}`,
    "-o",
    path.join("artifacts", binary),
    "./cmd/homewaf",
  ]);
  run("go", [
    "build",
    "-trimpath",
    "-o",
    path.join("artifacts", `homewaf-license${os === "windows" ? ".exe" : ""}`),
    "./cmd/homewaf-license",
  ]);
  console.log(`Built artifacts/${binary}`);
}
