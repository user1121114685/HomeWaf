import assert from 'node:assert/strict';
import test from 'node:test';
import { releaseNoteGroups, releaseNoteItems } from '../src/utils/releaseNotes.ts';

test('release notes remove full commit identifiers and keep version groups', () => {
  const body = `HomeWaf v2.0.3\n## Changelog\n* ${'a'.repeat(40)} fix(ui): Restore upgrade button\n* ${'b'.repeat(40)} feat: Add service diagnostics`;
  assert.deepEqual(releaseNoteItems(body), ['Restore upgrade button', 'Add service diagnostics']);
  assert.deepEqual(releaseNoteGroups('v2.0.3', body).map(group => group.version), ['v2.0.3']);
  assert.deepEqual(releaseNoteGroups('v2.0.3', '', [
    { version: 'v2.0.3', items: ['Fix updates'] },
    { version: 'v2.0.2', items: ['Improve certificates'] },
  ]).map(group => group.version), ['v2.0.3', 'v2.0.2']);
});

test('remote markup stays text and empty descriptions do not invent changes', () => {
  assert.deepEqual(releaseNoteItems('<img src=x onerror=alert(1)>'), ['<img src=x onerror=alert(1)>']);
  assert.deepEqual(releaseNoteItems('## Changelog\n'), []);
});
