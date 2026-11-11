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
const configs=new Map(),users=new Map(),tokens=new Set();let templates=clone(source),saveCalls=[],savePayloads=[],failSavePort=0;
for(const [port,id,name] of [[5000,'family-nas','家庭 NAS（测试）'],[5001,'public-site','公开小站（测试）']]){
  const tpl=templates.find(t=>t.id===id);configs.set(port,{schema_version:1,port,name,protocol:'http',listen_enabled:false,upstream:'http://127.0.0.1:18080',host_names:[],policy:clone(tpl.policy),template:{id:tpl.id,version:tpl.version},revision:1});users.set(port,[{username:'owner',enabled:true,expires_at:0,remark:'测试账号'}]);
}
const report={checks:[],errors:[],writes:[]};
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
  if(!endpoint.startsWith('pipeline/'))return ok({list:[],total:0});
  const action=endpoint.slice(9);
  if(req.method()==='POST')report.writes.push({action,port:p||undefined});
  if(action==='overview')return ok({ports:[...configs.values()].map(config=>({config,stats:{today_hits:3,today_denied:1},ban_count:0,honeypot_count:0,locked:false,listener:{running:config.listen_enabled}})),issues:{},punishment:{queued:0,capacity:4096},log_queue:{queued:0,capacity:8192}});
  if(action==='ports')return ok([...configs.values()]);
  if(action==='migration/legacy')return ok([{port:5300,sources:legacySources,imported:configs.has(5300),state:configs.has(5300)?'imported':''}]);
  if(action==='migration/preview'){legacyPreviewHost=input.policy_host;return ok({port:5300,config:clone(legacyConfig),sources:legacySources,policy_host:legacyPreviewHost,preview_token:'opaque-'+legacyPreviewHost,account_names:['existing-owner'],warnings:['同端口共用选定来源策略'],blockers:[],imported:false,state:''})}
  if(action==='migration/import'){if(!input.accept_changes||input.preview_token!=='opaque-'+legacyPreviewHost)return fail('必须重新预览');const config={...clone(legacyConfig),revision:1,listen_enabled:false};configs.set(5300,config);users.set(5300,[{username:'existing-owner',enabled:true}]);return ok(config)}
  if(action==='templates')return ok(templates);
  if(action==='regions')return ok([{country:'中国',province:'浙江省'},{country:'中国',province:'广东省'},{country:'United States',province:'California'}]);
  if(action==='ports/check')return configs.has(input.port)?fail('端口已有独立配置'):ok('检查通过');
  if(action==='upstream/test')return ok('连接成功');
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
  if(action==='ports/copy'){const config=clone(configs.get(input.source));config.port=input.target;config.revision=0;config.listen_enabled=false;config.template={};config.policy.gate.enabled=false;if(!input.include_ips)config.policy.ip_group.entries=[];return ok(config)}
  if(action==='ports/delete'){configs.delete(p);return ok({})}
  if(action==='gate/users')return ok(users.get(p)||[]);
  if(action==='gate/sessions'||action==='state'||action==='logs')return ok([]);
  if(action==='stats')return ok({port:{today_hits:3,today_denied:1},punishment:{queued:0,dropped_low:0},log_queue:{queued:0}});
  if(action==='gate/users/save'){const list=users.get(p)||[];const at=list.findIndex(u=>u.username===input.username);if(at<0&&input.password.length<10)return fail('口令过短');const value={...input,totp_configured:input.totp_secret?true:input.clear_totp?false:!!list[at]?.totp_configured};delete value.password;delete value.totp_secret;if(at<0)list.push(value);else list[at]=value;users.set(p,list);return ok({})}
  if(action==='gate/users/delete'){users.set(p,(users.get(p)||[]).filter(u=>u.username!==input.username));return ok({})}
  if(action==='gate/token/rotate'){const config=configs.get(p);if(config.revision!==input.revision)return fail('配置已更新');if(input.revoke)tokens.delete(p);else tokens.add(p);config.revision++;return ok({token:input.revoke?'':'ui-fixture-token-not-production',revision:config.revision})}
  if(action==='templates/preview'){
    const tpl=templates.find(t=>t.id===input.template_id),config=clone(input.config),keys=input.sections?.length?input.sections:Object.keys(tpl.policy),diff=[];
    for(const section of keys){if(JSON.stringify(config.policy[section])!==JSON.stringify(tpl.policy[section]))diff.push({section,current:clone(config.policy[section]),template:clone(tpl.policy[section])});const ips=config.policy.ip_group.entries,proxies=config.policy.network.trusted_proxies;config.policy[section]=clone(tpl.policy[section]);config.policy.ip_group.entries=ips;config.policy.network.trusted_proxies=proxies}
    config.template={id:tpl.id,version:tpl.version};return ok({config,diff});
  }
  if(action==='templates/export'){const config=configs.get(p),policy=clone(config.policy);policy.ip_group.entries=[];policy.network.trusted_proxies=[];return ok(JSON.stringify({schema_version:1,id:'user-'+p,name:config.name||'测试模板',version:1,builtin:false,policy}))}
  if(action==='templates/import-preview'||action==='templates/save'){
    const raw=JSON.stringify(input);if(/"(?:password|password_hash|gate_key|tls_key|secrets|users)"\s*:/.test(raw))return fail('模板不允许凭据字段');
    if(!input.id||!input.name||input.schema_version!==1||!input.policy)return fail('模板结构错误');
    if(input.policy.ip_group.entries?.length||input.policy.network.trusted_proxies?.length)return fail('模板不能包含手工IP');
    if(action==='templates/save'){const old=templates.find(t=>t.id===input.id);if(old?.builtin)return fail('内置模板只读');templates=templates.filter(t=>t.id!==input.id);input.version=(old?.version||0)+1;templates.push(input)}return ok(input);
  }
  if(action==='templates/delete'){templates=templates.filter(t=>t.id!==input.id);return ok({})}
  if(action==='feed')return ok({config:{enabled:false,url:'',interval_minutes:60,has_token:false},status:{authorized:false,enabled:false}});
  return fail('Unexpected isolated test endpoint '+action);
});
const page=await context.newPage();page.on('pageerror',error=>report.errors.push(error.message));
async function acceptConfirm(){const box=page.locator('.el-message-box');await expect(box).toBeVisible();await box.locator('.el-message-box__btns button').last().click()}
try{
  await page.goto(origin+'/#/waf-pipeline/ports');await expect(page.getByRole('heading',{name:'端口防护流水线'})).toBeVisible({timeout:30000});
  const table=page.getByTestId('ports-table');await expect(table.locator('.el-table__body-wrapper tbody tr')).toHaveCount(2);
  await page.screenshot({animations:'disabled',path:path.join(artifact,'01-overview-light.png')});report.checks.push('overview and existing port rows');
  await page.getByRole('button',{name:'新建端口',exact:true}).click();const editor=page.getByTestId('port-editor');await expect(editor).toBeVisible();
  await editor.locator('.pipeline-field').filter({has:page.locator('.field-label',{hasText:'监听端口'})}).locator('input').fill('5110');
  await editor.locator('.pipeline-field').filter({has:page.locator('.field-label',{hasText:'端口名称'})}).locator('input').fill('新NAS测试');
  await editor.locator('.pipeline-field').filter({has:page.locator('.field-label',{hasText:'上游地址'})}).locator('input').fill('http://127.0.0.1:18080');
  await editor.getByRole('button',{name:'检查并继续'}).click();await expect(editor.getByRole('combobox',{name:'起始模板',exact:true})).toBeVisible();
  await editor.getByRole('button',{name:'下一步',exact:true}).click();await expect(editor.getByLabel('初始账号',{exact:true})).toBeVisible();
  await editor.getByRole('button',{name:'下一步',exact:true}).click();await expect(editor.getByText('请设置账号和至少10字符的口令')).toBeVisible();expect(saveCalls.length).toBe(0);
  await editor.getByLabel('初始账号',{exact:true}).fill('owner');await editor.getByLabel('初始口令',{exact:true}).fill('important-ui-password-fixture');
  await editor.getByRole('button',{name:'下一步',exact:true}).click();await expect(editor.getByTestId('decision-trace')).toBeVisible();expect(saveCalls.length).toBe(0);
  await page.screenshot({animations:'disabled',path:path.join(artifact,'02-wizard-preview.png')});
  await editor.getByRole('button',{name:'创建为未监听'}).click();await expect(editor).toHaveCount(0);await expect(table.locator('.el-table__body-wrapper tbody tr')).toHaveCount(3);expect(saveCalls[0].listen_enabled).toBe(false);report.checks.push('wizard requires account and preview; creation stays disabled');
  const row=table.locator('.el-table__body-wrapper tbody tr').filter({hasText:'5110'});
  await row.getByRole('button',{name:'启用',exact:true}).click();const enableDialog=page.getByRole('dialog',{name:'启用前解释'});
  await expect(enableDialog.getByRole('button',{name:'确认启用监听'})).toBeDisabled();await enableDialog.getByText('观察启动',{exact:true}).click();await enableDialog.getByRole('button',{name:'解释选定启动方式'}).click();await expect(enableDialog.getByTestId('decision-trace')).toBeVisible();await enableDialog.getByRole('button',{name:'确认启用监听'}).click();await expect(enableDialog).toBeHidden();
  expect(configs.get(5110).policy.gate.enabled).toBe(true);expect(configs.get(5110).policy.hit_policy.enabled).toBe(false);report.checks.push('observation enable retains login gate');
  const untouched=JSON.stringify(configs.get(5001));await row.getByRole('button',{name:'配置',exact:true}).click();await expect(editor).toBeVisible();
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
  await page.getByRole('button',{name:'刷新',exact:true}).click();await expect(page.locator('.el-message')).toHaveCount(0,{timeout:8000});
  await page.screenshot({animations:'disabled',path:path.join(artifact,'04-templates-light.png')});
  await page.getByRole('tab',{name:'端口总览',exact:true}).click();await page.locator('html').evaluate(el=>el.classList.add('dark'));await page.screenshot({animations:'disabled',path:path.join(artifact,'05-overview-dark.png')});
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
