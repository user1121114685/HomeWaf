import { execFileSync } from 'node:child_process';

const versionPattern = /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;

export function resolveBuildVersion({ root = process.cwd(), version = process.env.HOMEWAF_VERSION, now = new Date(), git } = {}) {
  const readGit = git || ((args) => {
    try { return execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim(); }
    catch { return ''; }
  });
  // Only an ancestor tag describes this source tree; an unrelated newer tag must not raise its baseline.
  const base = version || readGit(['describe', '--tags', '--match', 'v[0-9]*.[0-9]*.[0-9]*', '--abbrev=0', 'HEAD']);
  if (!versionPattern.test(base)) throw new Error('Set HOMEWAF_VERSION to a valid vX.Y.Z version, or build from a checkout with an ancestor release tag.');
  const commit = readGit(['rev-parse', '--short=12', 'HEAD']) || 'unknown';
  const dirty = !!readGit(['status', '--porcelain', '--untracked-files=normal']);
  const buildId = now.toISOString().replace(/[-:TZ.]/g, '') + '.g' + commit + (dirty ? '.dirty' : '');
  // Build metadata is not a prerelease: v2.0.2 must never replace a local build based on v2.0.2.
  return { version: version || `${base.split('+')[0]}+local.${buildId}`, buildId, commit, builtAt: now.toISOString(), channel: version ? 'release' : 'internal' };
}

export function buildMetadataFlags(info) {
  return `-X HomeWaf/global.GWAF_BUILD_ID=${info.buildId} -X HomeWaf/global.GWAF_BUILD_COMMIT=${info.commit} -X HomeWaf/global.GWAF_BUILD_TIME=${info.builtAt} -X HomeWaf/global.GWAF_BUILD_CHANNEL=${info.channel}`;
}
