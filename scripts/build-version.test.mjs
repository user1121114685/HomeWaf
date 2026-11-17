import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveBuildVersion } from './build-version.mjs';
const now = new Date('2026-11-17T14:00:00.123Z');
const git = (args) => ({ describe: 'v2.0.2', 'rev-parse': '2593713c1234', status: ' M service/waf_service/waf_pipeline.go' })[args[0]];
test('local builds use their ancestor release with a distinct internal build identity', () => {
  const build = resolveBuildVersion({ now, git, version: '' });
  assert.equal(build.version, 'v2.0.2+local.20261117140000123.g2593713c1234.dirty');
  assert.equal(build.channel, 'internal');
  assert.notEqual(resolveBuildVersion({ now: new Date(now.getTime() + 1), git, version: '' }).buildId, build.buildId);
});
test('an explicit release version is preserved and an unversioned archive fails closed', () => {
  assert.equal(resolveBuildVersion({ now, git, version: 'v2.0.3' }).version, 'v2.0.3');
  assert.throws(() => resolveBuildVersion({ now, git: () => '', version: '' }));
  assert.throws(() => resolveBuildVersion({ now, git, version: 'invalid version' }));
});
