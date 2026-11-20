import fs from 'node:fs';
import assert from 'node:assert/strict';

// gen_swagger.bat generates docs/openapi/swagger.json before running this check.
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
const source=fs.readFileSync('internal/management/router/waf_pipeline.go','utf8');
const routes = new Set();
for(const match of source.matchAll(/r\.(GET|POST)\("([^"]+)", (?:a|api)\.(\w+)\)/g)) {
  const [,method,path,name]=match;
  routes.add(method + ' /api/v1/pipeline' + path);
  const operation=doc.paths[`/api/v1/pipeline${path}`]?.[method.toLowerCase()];
  assert(operation,`${name}: missing operation`);
  assert(operation.summary,`${name}: missing summary`);
  assert(operation.responses?.['200']?.schema,`${name}: missing response schema`);
  assert(operation.security?.some(s=>'ApiKeyAuth' in s),`${name}: missing authentication`);
  if(method==='POST' && path!=='/feed/sync')assert(operation.parameters?.some(p=>p.in==='body'&&p.schema),`${name}: missing body schema`);
}
assert(routes.has('POST /api/v1/pipeline/ports/save'), 'Missing service configuration entry point');
const documented = new Set();
for (const [path, operations] of Object.entries(doc.paths)) {
  if (!path.startsWith('/api/v1/pipeline/')) continue;
  for (const method of ['get', 'post', 'put', 'patch', 'delete', 'options', 'head']) {
    if (operations[method]) documented.add(method.toUpperCase() + ' ' + path);
  }
}
assert.deepEqual(documented, routes, 'Pipeline documentation and registered routes differ');
console.log(`Pipeline OpenAPI: ${routes.size} operations have response, request and authentication documentation.`);

const groupSchema=Object.entries(doc.definitions).find(([key])=>key.endsWith('.IPGroupConfig'))?.[1];
for(const key of ['allow_groups','deny_groups','denied_regions','block_outside_mainland'])assert(groupSchema?.properties?.[key],`IPGroupConfig: missing ${key}`);
const createGroupSchema=Object.entries(doc.definitions).find(([key])=>key.endsWith('.WafIPGroupAddReq'))?.[1];
assert(createGroupSchema?.properties?.initial_ips,'IP group creation: missing atomic initial_ips');
console.log('Shared access-policy fields and atomic IP-group creation are documented.');
