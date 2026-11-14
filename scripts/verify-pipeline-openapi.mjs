import fs from 'node:fs';
import assert from 'node:assert/strict';

// Generate docs/openapi/swagger.json first with gen_swagger.bat --generate-only.
const doc=JSON.parse(fs.readFileSync('docs/openapi/swagger.json','utf8'));
for(const action of ['add','edit','list','sync','detail','del']){
  const method=['detail','del'].includes(action)?'get':'post';
  const operation=doc.paths[`/api/v1/threatip/channel/${action}`]?.[method];
  assert(operation?.security?.some(s=>'ApiKeyAuth' in s),`Threat channel ${action}: missing authenticated operation`);
}
for(const suffix of ['WafThreatIPChannelAddReq','WafThreatIPChannelEditReq','ThreatIPChannel']){
  const schema=Object.entries(doc.definitions).find(([key])=>key.endsWith('.'+suffix))?.[1];
  for(const key of ['signature_url','feed_id','signing_key_id','signing_public_key'])assert(schema?.properties?.[key],`${suffix}: missing ${key}`);
}
const source=fs.readFileSync('router/waf_pipeline.go','utf8');
let count=0;
for(const match of source.matchAll(/r\.(GET|POST)\("([^"]+)", a\.(\w+)\)/g)) {
  const [,method,path,name]=match;
  const operation=doc.paths[`/api/v1/pipeline${path}`]?.[method.toLowerCase()];
  assert(operation,`${name}: missing operation`);
  assert(operation.summary,`${name}: missing summary`);
  assert(operation.responses?.['200']?.schema,`${name}: missing response schema`);
  assert(operation.security?.some(s=>'ApiKeyAuth' in s),`${name}: missing authentication`);
  if(method==='POST' && path!=='/feed/sync')assert(operation.parameters?.some(p=>p.in==='body'&&p.schema),`${name}: missing body schema`);
  count++;
}
assert.equal(count,32);
console.log(`Pipeline OpenAPI: ${count} operations have response, request and authentication documentation.`);

const groupSchema=Object.entries(doc.definitions).find(([key])=>key.endsWith('.IPGroupConfig'))?.[1];
for(const key of ['allow_groups','deny_groups','denied_regions','block_outside_mainland'])assert(groupSchema?.properties?.[key],`IPGroupConfig: missing ${key}`);
const createGroupSchema=Object.entries(doc.definitions).find(([key])=>key.endsWith('.WafIPGroupAddReq'))?.[1];
assert(createGroupSchema?.properties?.initial_ips,'IP group creation: missing atomic initial_ips');
console.log('Shared access-policy fields and atomic IP-group creation are documented.');
