// Verify the tracked dataset offline. --download explicitly restores the pinned file.
import fs from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const target = path.join(root, "cmd/homewaf/exedata/ip2region_v6.xdb");
const metadata = JSON.parse(
  await fs.readFile(target.replace(/\.xdb$/, ".source.json"), "utf8"),
);
const expectedURL = `https://raw.githubusercontent.com/lionsoul2014/ip2region/${metadata.commit}/data/ip2region_v6.xdb`;
if (
  !/^[a-f\d]{40}$/.test(metadata.commit) ||
  !/^[a-f\d]{64}$/.test(metadata.sha256) ||
  metadata.source !== expectedURL ||
  !Number.isInteger(metadata.bytes) ||
  metadata.bytes < 256 ||
  metadata.bytes > 128 * 1024 * 1024
)
  throw new Error("Invalid pinned IPv6 data manifest");
function valid(data) {
  return (
    data.length === metadata.bytes &&
    createHash("sha256").update(data).digest("hex") === metadata.sha256
  );
}
let existing;
try {
  existing = await fs.readFile(target);
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
if (existing && valid(existing)) {
  console.log("Bundled IPv6 database verified (cached, no download).");
} else {
  if (!process.argv.includes("--download"))
    throw new Error(
      "Tracked IPv6 database is missing or changed. Restore it from Git or run node scripts/prepare-ip-databases.mjs --download.",
    );
  console.log(
    `Preparing bundled IPv6 database (${(metadata.bytes / 1048576).toFixed(1)} MiB, pinned ${metadata.commit.slice(0, 12)})...`,
  );
  const response = await fetch(expectedURL, {
    signal: AbortSignal.timeout(300000),
  });
  if (!response.ok)
    throw new Error(`IPv6 database download failed: HTTP ${response.status}`);
  const chunks = [];
  let length = 0;
  for await (const chunk of response.body) {
    length += chunk.length;
    if (length > metadata.bytes)
      throw new Error("IPv6 download exceeds the pinned size");
    chunks.push(chunk);
  }
  const data = Buffer.concat(chunks);
  if (!valid(data))
    throw new Error(
      "IPv6 database size or SHA-256 mismatch; existing file was preserved",
    );
  const temporary = path.join(
    root,
    ".cache",
    "ip-databases",
    `ipv6-${process.pid}.xdb`,
  );
  await fs.mkdir(path.dirname(temporary), { recursive: true });
  try {
    await fs.writeFile(temporary, data);
    await fs.mkdir(path.dirname(target), { recursive: true });
    await fs.rename(temporary, target);
  } finally {
    await fs.rm(temporary, { force: true });
  }
  console.log("Pinned IPv6 database downloaded and verified for embedding.");
}
