import test from 'node:test';
import assert from 'node:assert/strict';
import {
  requestOutcome,
  logTimeBounds,
  normalizeLogColumns,
  recommendedLogColumns,
  logColumns,
} from '../src/pages/waf/attack/logPresentation.ts';

test('allowed observe-only matches never appear normal', () => {
  assert.equal(requestOutcome({ action: '放行', rule: 'ssrf', log_only_mode: 1, status_code: 200 }).abnormal, true);
  assert.equal(requestOutcome({ action: '放行', rule: '', log_only_mode: 0, status_code: 200 }).abnormal, false);
  assert.equal(requestOutcome({ action: '放行', status_code: 500 }).abnormal, true);
  assert.equal(requestOutcome({ action: '阻止', status_code: 403 }).blocked, true);
});
test('date-only ranges include the final day and reject cleared or inverted ranges', () => {
  const range = logTimeBounds(['2026-11-14', '2026-11-14']);
  assert.equal(Number(range.unix_add_time_end) - Number(range.unix_add_time_begin), 86400000 - 1);
  assert.equal(logTimeBounds(null), null);
  assert.equal(logTimeBounds(['', '']), null);
  assert.equal(logTimeBounds(['2026-11-15', '2026-11-14']), null);
});
test('saved columns retain order and exclusions as fields are introduced', () => {
  const available = logColumns((zh) => zh).map((column) => column.colKey);
  assert.deepEqual(normalizeLogColumns(['url', 'action', 'host', 'op'], available), [
    'url',
    'outcome',
    'service',
    'op',
  ]);
  assert.deepEqual(normalizeLogColumns(['src_ip', 'outcome', 'op'], available), ['src_ip', 'outcome', 'op']);
  assert.deepEqual(normalizeLogColumns(null, available), recommendedLogColumns);
});
