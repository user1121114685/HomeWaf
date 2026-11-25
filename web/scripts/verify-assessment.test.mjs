import test from 'node:test';
import assert from 'node:assert/strict';
import { importAssessmentLog, serviceTarget } from '../src/pages/waf/protocol-defense/assessment.ts';

test('import strips historical identity and credentials and does not equate blocks with attacks', () => {
  const rows = importAssessmentLog({ detail_records: [{ host:'private.invalid', url:'https://private.invalid/a', raw_query:'x=1', action:'阻止', status_code:403, method:'GET', header:JSON.stringify({Host:['private.invalid'],Cookie:['SECRET'],Authorization:['SECRET'],Origin:['https://private.invalid'],Referer:['https://private.invalid/a'],'X-Forwarded-For':['192.0.2.1']}) }] });
  assert.equal(rows[0].kind,'unknown');
  assert.equal(rows[0].path,'/a?x=1');
  assert.equal(rows[0].headers['X-Forwarded-For'],'192.0.2.1');
  assert.doesNotMatch(JSON.stringify(rows),/private\.invalid|SECRET/);
  assert.equal(importAssessmentLog([{url:'/',header:'User-Agent: scanner\r\nHost: private.invalid\r\n'}])[0].headers['User-Agent'],'scanner');
});
test('malformed imports fail without silently losing samples', () => {
  for(const value of [{},[],[{url:'//private.invalid/'}],[{url:'/',header:'broken'}],[{url:'/',header:'{broken'}],Array(257).fill({url:'/'})]) assert.throws(()=>importAssessmentLog(value));
});
test('selected service determines domain, route hostname, wildcard and IP fallback', () => {
  const config = {protocol:'https',port:15000,host_names:['selected.example.test'],policy:{network:{}}};
  assert.equal(serviceTarget(config),'https://selected.example.test:15000');
  config.host_names=['*.example.test'];
  assert.equal(serviceTarget(config),'https://assessment.example.test:15000');
  config.host_names=[];
  assert.equal(serviceTarget(config),'https://127.0.0.1:15000');
  config.policy.network.listen_ip_version='ipv6';
  assert.equal(serviceTarget(config),'https://[::1]:15000');
});
