import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { performance } from "node:perf_hooks";

const metadataSettleMs = 1000;

export const excludedRoots = new Set([".git", ".idea", ".cache", ".homewaf-keys", "artifacts", "web", "vendor", "node_modules", "conf", "data", "logs", "ssl", "download", "release", "docs"]);

export function ignoredInput(name) {
  const parts = name.replaceAll("\\", "/").split("/");
  return parts[0].startsWith(".") || excludedRoots.has(parts[0]) || parts.includes("node_modules") || parts.includes(".git");
}

function isBuildInput(name) {
  return (name.endsWith(".go") && !name.endsWith("_test.go")) ||
    ["go.mod", "go.sum", "go.work", "go.work.sum", "public/web_frontend.txt", "wafmangeweb/emergency_page.html"].includes(name) ||
    name.startsWith("cmd/homewaf/exedata/") || name.startsWith("public/dist/");
}

// Filesystem notifications are hints, not evidence of changed build inputs.
// Reconcile directories too, to detect package moves and deleted source files.
export class BuildInputs {
  constructor(root) {
    this.root = root;
    this.snapshot = new Map();
    this.scan();
  }

  scan() {
    const next = new Map();
    const changed = [];
    const visit = (relative) => {
      const absolute = path.join(this.root, relative);
      let entries;
      try { entries = fs.readdirSync(absolute, { withFileTypes: true }); }
      catch (error) { if (error.code === "ENOENT") return; throw error; }
      for (const entry of entries) {
        const name = relative ? `${relative}/${entry.name}` : entry.name;
        if (ignoredInput(name) || entry.isSymbolicLink()) continue;
        if (entry.isDirectory()) { visit(name); continue; }
        if (!entry.isFile() || !isBuildInput(name)) continue;
        let stat, hash, stableSince;
        const old = this.snapshot.get(name);
        try {
          const now = performance.now();
          stat = fs.statSync(path.join(this.root, name), { bigint: true });
          const sameMetadata = old && old.size === stat.size && old.mtime === stat.mtimeNs && old.ctime === stat.ctimeNs;
          stableSince = sameMetadata ? old.stableSince : now;
          // Rapid equal-size writes can share filesystem timestamps. Recheck
          // bytes until metadata settles, then avoid read/access-notification loops.
          hash = sameMetadata && now - stableSince >= metadataSettleMs
            ? old.hash : createHash("sha256").update(fs.readFileSync(path.join(this.root, name))).digest("hex");
          if (!sameMetadata || old.hash !== hash) stableSince = performance.now();
        } catch (error) { if (error.code === "ENOENT") continue; throw error; }
        next.set(name, { size: stat.size, mtime: stat.mtimeNs, ctime: stat.ctimeNs, hash, stableSince });
        if (old?.hash !== hash) changed.push(name);
      }
    };
    visit("");
    for (const name of this.snapshot.keys()) if (!next.has(name)) changed.push(name);
    this.snapshot = next;
    return changed.sort();
  }
}
