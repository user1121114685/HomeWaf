import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root=fileURLToPath(new URL('../..',import.meta.url));
const source=JSON.parse(execFileSync('go',['run','./scripts/pipeline-fixtures'],{cwd:root,encoding:'utf8',env:{...process.env,CGO_ENABLED:'0',GOPROXY:process.env.GOPROXY||'off',GOSUMDB:process.env.GOSUMDB||'off'}}));
const origin=process.env.PIPELINE_TEST_ORIGIN||'http://127.0.0.1:3108';
const artifact=path.join(root,'artifacts','pipeline-ui');fs.mkdirSync(artifact,{recursive:true});
const clone=v=>JSON.parse(JSON.stringify(v));
const configs=new Map(),users=new Map(),tokens=new Set();let templates=clone(source),saveCalls=[],savePayloads=[],failSavePort=0,failConnection=false,failOverview=false;
for(const [port,id,name] of [[5000,'family-nas','家庭 NAS（测试）'],[5001,'public-site','公开小站（测试）']]){
  const tpl=templates.find(t=>t.id===id);configs.set(port,{schema_version:1,port,name,protocol:'http',listen_enabled:false,upstream:'http://127.0.0.1:18080',host_names:[],policy:clone(tpl.policy),template:{id:tpl.id,version:tpl.version},revision:1});users.set(port,[{username:'owner',enabled:true,expires_at:0,remark:'测试账号'}]);
}
const report={checks:[],errors:[],writes:[]};
const ipGroups=[{id:'local',group_code:'local',group_name:'家庭内网',item_count:4},{id:'blocked',group_code:'blocked',group_name:'扫描来源',item_count:2}];
let failGroupAdd=false,groupCreates=0;
const legacyConfig={...clone(configs.get(5000)),port:5300,name:'原有NAS',revision:0,routes:[{hosts:['nas.test'],upstream:'http://127.0.0.1:18080',dial_ip:'127.0.0.1'},{hosts:['camera.test'],upstream:'http://127.0.0.1:18081'}]};
const legacySources=[{code:'nas',name:'nas.test',enabled:true},{code:'camera',name:'camera.test',enabled:true}];
let legacyPreviewHost='';
const browser=await chromium.launch({channel:'chrome',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1100},colorScheme:'light'});
await context.addInitScript(()=>{localStorage.setItem('access_token','isolated-pipeline-ui-test');localStorage.setItem('lang','zh_CN')});
await context.routeWebSocket('**/api/**',ws=>ws.close());
function resultFor(config){const blocked=!config.policy.access.enabled||config.policy.ip_group.unlisted_action==='deny';const gate=config.policy.gate.enabled||config.policy.gate.require_token;return {req_id:'ui-preview',verdict:{action:blocked?'deny':gate?'challenge':'allow',stage:blocked?1:gate?3:5,reason:blocked?'access_policy':gate?'auth_required':'passed'},trace:[{stage:1,check:'access_switch',elapsed_us:2,verdict:{action:'continue',reason:'access_enabled'}},{stage:2,check:'ip_group',elapsed_us:2,verdict:{action:'continue',reason:'ip:unlisted'}},{stage:3,check:'auth_gate',elapsed_us:3,verdict:{action:gate?'challenge':'continue',reason:gate?'auth_required':'gate_disabled'}}]}}
await context.route('**/api/**',async route=>{
  const req=route.request(),url=new URL(req.url()),endpoint=url.pathname.split('/api/v1/')[1]||'';
  if(endpoint.includes('handshake')||endpoint==='public/seckey'||endpoint==='sysinfo/checkversion')return route.fulfill({json:{code:1,msg:'Isolated UI fixture'}});
  let input={};if(req.postData()){try{input=JSON.parse(AesDecrypt(req.postData()))}catch{try{input=JSON.parse(req.postData())}catch{}}}
  const ok=data=>route.fulfill({json:{code:0,data:AesEncrypt(JSON.stringify(data)),msg:''}});
  const fail=msg=>route.fulfill({json:{code:1,msg,data:AesEncrypt(JSON.stringify({}))}});
  const p=Number(url.searchParams.get('port')||input.port||input.config?.port);
  if(endpoint==='iplocation/status')return ok({ipv4_file_size:11122714,ipv6_file_size:37283761,ipv4_builtin:true,ipv6_builtin:true});
  if(endpoint==='wafhost/ipgroup/options')return ok(clone(ipGroups));
  if(endpoint==='wafhost/ipgroup/list')return ok({list:clone(ipGroups),total:ipGroups.length});
  if(endpoint==='wafhost/ipgroup/refs'){const code=url.searchParams.get('group_code');return ok({group_code:code,hosts:[],block_count:0,allow_count:0,ports:[...configs.values()].filter(c=>(c.policy.ip_group.allow_groups||[]).includes(code)||(c.policy.ip_group.deny_groups||[]).includes(code)).map(c=>({port:c.port,name:c.name,allow:(c.policy.ip_group.allow_groups||[]).includes(code),block:(c.policy.ip_group.deny_groups||[]).includes(code)}))})}
  if(endpoint==='wafhost/ipgroupitem/list')return ok({list:[],total:0});
  if(endpoint==='wafhost/ipgroup/add'){
    if(failGroupAdd)return fail('IP 组保存失败，请重试');
    if(input.initial_ips?.some(ip=>ip==='300.1.1.1'))return fail('IP格式不正确：300.1.1.1');
    if(ipGroups.some(group=>group.group_name===input.group_name))return fail('组名称已存在');
    const group={id:'created-'+(++groupCreates),group_code:'created-'+groupCreates,group_name:input.group_name,item_count:input.initial_ips.length};ipGroups.push(group);return ok(group);
  }
  if(!endpoint.startsWith('pipeline/'))return ok({list:[],total:0});
  const action=endpoint.slice(9);
  if(action==='overview'&&failOverview)return fail('暂时无法读取服务状态');
  if(req.method()==='POST')report.writes.push({action,port:p||undefined});
  if(action==='overview')return ok({ports:[...configs.values()].map(config=>({config,stats:{today_hits:3,today_denied:1},ban_count:0,honeypot_count:0,locked:false,listener:{running:config.listen_enabled}})),issues:{},punishment:{queued:0,capacity:4096},log_queue:{queued:0,capacity:8192}});
  if(action==='ports')return ok([...configs.values()]);
  if(action==='migration/legacy')return ok([{port:5300,sources:legacySources,imported:configs.has(5300),state:configs.has(5300)?'imported':''}]);
  if(action==='migration/preview'){legacyPreviewHost=input.policy_host;return ok({port:5300,config:clone(legacyConfig),sources:legacySources,policy_host:legacyPreviewHost,preview_token:'opaque-'+legacyPreviewHost,account_names:['existing-owner'],warnings:['同端口共用选定来源策略'],blockers:[],imported:false,state:''})}
  if(action==='migration/import'){if(!input.accept_changes||input.preview_token!=='opaque-'+legacyPreviewHost)return fail('必须重新预览');const config={...clone(legacyConfig),revision:1,listen_enabled:false};configs.set(5300,config);users.set(5300,[{username:'existing-owner',enabled:true}]);return ok(config)}
  if(action==='templates')return ok(templates);
  if(action==='regions')return ok([{country:'中国',province:'重庆'},{country:'中国',province:'浙江省'},{country:'中国',province:'广东省'},{country:'United States',province:'California'}]);
  if(action==='ports/check')return configs.has(input.port)?fail('端口已有独立配置'):ok('检查通过');
  if(action==='upstream/test')return failConnection?fail('无法连接现有服务，请检查地址和端口'):ok('连接成功');
  if(action==='explain')return ok(resultFor(input.config));
  if(action==='ports/save'){
    if(p===failSavePort)return fail('模拟并发修改，原配置保留');
    const old=configs.get(p),config=clone(input.config);
    if(old&&old.revision!==config.revision)return fail('配置已更新，请刷新');
    if(config.policy.gate.enabled&&!(users.get(p)||[]).some(u=>u.enabled)&&(!input.initial_username||input.initial_password?.length<10))return fail('登录门启用前必须建立账号');
    if(config.policy.gate.require_token&&config.listen_enabled&&!tokens.has(p))return fail('开始监听前必须配置服务令牌');
    if(!old&&config.listen_enabled)return fail('新建端口不得直接监听');
    saveCalls.push(clone(config));savePayloads.push(clone(input));config.revision=(old?.revision||0)+1;configs.set(p,config);
    if(input.initial_username)users.set(p,[{username:input.initial_username,enabled:true,expires_at:0,remark:''}]);return ok(config);
  }
  if(action==='ports/copy'){const config=clone(configs.get(input.source));config.port=input.target;config.revision=0;config.listen_enabled=false;config.template={};config.policy.gate.enabled=false;if(!input.include_ips){config.policy.ip_group.entries=[];config.policy.ip_group.allow_groups=[];config.policy.ip_group.deny_groups=[];}return ok(config)}
  if(action==='ports/delete'){configs.delete(p);return ok({})}
  if(action==='gate/users')return ok(users.get(p)||[]);
  if(action==='gate/sessions'||action==='state')return ok([]);
  if(action==='logs')return ok(configs.get(p)?.policy.ip_group.block_outside_mainland?[{time_ms:Date.now(),client_ip:'2001:db8::1',kind:'request',stage:2,verdict:'deny',reason:'blacklist:outside_mainland',detail_json:JSON.stringify({req_id:'blocked-ipv6',verdict:{action:'deny',reason:'blacklist:outside_mainland'},trace:[{stage:2,check:'ip_group',verdict:{action:'deny',reason:'blacklist:outside_mainland'},elapsed_us:2}]})}]:[]);
  if(action==='stats')return ok({port:{today_hits:3,today_denied:1},punishment:{queued:0,dropped_low:0},log_queue:{queued:0}});
  if(action==='gate/users/save'){const list=users.get(p)||[];const at=list.findIndex(u=>u.username===input.username);if(at<0&&input.password.length<10)return fail('口令过短');const value={...input,totp_configured:input.totp_secret?true:input.clear_totp?false:!!list[at]?.totp_configured};delete value.password;delete value.totp_secret;if(at<0)list.push(value);else list[at]=value;users.set(p,list);return ok({})}
  if(action==='gate/users/delete'){users.set(p,(users.get(p)||[]).filter(u=>u.username!==input.username));return ok({})}
  if(action==='gate/token/rotate'){const config=configs.get(p);if(config.revision!==input.revision)return fail('配置已更新');if(input.revoke)tokens.delete(p);else tokens.add(p);config.revision++;return ok({token:input.revoke?'':'ui-fixture-token-not-production',revision:config.revision})}
  if(action==='templates/preview'){
    const tpl=templates.find(t=>t.id===input.template_id),config=clone(input.config),keys=input.sections?.length?input.sections:Object.keys(tpl.policy),diff=[];
    for(const section of keys){if(JSON.stringify(config.policy[section])!==JSON.stringify(tpl.policy[section]))diff.push({section,current:clone(config.policy[section]),template:clone(tpl.policy[section])});const ips=config.policy.ip_group.entries,proxies=config.policy.network.trusted_proxies;config.policy[section]=clone(tpl.policy[section]);config.policy.ip_group.entries=ips;config.policy.network.trusted_proxies=proxies}
    config.template={id:tpl.id,version:tpl.version};return ok({config,diff});
  }
  if(action==='templates/export'){const config=configs.get(p),policy=clone(config.policy);policy.ip_group.entries=[];policy.ip_group.allow_groups=[];policy.ip_group.deny_groups=[];policy.network.trusted_proxies=[];return ok(JSON.stringify({schema_version:1,id:'user-'+p,name:config.name||'测试模板',version:1,builtin:false,policy}))}
  if(action==='templates/import-preview'||action==='templates/save'){
    const raw=JSON.stringify(input);if(/"(?:password|password_hash|gate_key|tls_key|secrets|users)"\s*:/.test(raw))return fail('模板不允许凭据字段');
    if(!input.id||!input.name||input.schema_version!==1||!input.policy)return fail('模板结构错误');
    if(input.policy.ip_group.entries?.length||input.policy.ip_group.allow_groups?.length||input.policy.ip_group.deny_groups?.length||input.policy.network.trusted_proxies?.length)return fail('模板不能包含手工IP');
    if(action==='templates/save'){const old=templates.find(t=>t.id===input.id);if(old?.builtin)return fail('内置模板只读');templates=templates.filter(t=>t.id!==input.id);input.version=(old?.version||0)+1;templates.push(input)}return ok(input);
  }
  if(action==='templates/delete'){templates=templates.filter(t=>t.id!==input.id);return ok({})}
  if(action==='feed')return ok({config:{enabled:false,url:'',interval_minutes:60,has_token:false},status:{authorized:false,enabled:false}});
  return fail('Unexpected isolated test endpoint '+action);
});
const page=await context.newPage();page.on('pageerror',error=>report.errors.push(error.message));
async function acceptConfirm(){const box=page.locator('.el-message-box');await expect(box).toBeVisible();await box.locator('.el-message-box__btns button').last().click()}
try{
  const initialConfigs=[...configs.entries()].map(([key,value])=>[key,clone(value)]);
  configs.clear();
  await page.goto(origin+'/#/dashboard/start');
  await expect(page.getByRole('heading',{name:'先把你的服务接进来'})).toBeVisible({timeout:30000});
  await expect(page.getByRole('navigation',{name:'常用功能'}).getByRole('link')).toHaveCount(6);
  await page.getByRole('button',{name:'查看全部功能'}).click();
  await page.getByRole('textbox',{name:'搜索全部功能'}).fill('证书');
  await expect(page.locator('.nav-all-functions a').first()).toBeVisible();
  await page.getByRole('button',{name:'收起全部功能'}).click();
  for(const [width,height] of [[1440,900],[1024,768],[820,1180],[768,1024],[390,844],[320,780]]){
    await page.setViewportSize({width,height});
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(2);
    if(width===390){
      const toggle=page.locator('.sidebar-toggle');await toggle.click();
      await expect(toggle).toHaveAttribute('aria-expanded','true');
      await expect(page.getByRole('navigation',{name:'常用功能'})).toBeVisible();
      await page.locator('.workspace-nav').getByRole('link',{name:'工作台',exact:true}).focus();await page.keyboard.press('Escape');
      await expect(toggle).toHaveAttribute('aria-expanded','false');
    }
    await page.screenshot({animations:'disabled',path:path.join(artifact,`workbench-empty-${width}.png`)});
  }
  await page.setViewportSize({width:1440,height:900});
  await page.getByRole('button',{name:'接入服务',exact:true}).click();
  const guide=page.getByTestId('guided-setup');await expect(guide).toBeVisible();
  await page.screenshot({animations:'disabled',path:path.join(artifact,'guided-scenes.png')});
  await page.setViewportSize({width:390,height:844});await expect(guide.getByRole('button',{name:'继续填写地址'})).toBeInViewport();await page.screenshot({animations:'disabled',path:path.join(artifact,'guided-scenes-mobile.png')});await page.setViewportSize({width:1440,height:900});
  await page.emulateMedia({reducedMotion:'reduce'});await expect(guide.locator('.scene-option').first()).toHaveCSS('transition-duration','0s');await expect(guide.getByRole('button',{name:'继续填写地址'})).toBeVisible();await page.emulateMedia({reducedMotion:'no-preference'});
  await guide.getByRole('button',{name:'继续填写地址'}).click();await guide.getByRole('checkbox',{name:'使用独立访问账号'}).locator('xpath=ancestor::label[1]').click();await guide.getByLabel('访问账号',{exact:true}).fill('unused-private-user');await guide.getByLabel('访问密码',{exact:true}).fill('unused-private-password');await guide.getByRole('button',{name:'上一步'}).click();
  await guide.getByRole('radio',{name:/公开网站/}).check();
  await guide.getByRole('button',{name:'继续填写地址'}).click();
  await expect(guide.locator('input.el-input__inner:visible')).toHaveCount(1);
  await guide.getByLabel('现有服务地址',{exact:true}).fill('https://user:secret@nas.test');
  await guide.getByRole('button',{name:'检查连接并继续'}).click();
  await expect(guide.getByText('地址中不能包含账号密码，请只填写服务地址。')).toBeVisible();
  await guide.getByLabel('现有服务地址',{exact:true}).fill('127.0.0.1:18080');failConnection=true;
  await guide.getByRole('button',{name:'检查连接并继续'}).click();
  await expect(guide.getByText('无法连接现有服务，请检查地址和端口')).toBeVisible();
  await expect(guide.getByLabel('现有服务地址',{exact:true})).toHaveValue('http://127.0.0.1:18080');
  failConnection=false;
  for(const [width,height] of [[1440,900],[390,844],[320,780]]){
    await page.setViewportSize({width,height});
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(2);
    await page.screenshot({animations:'disabled',path:path.join(artifact,`guided-address-${width}.png`)});
  }
  await guide.getByRole('button',{name:'检查连接并继续'}).click();
  await expect(guide.getByRole('heading',{name:'确认服务配置'})).toBeVisible();
  const beforeNewSave=saveCalls.length;failSavePort=5000;
  await guide.getByRole('button',{name:'保存服务配置'}).click();
  await expect(guide.getByText('模拟并发修改，原配置保留')).toBeVisible();expect(saveCalls.length).toBe(beforeNewSave);
  failSavePort=0;
  await guide.getByRole('button',{name:'保存服务配置'}).evaluate(button=>{button.click();button.click()});
  await expect(guide).toHaveCount(0);expect(saveCalls.length).toBe(beforeNewSave+1);
  expect(configs.get(5000).listen_enabled).toBe(false);expect(configs.get(5000).policy.gate.enabled).toBe(false);expect(configs.get(5000).name).toBe('公开网站 · 127.0.0.1');
  expect(savePayloads.at(-1).initial_username).toBe('');expect(savePayloads.at(-1).initial_password).toBe('');
  await expect(page.getByRole('button',{name:'继续启用服务'})).toBeVisible();
  report.checks.push('first-use workbench, searchable complete navigation, public-site flow needs one field, inline errors, retained input and exactly-once save retry');
  await page.setViewportSize({width:1440,height:900});await page.goto(origin+'/#/waf-pipeline/ports?action=create');await expect(guide).toBeVisible();
  await guide.getByRole('button',{name:'继续填写地址'}).click();await expect(guide.getByLabel('访问账号',{exact:true})).toHaveCount(0);
  await guide.getByLabel('现有服务地址',{exact:true}).fill('127.0.0.1:18080');
  await guide.getByRole('checkbox',{name:'限制允许访问的地区'}).locator('xpath=ancestor::label[1]').click();await guide.getByRole('button',{name:'检查连接并继续'}).click();await expect(guide.getByText('请选择至少一个允许访问的地区，或取消地区限制。')).toBeVisible();
  await guide.locator('#allowed-service-regions').fill('重庆');await page.getByRole('option',{name:'中国 / 重庆',exact:true}).click();
  await guide.getByRole('checkbox',{name:'使用白名单IP组'}).locator('xpath=ancestor::label[1]').click();
  await expect(guide.getByLabel('允许的IP或网段',{exact:true})).toHaveCount(0);
  await guide.locator('#allow-service-groups').fill('家庭');await page.getByRole('option',{name:'家庭内网 · 4 条地址',exact:true}).click();await guide.getByRole('heading',{name:'连接现有服务'}).click();
  const allowPicker=guide.locator('.group-picker').first();await allowPicker.getByRole('button',{name:'新建 IP 组',exact:true}).click();
  await allowPicker.getByLabel('新IP组名称').fill('VPN 出口');await allowPicker.getByLabel('新IP组地址').fill('300.1.1.1');await allowPicker.getByRole('button',{name:'创建并选用 IP 组'}).click();await expect(allowPicker.getByText('IP格式不正确：300.1.1.1')).toBeVisible();
  await allowPicker.getByLabel('新IP组地址').fill('192.168.1.0/24\n240e:1234::/48');failGroupAdd=true;await allowPicker.getByRole('button',{name:'创建并选用 IP 组'}).click();await expect(allowPicker.getByText('IP 组保存失败，请重试')).toBeVisible();await expect(allowPicker.getByLabel('新IP组名称')).toHaveValue('VPN 出口');
  failGroupAdd=false;await allowPicker.getByRole('button',{name:'创建并选用 IP 组'}).click();await expect(allowPicker.getByText('已创建并选用“VPN 出口”。')).toBeVisible();expect(groupCreates).toBe(1);
  await guide.getByRole('checkbox',{name:'启用IP黑名单'}).locator('xpath=ancestor::label[1]').click();await guide.getByRole('checkbox',{name:'禁止中国大陆以外IP访问'}).locator('xpath=ancestor::label[1]').click();
  await guide.locator('#denied-service-regions').fill('California');await page.getByRole('option',{name:'United States / California',exact:true}).click();await guide.getByRole('heading',{name:'连接现有服务'}).click();
  await guide.locator('#deny-service-groups').fill('扫描');await page.getByRole('option',{name:'扫描来源 · 2 条地址',exact:true}).click();await guide.getByRole('heading',{name:'连接现有服务'}).click();
  await guide.getByRole('checkbox',{name:'启用IP黑名单'}).locator('xpath=ancestor::label[1]').click();await expect(guide.locator('#deny-service-groups')).toHaveCount(0);await guide.getByRole('checkbox',{name:'启用IP黑名单'}).locator('xpath=ancestor::label[1]').click();await expect(guide.getByRole('checkbox',{name:'禁止中国大陆以外IP访问'})).toBeChecked();
  await guide.getByRole('checkbox',{name:'限制允许访问的地区'}).scrollIntoViewIfNeeded();await page.screenshot({animations:'disabled',path:path.join(artifact,'optional-access-desktop.png')});
  for(const width of [1024,768,390,320]){await page.setViewportSize({width,height:844});await expect(guide.getByRole('button',{name:'检查连接并继续'})).toBeInViewport();await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-window.innerWidth)).toBeLessThanOrEqual(2)}
  await page.setViewportSize({width:390,height:844});await guide.getByRole('checkbox',{name:'启用IP黑名单'}).scrollIntoViewIfNeeded();await page.screenshot({animations:'disabled',path:path.join(artifact,'optional-access-mobile.png')});await page.setViewportSize({width:1440,height:900});
  await guide.getByRole('button',{name:'检查连接并继续'}).click();await expect(guide.getByRole('heading',{name:'确认服务配置'})).toBeVisible();await expect(guide.getByText(/拒绝中国大陆以外及未知公网来源/)).toBeVisible();
  await guide.getByRole('button',{name:'保存服务配置'}).click();await expect(guide).toHaveCount(0);
  const restricted=savePayloads.at(-1);expect(restricted.config.policy.gate.enabled).toBe(false);expect(restricted.initial_username).toBe('');expect(restricted.config.policy.ip_group.unlisted_action).toBe('deny');expect(restricted.config.policy.ip_group.regions).toEqual([{country:'中国',province:'重庆'}]);expect(restricted.config.policy.ip_group.entries).toEqual([]);expect(restricted.config.policy.ip_group.allow_groups).toEqual(['local','created-1']);expect(restricted.config.policy.ip_group.deny_groups).toEqual(['blocked']);expect(restricted.config.policy.ip_group.denied_regions).toEqual([{country:'United States',province:'California'}]);expect(restricted.config.policy.ip_group.block_outside_mainland).toBe(true);expect(restricted.config.policy.network.listen_ip_version).toBe('both');
  report.checks.push('region picker and shared IPv4/IPv6 groups, atomic create/retry, blacklist presets and saved policy agree without duplicate address fields');
  await page.goto(origin+'/#/waf-pipeline/ports?edit='+restricted.config.port);const accessEditor=page.getByTestId('port-editor');await expect(accessEditor).toBeVisible();await expect(accessEditor.getByRole('checkbox',{name:'禁止中国大陆以外IP访问'})).toBeChecked();await expect(accessEditor.getByText('VPN 出口 · 2 条地址',{exact:true})).toBeVisible();
  await accessEditor.getByRole('button',{name:'验证来源与防护'}).click();await expect(accessEditor.getByLabel('解释来源IP',{exact:true})).toBeVisible();await page.getByRole('dialog',{name:/配置服务|端口配置|服务配置/}).locator('.el-dialog__headerbtn').click();await acceptConfirm();
  report.checks.push('shared-group service link restores the saved restrictions and opens source/protection diagnostics');
  await page.goto(origin+'/#/waf-host/wafipgroup');let groupRow=page.locator('.el-table__body-wrapper tbody tr').filter({hasText:'家庭内网'});await expect(groupRow).toBeVisible();await groupRow.getByText('管理IP',{exact:true}).click();
  const groupDrawer=page.locator('.el-drawer');await expect(groupDrawer.getByText('修改后立即同步到以下服务：',{exact:false})).toBeVisible();await groupDrawer.getByRole('link',{name:new RegExp(String(restricted.config.port))}).click();await expect(page.getByTestId('port-editor')).toBeVisible();
  await page.goto(origin+'/#/waf-host/wafipgroup');groupRow=page.locator('.el-table__body-wrapper tbody tr').filter({hasText:'家庭内网'});await groupRow.getByText('删除',{exact:true}).click();const deleteGroupDialog=page.locator('.el-dialog').filter({hasText:'以下服务正在使用此 IP 组'});await expect(deleteGroupDialog.getByRole('button',{name:'确认',exact:true})).toBeDisabled();
  await page.goto(origin+'/#/waf-pipeline/ports?inspect='+restricted.config.port);await page.getByRole('tab',{name:'最近请求 / 状态变化',exact:true}).click();await expect(page.getByText('来源不属于已识别的中国大陆公网地址（blacklist:outside_mainland）',{exact:true})).toBeVisible();await page.getByRole('button',{name:'展开trace',exact:true}).click();await expect(page.getByTestId('decision-trace')).toContainText('来源不属于已识别的中国大陆公网地址');
  report.checks.push('IP group management shows linked services, navigation opens the service, referenced groups cannot be deleted, and request logs explain geographic rejection');

  await page.goto(origin+'/#/dashboard/start');
  await expect(page.getByRole('heading',{name:'还有服务等待启用'})).toBeVisible();failOverview=true;
  await page.getByRole('button',{name:'刷新状态'}).click();await expect(page.getByText(/服务状态读取失败/)).toBeVisible();
  await expect(page.getByRole('heading',{name:'还有服务等待启用'})).toBeVisible();failOverview=false;
  await page.getByRole('button',{name:'刷新状态'}).click();await expect(page.getByText(/服务状态读取失败/)).toHaveCount(0);
  await page.setViewportSize({width:1440,height:900});
  configs.clear();for(const [key,value] of initialConfigs)configs.set(key,value);saveCalls=[];savePayloads=[];
  await page.goto(origin+'/#/waf-pipeline/ports?action=create');await expect(guide).toBeVisible();
  await guide.getByRole('radio',{name:/API 与自动化/}).check();await guide.getByRole('button',{name:'继续填写地址'}).click();
  await expect(guide.getByText('此场景需要服务令牌。保存后，在服务配置中生成令牌，再启用服务。')).toBeVisible();await expect(guide.getByLabel('访问账号',{exact:true})).toHaveCount(0);
  await page.getByRole('dialog',{name:'接入服务',exact:true}).locator('.el-dialog__headerbtn').click();await acceptConfirm();
  report.checks.push('overview failure retains last result, recovery clears error, API scenario retains required token');
  const publicCard=page.locator('.service-card').filter({hasText:'公开小站（测试）'});
  await publicCard.getByRole('button',{name:'更多服务操作'}).click();await page.getByRole('menuitem',{name:'复制服务配置'}).click();
  await page.getByRole('button',{name:'生成复制草稿'}).click();await expect(guide.getByRole('radio',{name:/保留复制的配置/})).toBeChecked();
  await guide.getByRole('button',{name:'继续填写地址'}).click();await expect(guide.getByLabel('访问账号',{exact:true})).toHaveCount(0);
  await guide.getByRole('button',{name:'上一步'}).click();await guide.getByRole('radio',{name:/NAS 与家庭相册/}).check();await guide.getByRole('button',{name:'继续填写地址'}).click();await guide.getByRole('checkbox',{name:'使用独立访问账号'}).locator('xpath=ancestor::label[1]').click();await expect(guide.getByLabel('访问账号',{exact:true})).toBeVisible();
  await guide.getByRole('button',{name:'上一步'}).click();await guide.getByRole('radio',{name:/保留复制的配置/}).check();await guide.getByRole('button',{name:'继续填写地址'}).click();await expect(guide.getByLabel('访问账号',{exact:true})).toHaveCount(0);
  await page.getByRole('dialog',{name:'接入服务',exact:true}).locator('.el-dialog__headerbtn').click();await acceptConfirm();
  expect(saveCalls).toHaveLength(0);report.checks.push('copy flow preserves original policy and can restore it after trying a different scenario; mobile navigation opens and closes by keyboard');
  await page.goto(origin+'/#/waf-pipeline/ports');await expect(page.getByRole('heading',{name:'服务接入'})).toBeVisible({timeout:30000});
  await page.screenshot({animations:'disabled',path:path.join(artifact,'services-cards-light.png')});
  await page.getByRole('button',{name:'表格与批量管理',exact:true}).click();const table=page.getByTestId('ports-table');await expect(table.locator('.el-table__body-wrapper tbody tr')).toHaveCount(2);
  await page.screenshot({animations:'disabled',path:path.join(artifact,'01-overview-light.png')});report.checks.push('overview and existing port rows');
  await page.getByRole('button',{name:'接入服务',exact:true}).click();const editor=page.getByTestId('port-editor');await expect(editor).toBeVisible();
  await editor.getByRole('button',{name:'继续填写地址'}).click();
  await expect(editor.getByLabel('访问账号',{exact:true})).toHaveCount(0);await editor.getByRole('checkbox',{name:'使用独立访问账号'}).locator('xpath=ancestor::label[1]').click();
  await editor.getByRole('button',{name:'检查连接并继续'}).click();
  await expect(editor.getByText('填写现有服务地址，例如 192.168.1.20:5000。')).toBeVisible();
  await expect(editor.getByText('填写供访客使用的访问账号。')).toBeVisible();
  expect(saveCalls.length).toBe(0);
  await editor.getByLabel('现有服务地址',{exact:true}).fill('127.0.0.1:18080');
  await editor.locator('summary').filter({hasText:'连接选项'}).click();
  await editor.getByRole('spinbutton',{name:'HomeWaf 监听端口'}).fill('5110');
  await editor.getByLabel('服务名称',{exact:true}).fill('新NAS测试');
  await editor.getByLabel('访问账号',{exact:true}).fill('owner');await editor.getByLabel('访问密码',{exact:true}).fill('important-ui-password-fixture');
  await editor.getByRole('button',{name:'检查连接并继续'}).click();await expect(editor.getByText('端口检查与 TCP 连接通过')).toBeVisible();expect(saveCalls.length).toBe(0);
  await page.screenshot({animations:'disabled',path:path.join(artifact,'02-wizard-preview.png')});
  await editor.getByRole('button',{name:'保存服务配置'}).click();await expect(editor).toHaveCount(0);await expect(table.locator('.el-table__body-wrapper tbody tr')).toHaveCount(3);expect(saveCalls[0].listen_enabled).toBe(false);expect(saveCalls[0].upstream).toBe('http://127.0.0.1:18080');report.checks.push('guided setup validates inline, normalizes address, requires account and preview; creation stays disabled');
  const row=table.locator('.el-table__body-wrapper tbody tr').filter({hasText:'5110'});
  await row.getByRole('button',{name:'启用',exact:true}).click();const enableDialog=page.getByRole('dialog',{name:'启用服务'});
  await expect(enableDialog.getByRole('button',{name:'确认启用服务'})).toBeEnabled();await enableDialog.getByText('先观察运行',{exact:true}).click();await enableDialog.getByRole('button',{name:'重新检查配置'}).click();await expect(enableDialog.getByText('配置检查完成')).toBeVisible();await enableDialog.getByRole('button',{name:'确认启用服务'}).click();await expect(enableDialog).toBeHidden();
  expect(configs.get(5110).policy.gate.enabled).toBe(true);expect(configs.get(5110).policy.hit_policy.enabled).toBe(false);report.checks.push('observation enable retains login gate');
  const untouched=JSON.stringify(configs.get(5001));await row.getByRole('button',{name:'配置',exact:true}).click();await expect(editor).toBeVisible();
  await page.screenshot({animations:'disabled',path:path.join(artifact,'service-common-settings.png')});
  await expect(editor.getByRole('tab',{name:'常用设置'})).toBeVisible();await editor.getByRole('button',{name:'显示高级配置'}).click();
  await editor.getByRole('tab',{name:'3 · 规则与登录门'}).click();await expect(editor.getByText('本端口账号',{exact:false}).first()).toBeVisible();
  await editor.getByLabel('登录页Logo',{exact:true}).setInputFiles({name:'family-logo.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAEUlEQVR4nGNQSDjwH4QZYAwASvQI/dnllI8AAAAASUVORK5CYII=','base64')});await expect(editor.getByAltText('登录页Logo预览')).toBeVisible();await expect.poll(()=>editor.getByAltText('登录页Logo预览').evaluate(image=>image.naturalWidth)).toBe(2);
  await page.screenshot({animations:'disabled',path:path.join(artifact,'03-rules-gate.png')});
  await editor.getByRole('button',{name:'添加账号',exact:true}).click();const accountDialog=page.getByRole('dialog').filter({has:page.getByLabel('端口账号用户名',{exact:true})});
  await accountDialog.getByLabel('端口账号用户名',{exact:true}).fill('family');await accountDialog.getByLabel('端口账号口令',{exact:true}).fill('family-password-ui-fixture');await accountDialog.getByRole('button',{name:'立即保存账号'}).click();await expect(accountDialog).toBeHidden();expect(users.get(5110).some(u=>u.username==='family')).toBe(true);
  await editor.getByLabel('签名插件策略',{exact:true}).fill(JSON.stringify({mode:'observe',ids:['filter'],timeout_ms:20}));
  await editor.getByLabel('加密插件配置JSON',{exact:true}).fill('[]');const beforeInvalidPluginSave=saveCalls.length;
  await editor.getByRole('button',{name:'保存端口配置'}).click();await expect(editor.getByText('插件启动配置必须是JSON对象',{exact:true})).toBeVisible();expect(saveCalls.length).toBe(beforeInvalidPluginSave);
  await editor.getByLabel('加密插件配置JSON',{exact:true}).fill(JSON.stringify({filter:{binary:'fixture-plugin',params:{key:'ui-private-fixture'}}}));
  const oldRevision=configs.get(5110).revision;await editor.getByRole('button',{name:'生成 / 轮换服务令牌'}).click();await page.locator('.el-message-box').getByRole('button',{name:/确定|确认/}).click();await expect(editor.getByLabel('一次性服务令牌')).toHaveValue('ui-fixture-token-not-production');expect(configs.get(5110).revision).toBe(oldRevision+1);
  report.checks.push('account creation and service-token rotation update current draft revision');
  await editor.getByRole('tab',{name:'2 · IP 组'}).click();await editor.getByRole('button',{name:'添加条目'}).click();await editor.getByLabel('名单IP',{exact:true}).fill('192.168.20.0/24');
  await editor.getByRole('tab',{name:'5 · 业务代理'}).click();await editor.getByLabel('同端口业务路由JSON',{exact:true}).fill('{}');await expect(editor.getByRole('button',{name:'保存端口配置'})).toBeDisabled();
  await editor.getByLabel('同端口业务路由JSON',{exact:true}).fill(JSON.stringify([{hosts:['nas.test'],upstream:'http://127.0.0.1:18080'}]));await expect(editor.getByRole('button',{name:'保存端口配置'})).toBeEnabled();
  await editor.getByRole('button',{name:'保存端口配置'}).click();await expect(editor).toHaveCount(0);expect(configs.get(5110).policy.ip_group.entries[0].value).toBe('192.168.20.0/24');expect(JSON.stringify(configs.get(5001))).toBe(untouched);report.checks.push('port edit persists only selected port');
  expect(savePayloads.at(-1).plugin_profiles.filter.params.key).toBe('ui-private-fixture');expect(JSON.stringify(configs.get(5110))).not.toContain('ui-private-fixture');report.checks.push('plugin policy and separate secret submission with readable validation');
  await row.getByRole('button',{name:'存为模板'}).click();const templateDialog=page.getByRole('dialog',{name:'用户模板 / 导入预览'});await expect(templateDialog).toBeVisible();const templateBody=JSON.parse(await templateDialog.getByLabel('模板JSON',{exact:true}).inputValue());expect(templateBody.policy.ip_group.entries).toEqual([]);
  await expect(templateDialog.getByRole('button',{name:'确认保存模板'})).toBeDisabled();await templateDialog.getByRole('button',{name:'校验并预览'}).click();await expect(templateDialog.getByRole('button',{name:'确认保存模板'})).toBeEnabled();await templateDialog.getByRole('button',{name:'确认保存模板'}).click();await expect(templateDialog).toBeHidden();report.checks.push('save-as template strips manual IPs and requires validation');
  const batchBefore=JSON.stringify(configs.get(5001));await table.locator('.el-table__body-wrapper tbody tr').filter({hasText:'5000'}).locator('label.el-checkbox').click();await table.locator('.el-table__body-wrapper tbody tr').filter({hasText:'5001'}).locator('label.el-checkbox').click();await page.getByRole('button',{name:'批量应用模板',exact:true}).click();const batch=page.getByRole('dialog',{name:'批量应用模板：逐端口预览'});
  await batch.getByRole('button',{name:'为每个端口生成差异'}).click();await expect(batch.locator('summary')).toHaveCount(2);failSavePort=5001;await batch.getByRole('button',{name:'确认差异并逐端口保存'}).click();await page.locator('.el-message-box').getByRole('button',{name:/确定|确认/}).click();await expect(batch.getByText('5000成功；5001失败：模拟并发修改，原配置保留',{exact:true})).toBeVisible();expect(JSON.stringify(configs.get(5001))).toBe(batchBefore);failSavePort=0;await batch.getByRole('button',{name:'关闭',exact:true}).click();report.checks.push('batch preview and partial failure preserve failed port');
  await page.getByRole('tab',{name:'模板管理',exact:true}).click();await page.getByRole('button',{name:'导入JSON',exact:true}).click();const unsafe=clone(templateBody);unsafe.password_hash='not-allowed-secret';await templateDialog.getByLabel('模板JSON',{exact:true}).fill(JSON.stringify(unsafe));await templateDialog.getByRole('button',{name:'校验并预览'}).click();await expect(templateDialog.getByRole('button',{name:'确认保存模板'})).toBeDisabled();await templateDialog.getByRole('button',{name:'关闭',exact:true}).click();report.checks.push('credential-bearing template refused');
  await page.getByRole('button',{name:'刷新状态',exact:true}).click();await expect(page.locator('.el-message')).toHaveCount(0,{timeout:8000});
  await page.screenshot({animations:'disabled',path:path.join(artifact,'04-templates-light.png')});
  await page.getByRole('tab',{name:'我的服务',exact:true}).click();await page.emulateMedia({colorScheme:'dark'});await expect(page.locator('html')).toHaveAttribute('theme-mode','dark');await page.screenshot({animations:'disabled',path:path.join(artifact,'05-overview-dark.png')});
  const legacy=page.getByTestId('legacy-ports');await expect(legacy.getByText('nas.test、camera.test')).toBeVisible();
  await legacy.getByRole('button',{name:'预览迁入'}).click();const migration=page.getByRole('dialog',{name:'现有端口迁入预览'});
  const migrateButton=migration.getByRole('button',{name:'确认迁入为未监听'});await expect(migrateButton).toBeDisabled();
  await migration.getByRole('button',{name:'解释陌生来源请求'}).click();await expect(migration.getByTestId('decision-trace')).toBeVisible();
  await migration.locator('.el-select__wrapper').click();await page.getByRole('option',{name:'camera.test',exact:true}).click();
  await expect(migration.getByTestId('decision-trace')).toHaveCount(0);await expect(migrateButton).toBeDisabled();
  await migration.getByRole('button',{name:'解释陌生来源请求'}).click();await migration.getByRole('button',{name:'检查上游TCP连通性'}).click();
  await expect(migration.getByRole('button',{name:'检查上游TCP连通性'})).toBeEnabled();
  await migration.locator('label.el-checkbox').click();await expect(migration.getByRole('checkbox')).toBeChecked();await expect(migrateButton).toBeEnabled();await migrateButton.click();await expect(migration).toBeHidden();
  expect(configs.get(5300).listen_enabled).toBe(false);expect(configs.get(5300).routes).toHaveLength(2);expect(users.get(5300)[0].username).toBe('existing-owner');
  await expect(table.locator('.el-table__body-wrapper tbody tr').filter({hasText:'5300'})).toBeVisible();report.checks.push('legacy migration lists business, invalidates preview on source change, checks routes and copies accounts without activation');
  await page.setViewportSize({width:430,height:932});await page.screenshot({animations:'disabled',path:path.join(artifact,'06-overview-mobile.png')});
  await page.goto(origin+'/#/waf-host/wafhost');await expect(page.getByTestId('pipeline-page')).toBeVisible();await expect(page).toHaveURL(/waf-pipeline\/ports/);report.checks.push('old site entry redirects to the unified port overview');
  report.layout=await page.evaluate(()=>({width:window.innerWidth,scroll:document.documentElement.scrollWidth,elements:[...document.querySelectorAll('body *')].map(el=>({tag:el.tagName,cls:el.className,left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right,width:el.getBoundingClientRect().width})).filter(el=>el.right>window.innerWidth+2&&el.width>0).slice(0,15)}));
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-window.innerWidth)).toBeLessThanOrEqual(2);report.checks.push('dark and mobile rendering without document overflow');
  expect(report.errors).toEqual([]);fs.writeFileSync(path.join(artifact,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));
}catch(error){await page.screenshot({animations:'disabled',path:path.join(artifact,'failure.png')});console.error(JSON.stringify(report,null,2));throw error}finally{await browser.close()}
