import test from 'node:test';
import assert from 'node:assert/strict';
import {
  diagnosticNumber,
  diagnosticDetail,
  diagnosticAction,
  diagnosticModes,
  serviceReturnPath,
  filterDiagnosticEvents,
} from '../src/pages/waf/pipeline/diagnostics.ts';

test('missing or failed diagnostic counters cannot appear as successful zero readings', () => {
  for (const value of [undefined, null, NaN, Infinity, '0']) assert.equal(diagnosticNumber(value), '未提供');
  assert.equal(diagnosticNumber(0), '0');
});
test('diagnostic editing detours only return to supported local service destinations', () => {
  for (const value of [
    'https://example.com',
    '//example.com',
    '/waf-pipeline/ports/../admin',
    '/waf-pipeline/ports#bad',
    null,
  ]) {
    assert.equal(serviceReturnPath(value, true), '/waf-pipeline/ports');
  }
  assert.equal(
    serviceReturnPath('/waf-pipeline/ports?search=NAS&view=table'),
    '/waf-pipeline/ports?search=NAS&view=table',
  );
  assert.equal(
    serviceReturnPath('/waf-pipeline/ports/5000/diagnostics?tab=state', true),
    '/waf-pipeline/ports/5000/diagnostics?tab=state',
  );
});
test('event filtering distinguishes state changes, observation and blocked requests', () => {
  const rows = [
    { kind: 'request', verdict: 'allow', observe: true, client_ip: '192.0.2.1', reason: 'rule:scan' },
    { kind: 'request', verdict: 'deny', client_ip: '192.0.2.2', reason: 'blacklist:custom' },
    { kind: 'state', verdict: 'ban', client_ip: '192.0.2.2', reason: 'released' },
  ];
  assert.equal(filterDiagnosticEvents(rows, '192.0.2.2', 'state')[0].reason, 'released');
  assert.equal(filterDiagnosticEvents(rows, '手动 IP 黑名单', 'request')[0].verdict, 'deny');
  assert.equal(diagnosticAction(rows[0]), '放行 · 观察命中');
  assert.equal(filterDiagnosticEvents(rows, 'does-not-exist', '').length, 0);
});
test('malformed untrusted event details cannot become a fabricated decision trace', () => {
  for (const detail_json of ['broken', 'null', '[]', '42']) assert.equal(diagnosticDetail({ detail_json }), null);
  assert.deepEqual(diagnosticDetail({ detail_json: '{"event":"released"}' }), { event: 'released' });
});
test('mixed detection policies preserve independent observation and blocking modes', () => {
  assert.deepEqual(
    diagnosticModes({
      ruleset: { bot: 'observe', qps: { mode: 'block' }, rules: [{ mode: 'off' }, { mode: 'observe' }] },
    }),
    { block: 1, observe: 2 },
  );
});
