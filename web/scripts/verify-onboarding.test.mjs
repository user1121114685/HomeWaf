import test from 'node:test';
import assert from 'node:assert/strict';
import {
  normalizeServiceAddress,
  serviceStatus,
  parseListenPort,
  parseServiceHosts,
  suggestedName,
  optionalAccessDefaults,
  parseAllowedSources,
} from '../src/pages/waf/pipeline/onboarding.ts';
test('new NAS offers optional login and dual-stack defaults without changing copied policies', () => {
  const config = { template: { id: 'family-nas' }, policy: { gate: { enabled: true }, network: {} } };
  optionalAccessDefaults(config);
  assert.equal(config.policy.gate.enabled, false);
  assert.equal(config.policy.network.listen_ip_version, 'both');
  const copy = { template: {}, policy: { gate: { enabled: true }, network: { listen_ip_version: 'ipv4' } } };
  optionalAccessDefaults(copy);
  assert.equal(copy.policy.gate.enabled, true);
  assert.equal(copy.policy.network.listen_ip_version, 'ipv4');
});
test('source allowlist validates IPv4 and IPv6 networks and rejects invalid prefixes', () => {
  assert.deepEqual(parseAllowedSources('192.168.1.0/24\n240e:1234::/48\n192.168.1.0/24'), [
    '192.168.1.0/24',
    '240e:1234::/48',
  ]);
  for (const value of ['300.1.2.3', '1.2.3.4/33', '240e::/129', '240e::xyz', 'example.com', '1.2.3.4/-1', '1.2.3.4/'])
    assert.throws(() => parseAllowedSources(value));
});
test('address entry accepts pasted LAN addresses, IPv6 and explicit HTTPS', () => {
  assert.equal(normalizeServiceAddress(' 192.168.1.20:5000 '), 'http://192.168.1.20:5000');
  assert.equal(normalizeServiceAddress('https://nas.example.test/'), 'https://nas.example.test');
  assert.equal(normalizeServiceAddress('[::1]:8080'), 'http://[::1]:8080');
});
test('address validation prevents credentials and silently dropping a path', () => {
  for (const input of [
    '',
    'ftp://host.test',
    'https://user:password@host.test',
    'http://host.test/path',
    'http://host.test?key=secret',
    'http://host.test/#fragment',
    'http://host.test:99999',
  ])
    assert.throws(() => normalizeServiceAddress(input));
});
test('listener port must be explicitly entered and remain in range', () => {
  assert.equal(parseListenPort(' 8443 '), 8443);
  assert.equal(parseListenPort('65535'), 65535);
  for (const value of ['', '0', '65536', '-1', '8e3', '80.5', 'http://80']) assert.throws(() => parseListenPort(value));
});
test('allowed hosts accept domains and IP addresses without protocols or ports', () => {
  assert.deepEqual(parseServiceHosts('nas.example.test，192.0.2.10\n2001:db8::1,nas.example.test'), [
    'nas.example.test',
    '192.0.2.10',
    '2001:db8::1',
  ]);
  assert.deepEqual(parseServiceHosts(''), []);
  assert.deepEqual(parseServiceHosts('*.example.test'), ['*.example.test']);
  for (const value of [
    'https://nas.test',
    'nas.test:8443',
    'nas.test/path',
    'nas.test?x=1',
    'user@nas.test',
    '2001:xyz::1',
  ])
    assert.throws(() => parseServiceHosts(value));
});
test('generated names describe the selected service and actual host', () => {
  assert.equal(suggestedName('public-site', '192.168.1.2:8080'), '公开网站 · 192.168.1.2');
});
test('running requires the actual listener; disabled access and errors are never healthy', () => {
  const row = { config: { listen_enabled: true, policy: { access: { enabled: true } } }, listener: { running: false } };
  assert.equal(serviceStatus(row).label, '未监听');
  row.listener.running = true;
  assert.equal(serviceStatus(row).label, '运行中');
  row.config.policy.access.enabled = false;
  assert.equal(serviceStatus(row).label, '访问已关闭');
  row.listener.error = 'port occupied';
  assert.equal(serviceStatus(row).label, '启动异常');
  row.listener.error = '';
  row.config.listen_enabled = false;
  assert.equal(serviceStatus(row).label, '待启用');
});
