import { spawnSync } from 'node:child_process';
import { appendFileSync, readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const stableVersion = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;

function compareVersions(a, b) {
  const left = a.slice(1).split('.').map(BigInt);
  const right = b.slice(1).split('.').map(BigInt);
  for (let i = 0; i < left.length; i++) {
    if (left[i] !== right[i]) return left[i] > right[i] ? 1 : -1;
  }
  return 0;
}

function command(program, args) {
  const result = spawnSync(program, args, { encoding: 'utf8', windowsHide: true });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${program} failed: ${result.stderr.trim()}`);
  return result.stdout.trim();
}

export function readRelease(repository, version, run = spawnSync) {
  const result = run('gh', ['api', `repos/${repository}/releases/tags/${version}`], {
    encoding: 'utf8', windowsHide: true,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    if (result.status === 1 && /\(HTTP 404\)/.test(result.stderr)) return null;
    throw new Error(`Could not check release ${version}: ${result.stderr.trim()}`);
  }
  const release = JSON.parse(result.stdout);
  if (release.tag_name !== version || typeof release.draft !== 'boolean') {
    throw new Error(`Invalid release response for ${version}`);
  }
  return release;
}

export function prepareRelease({ env, defaultBranch, git = args => command('git', args), getRelease = readRelease }) {
  const manual = env.GITHUB_EVENT_NAME === 'workflow_dispatch';
  const tagged = env.GITHUB_REF_TYPE === 'tag';
  if (!manual && !(env.GITHUB_EVENT_NAME === 'push' && tagged)) {
    throw new Error('Releases require a manual run or a version tag push');
  }
  if (!tagged && (!defaultBranch || env.GITHUB_REF_NAME !== defaultBranch)) {
    throw new Error('Create new releases from the default branch only');
  }
  if (!/^\d+$/.test(env.GITHUB_RUN_ID || '')) throw new Error('Missing workflow run ID');
  if (!/^[\w.-]+\/[\w.-]+$/.test(env.GITHUB_REPOSITORY || '')) throw new Error('Invalid repository');
  const requested = (env.RELEASE_VERSION || '').trim();
  if (requested && !stableVersion.test(requested)) throw new Error('Version must be vX.Y.Z without a prerelease suffix');
  if (tagged && requested && requested !== env.GITHUB_REF_NAME) throw new Error('Requested version must match the selected tag');

  git(['fetch', 'origin', '--tags']);
  const head = git(['rev-parse', 'HEAD']);
  if (head !== env.GITHUB_SHA) throw new Error('Checkout does not match the workflow commit');
  const tags = git(['for-each-ref', '--format=%(refname:strip=2)%09%(contents:subject)', 'refs/tags'])
    .split('\n').filter(Boolean).map(line => {
      const [name, subject] = line.split('\t');
      return { name, subject };
    }).filter(tag => stableVersion.test(tag.name));
  tags.sort((a, b) => compareVersions(a.name, b.name));
  const latest = tags.at(-1)?.name;
  // A rerun must resume its own draft/tag instead of consuming another version.
  const marker = `HomeWaf release for workflow run ${env.GITHUB_RUN_ID}`;
  const resumed = tags.filter(tag => tag.subject === marker);
  if (resumed.length > 1) throw new Error('Multiple release tags belong to this workflow run');
  let version = tagged ? env.GITHUB_REF_NAME : requested || resumed[0]?.name;
  if (!version) {
    if (!latest) throw new Error('Specify the first release version explicitly');
    version = latest.replace(/\d+$/, patch => String(BigInt(patch) + 1n));
  }
  if (!stableVersion.test(version)) throw new Error('Version must be vX.Y.Z without a prerelease suffix');
  const exists = tags.some(tag => tag.name === version);
  if (tagged && !exists) throw new Error('Selected release tag does not exist');
  if (exists && git(['rev-parse', `refs/tags/${version}^{commit}`]) !== head) {
    throw new Error(`Tag ${version} belongs to a different commit; select that tag to retry it`);
  }
  if (latest && compareVersions(version, latest) < 0) throw new Error('Refusing to publish an older release as latest');
  const release = getRelease(env.GITHUB_REPOSITORY, version);
  if (release && !release.draft) throw new Error(`Refusing to overwrite public release ${version}`);
  if (release && !exists) throw new Error(`Release ${version} exists without its tag`);
  if (!exists) {
    git(['-c', 'user.name=github-actions[bot]', '-c', 'user.email=41898282+github-actions[bot]@users.noreply.github.com',
      'tag', '-a', version, head, '-m', marker]);
    // Never force-push: a concurrent tag creation must fail without replacing it.
    git(['push', 'origin', `refs/tags/${version}`]);
  }
  // The GITHUB_TOKEN tag push does not trigger a second workflow. This run builds,
  // verifies and publishes the resolved tag through the package job output.
  git(['checkout', '--detach', `refs/tags/${version}`]);
  return version;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
  const version = prepareRelease({ env: process.env, defaultBranch: event.repository?.default_branch });
  appendFileSync(process.env.GITHUB_OUTPUT, `version=${version}\n`);
  console.log(`Release version: ${version}`);
}
