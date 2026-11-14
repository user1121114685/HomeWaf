import test from 'node:test';
import assert from 'node:assert/strict';
import { newPort, observationConfig, publicTemplate, mapBounded } from '../src/pages/waf/pipeline/model.ts';

function template() {
  return { schema_version: 1, id: 'family', name: '家庭', version: 1, builtin: true, policy: {
    access: {enabled:false}, ip_group:{entries:[{type:'allow',trust:'trusted',value:'192.168.1.0/24'}],unlisted_action:'deny'},
    gate:{enabled:true}, ruleset:{qps:{mode:'block'},cc:{mode:'block'},bot:'block',owasp:'off',rules:[{id:'a',mode:'block'},{id:'b',mode:'off'}]},
    hit_policy:{enabled:true,lockdown:{enabled:true}},honeypot:{enabled:true},network:{trusted_proxies:['10.0.0.0/8']},
  }};
}
test('new port and applied template own independent mutable policy copies',()=>{
  const source=template(),a=newPort(source,5000),b=newPort(source,5001);
  a.policy.ruleset.rules[0].mode='observe';a.policy.gate.enabled=false;
  assert.equal(b.policy.ruleset.rules[0].mode,'block');assert.equal(source.policy.gate.enabled,true);
  assert.equal(a.listen_enabled,false);assert.equal(a.revision,0);
});
test('observation startup retains access controls and never activates disabled detectors',()=>{
  const original=newPort(template()),next=observationConfig(original);
  assert.equal(next.policy.access.enabled,false);assert.equal(next.policy.ip_group.unlisted_action,'deny');assert.equal(next.policy.gate.enabled,true);
  assert.equal(next.policy.ruleset.rules[0].mode,'observe');assert.equal(next.policy.ruleset.rules[1].mode,'off');assert.equal(next.policy.ruleset.owasp,'off');
  assert.equal(next.policy.hit_policy.enabled,false);assert.equal(next.policy.hit_policy.lockdown.enabled,false);assert.equal(next.policy.honeypot.enabled,false);
  assert.equal(original.policy.ruleset.rules[0].mode,'block');assert.equal(original.policy.hit_policy.enabled,true);
});
test('observation startup includes migrated scripts URL rules and local AI',()=>{
  const config=newPort(template());config.policy.ruleset.ai='block';config.policy.ruleset.plugins={mode:'block',ids:['filter']};config.policy.ruleset.scripts=[{mode:'block'},{mode:'off'}];config.policy.ruleset.urls=[{mode:'block'}];
  const next=observationConfig(config);assert.equal(next.policy.ruleset.ai,'observe');assert.deepEqual(next.policy.ruleset.scripts.map(rule=>rule.mode),['observe','off']);assert.equal(next.policy.ruleset.urls[0].mode,'observe');assert.equal(config.policy.ruleset.ai,'block');
  assert.equal(next.policy.ruleset.plugins.mode,'observe');assert.equal(config.policy.ruleset.plugins.mode,'block');
});
test('template projection excludes manual identities without mutating port config',()=>{
  const port=newPort(template());const exported=publicTemplate(port,'独立模板');
  assert.deepEqual(exported.policy.ip_group.entries,[]);assert.deepEqual(exported.policy.network.trusted_proxies,[]);
  assert.equal(port.policy.ip_group.entries.length,1);assert.equal(port.policy.network.trusted_proxies.length,1);
  assert.equal('initial_password' in exported,false);assert.equal('secrets' in exported,false);
});
test('batch preview limits concurrent API calls and preserves port result order',async()=>{
  let active=0,peak=0;const values=Array.from({length:20},(_,i)=>i);
  const out=await mapBounded(values,async value=>{active++;peak=Math.max(peak,active);await new Promise(resolve=>setTimeout(resolve,value%3));active--;return value*2},4);
  assert.equal(peak,4);assert.deepEqual(out,values.map(v=>v*2));assert.equal(active,0);
});


test('portable templates omit shared group identities while observation preserves source controls',()=>{
  const port=newPort(template());Object.assign(port.policy.ip_group,{allow_groups:['family'],deny_groups:['scanners'],block_outside_mainland:true,denied_regions:[{country:'Example'}]});
  const exported=publicTemplate(port,'不包含本地引用');assert.deepEqual(exported.policy.ip_group.allow_groups,[]);assert.deepEqual(exported.policy.ip_group.deny_groups,[]);assert.equal(exported.policy.ip_group.block_outside_mainland,true);
  const observed=observationConfig(port);assert.deepEqual(observed.policy.ip_group,port.policy.ip_group);assert.deepEqual(port.policy.ip_group.allow_groups,['family']);
});
