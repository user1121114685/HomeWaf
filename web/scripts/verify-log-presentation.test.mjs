import test from 'node:test';
import assert from 'node:assert/strict';
import { requestDiagnosticView } from '../src/pages/waf/attack/detail/diagnostics.ts';
import {
  requestOutcome,
  logTimeBounds,
  normalizeLogColumns,
  recommendedLogColumns,
  logColumns,
} from '../src/pages/waf/attack/logPresentation.ts';

test('observed requests are separate from normal traffic and abnormal outcomes', () => {
  for (const rule of ['csrf', 'ssrf']) {
    const outcome = requestOutcome({ action: '放行', rule, log_only_mode: 1, status_code: 200 });
    assert.equal(outcome.abnormal, false);
    assert.equal(outcome.observed, true);
    assert.equal(outcome.matched, true);
  }
  assert.equal(requestOutcome({ action: '放行', rule: '', log_only_mode: 0, status_code: 200 }).abnormal, false);
  assert.equal(requestOutcome({ action: '放行', rule: '', log_only_mode: 0, status_code: 200 }).observed, false);
  assert.equal(requestOutcome({ action: '放行', status_code: 500 }).abnormal, true);
  assert.equal(requestOutcome({ action: '阻止', status_code: 403 }).blocked, true);
  for (const row of [
    { action: '放行', rule: 'csrf', log_only_mode: 1, status_code: 500 },
    { action: '阻止', rule: 'sqli', log_only_mode: 1, status_code: 403 },
    { action: '放行', rule: 'sqli', log_only_mode: 0, status_code: 200 },
    { action: '', log_only_mode: 1, status_code: 200 },
  ]) {
    assert.equal(requestOutcome(row).abnormal, true);
    assert.equal(requestOutcome(row).observed, false);
  }
});

test('host policy rejection is explained even without detector hits', () => {
  const view = requestDiagnosticView({ action: '阻止', rule: 'host_not_allowed', diagnostics: { state: 'available', hits: [] } });
  assert.match(view.summary, /Host.*拒绝.*未转发/);
  assert.doesNotMatch(view.summary, /没有逐项规则命中/);
});

test('request diagnostics explain observed matches without copying request secrets', () => {
  const detail = {
    req_uuid: 'request-1',
    action: '放行',
    status_code: 200,
    method: 'POST',
    url: '/webapi/entry.cgi?token=PRIVATE-TOKEN',
    cookies: 'PRIVATE-COOKIE',
    body: 'PRIVATE-BODY',
    header: 'Authorization: PRIVATE-AUTH',
    diagnostics: {
      state: 'available',
      hits: [
        {
          category: 'csrf',
          rule_id: 'csrf',
          source: 'header',
          code: 'missing_origin_observe',
          observe: true,
          evidence: 'PRIVATE-EVIDENCE',
        },
      ],
    },
  };
  const view = requestDiagnosticView(detail);
  assert.equal(view.hits[0].position, '请求头');
  assert.match(view.hits[0].reason, /原生 App/);
  assert.match(view.hits[0].mode, /仅观察/);
  assert.match(view.report, /missing_origin_observe/);
  assert.match(view.report, /request-1/);
  assert.doesNotMatch(view.report, /PRIVATE-/);
  assert.match(requestDiagnosticView(detail, 'en_US').report, /Native apps/);
});

test('missing or failed diagnostics never claim the request was clean', () => {
  for (const state of ['missing', 'unavailable', 'invalid', 'unsupported']) {
    const view = requestDiagnosticView({ diagnostics: { state, hits: [{ category: 'ssrf' }] } });
    assert.equal(view.hits.length, 0);
    assert.match(view.report, new RegExp(state));
    assert.doesNotMatch(view.summary, /没有逐项规则命中/);
  }
  assert.equal(requestDiagnosticView({}).state, 'unsupported');
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
