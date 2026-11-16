import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { prepareRelease, readRelease } from './prepare-release.mjs';

function fixture(t) {
  const cache = path.resolve('.cache/release-tests');
  mkdirSync(cache, { recursive: true });
  const dir = mkdtempSync(path.join(cache, 'version-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const remote = path.join(dir, 'remote.git');
  const checkout = path.join(dir, 'checkout');
  const run = (cwd, args) => execFileSync('git', args, {
    cwd, encoding: 'utf8', windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
  }).trim();
  run(dir, ['init', '--bare', remote]);
  run(dir, ['init', '--initial-branch=master', checkout]);
  const git = args => run(checkout, args);
  git(['config', 'user.name', 'Release test']);
  git(['config', 'user.email', 'release-test@example.invalid']);
  git(['config', 'commit.gpgsign', 'false']);
  git(['config', 'tag.gpgsign', 'false']);
  git(['remote', 'add', 'origin', remote]);
  git(['commit', '--allow-empty', '-m', 'Initial fixture']);
  for (const tag of ['v1.99.0', 'v2.0.9', 'v2.0.10', 'v99.0.0-beta.1', 'unrelated']) git(['tag', tag]);
  git(['push', 'origin', 'master', '--tags']);
  git(['commit', '--allow-empty', '-m', 'Release candidate']);
  git(['push', 'origin', 'master']);
  const env = {
    GITHUB_EVENT_NAME: 'workflow_dispatch', GITHUB_REF_TYPE: 'branch', GITHUB_REF_NAME: 'master',
    GITHUB_SHA: git(['rev-parse', 'HEAD']), GITHUB_REPOSITORY: 'example/HomeWaf', GITHUB_RUN_ID: '1234',
  };
  const options = { env, defaultBranch: 'master', git, getRelease: () => null };
  return { git, env, options, remoteGit: args => run(remote, args) };
}

test('manual releases increment the highest stable patch, push the tag and resume the same run', t => {
  const { git, env, options, remoteGit } = fixture(t);
  assert.equal(prepareRelease(options), 'v2.0.11');
  assert.equal(remoteGit(['rev-parse', 'refs/tags/v2.0.11^{commit}']), env.GITHUB_SHA);
  assert.equal(git(['describe', '--tags', '--exact-match']), 'v2.0.11');
  assert.equal(git(['for-each-ref', '--format=%(contents:subject)', 'refs/tags/v2.0.11']), 'HomeWaf release for workflow run 1234');
  assert.equal(prepareRelease({ ...options, getRelease: () => ({ draft: true }) }), 'v2.0.11');
  assert.equal(remoteGit(['tag', '--list', 'v2.0.12']), '');
  env.GITHUB_RUN_ID = '1235';
  assert.equal(prepareRelease(options), 'v2.0.12');
});

test('explicit versions and tag-triggered retries keep the selected version', t => {
  const { env, options, git } = fixture(t);
  env.RELEASE_VERSION = 'v2.1.0';
  assert.equal(prepareRelease(options), 'v2.1.0');
  env.GITHUB_EVENT_NAME = 'push';
  env.GITHUB_REF_TYPE = 'tag';
  env.GITHUB_REF_NAME = 'v2.1.0';
  delete env.RELEASE_VERSION;
  assert.equal(prepareRelease({ ...options, getRelease: () => ({ draft: true }) }), 'v2.1.0');
  env.GITHUB_EVENT_NAME = 'workflow_dispatch';
  env.GITHUB_RUN_ID = '1236';
  assert.equal(prepareRelease(options), 'v2.1.0');
  assert.equal(git(['tag', '--list', 'v2.1.1']), '');
  env.RELEASE_VERSION = 'v3.0.0';
  assert.throws(() => prepareRelease(options), /match the selected tag/);
});

test('published releases, API failures, foreign commits and older versions are never overwritten', t => {
  const { env, options, git, remoteGit } = fixture(t);
  env.RELEASE_VERSION = 'v2.0.10';
  assert.throws(() => prepareRelease(options), /different commit/);
  assert.equal(remoteGit(['rev-parse', 'refs/tags/v2.0.10^{commit}']), git(['rev-parse', 'HEAD~1']));
  env.RELEASE_VERSION = 'v1.0.0';
  assert.throws(() => prepareRelease(options), /older release/);
  env.RELEASE_VERSION = 'v2.0.11';
  assert.throws(() => prepareRelease({ ...options, getRelease: () => ({ draft: false }) }), /overwrite public release/);
  assert.throws(() => prepareRelease({ ...options, getRelease: () => ({ draft: true }) }), /without its tag/);
  assert.throws(() => prepareRelease({ ...options, getRelease: () => { throw new Error('GitHub unavailable'); } }), /GitHub unavailable/);
  assert.equal(remoteGit(['tag', '--list', 'v2.0.11']), '');
  assert.equal(prepareRelease(options), 'v2.0.11');
  assert.throws(() => prepareRelease({ ...options, getRelease: () => ({ draft: false }) }), /overwrite public release/);
  assert.equal(remoteGit(['tag', '--list', 'v2.0.12']), '');
});

test('invalid inputs and non-default branches fail before contacting the remote', () => {
  const env = {
    GITHUB_EVENT_NAME: 'workflow_dispatch', GITHUB_REF_TYPE: 'branch', GITHUB_REF_NAME: 'master',
    GITHUB_RUN_ID: '1234', GITHUB_REPOSITORY: 'example/HomeWaf',
  };
  const options = { env, defaultBranch: 'master', git: () => assert.fail('Must not call Git') };
  for (const version of ['v2.0.1-beta.1', '2.0.1', 'v02.0.1', 'v2.0.1\nversion=v3.0.0', '--force']) {
    assert.throws(() => prepareRelease({ ...options, env: { ...env, RELEASE_VERSION: version } }), /Version must be/);
  }
  assert.throws(() => prepareRelease({ ...options, env: { ...env, GITHUB_REF_NAME: 'feature' } }), /default branch/);
  assert.throws(() => prepareRelease({ ...options, env: { ...env, GITHUB_EVENT_NAME: 'pull_request' } }), /manual run or a version tag/);
});

test('a stale checkout and a concurrent remote tag cannot redirect a release', t => {
  const { env, options, git, remoteGit } = fixture(t);
  assert.throws(() => prepareRelease({ ...options, env: { ...env, GITHUB_SHA: 'wrong' } }), /workflow commit/);
  const previous = git(['rev-parse', 'HEAD~1']);
  let raced = false;
  const racingGit = args => {
    if (args[0] === 'push' && !raced) {
      raced = true;
      remoteGit(['tag', 'v2.0.11', previous]);
    }
    return git(args);
  };
  assert.throws(() => prepareRelease({ ...options, git: racingGit }), /already exists/);
  assert.equal(remoteGit(['rev-parse', 'refs/tags/v2.0.11^{commit}']), previous);
  assert.equal(git(['rev-parse', 'HEAD']), env.GITHUB_SHA);
});

test('release lookup only treats HTTP 404 as absent and validates successful responses', () => {
  const lookup = result => readRelease('example/HomeWaf', 'v2.0.11', (program, args) => {
    assert.equal(program, 'gh');
    assert.deepEqual(args, ['api', 'repos/example/HomeWaf/releases/tags/v2.0.11']);
    return result;
  });
  assert.equal(lookup({ status: 1, stderr: 'gh: Not Found (HTTP 404)' }), null);
  for (const stderr of ['gh: Forbidden (HTTP 403)', 'gh: Bad Gateway (HTTP 502)', 'connection refused']) {
    assert.throws(() => lookup({ status: 1, stderr }), /Could not check release/);
  }
  assert.throws(() => lookup({ error: new Error('gh missing') }), /gh missing/);
  for (const body of [{}, { tag_name: 'v2.0.12', draft: true }, { tag_name: 'v2.0.11' }]) {
    assert.throws(() => lookup({ status: 0, stdout: JSON.stringify(body) }), /Invalid release response/);
  }
  for (const draft of [true, false]) {
    const body = { tag_name: 'v2.0.11', draft };
    assert.deepEqual(lookup({ status: 0, stdout: JSON.stringify(body) }), body);
  }
});
