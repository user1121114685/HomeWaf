import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { handleTransportHandshake, routeSession } from './transport-fixture.mjs';
const root=fileURLToPath(new URL('../..',import.meta.url));
const origin=process.env.ASSESSMENT_TEST_ORIGIN || 'http://127.0.0.1:3119';
const output=path.join(root,'artifacts','assessment-browser');fs.mkdirSync(output,{recursive:true});
const templates=JSON.parse(execFileSync('go',['run','./scripts/pipeline-fixtures'],{cwd:root,encoding:'utf8'}));
const config={schema_version:1,port:15000,name:'测试服务',protocol:'http',listen_enabled:true,upstream:'http://127.0.0.1:18080',host_names:['selected.example.test'],policy:templates.find(x=>x.id==='public-site').policy,template:{},revision:7};
const camo={profiles:[{id:'http',name:'测试 HTTP 画像',port:18081,protocol_profile:'http',version:'nginx/1.24.0',server_name:'nas',listen_ip_version:'dual',enabled:true}],local_cidr_mode:'custom',local_cidrs:['192.168.0.0/16']};
let fail=false,delay=false,deny=false,lastInput,serviceEmpty=false,cancelCount=0,polls=0;
const plan={ipv4:5,ipv6:5,pressure:600,warnings:[],cities:[{province:'重庆市',city:'重庆',ipv4:5,ipv6:5,ipv4_target:5,ipv6_target:5,ipv4_carriers:['移动','联通','电信','广电'],ipv6_carriers:['移动','联通','电信']},{province:'云南省',city:'保山',ipv4:5,ipv6:0,ipv4_target:5,ipv6_target:5,ipv4_carriers:['移动'],ipv6_carriers:[]}],sources:[{ip:'1.2.3.47',family:'IPv4',country:'中国',province:'重庆市',city:'重庆',isp:'移动',expect_access:true},{ip:'240e:1234:5678:9abc::789a',family:'IPv6',country:'中国',province:'重庆市',city:'重庆',isp:'电信',expect_access:true}],samples:[{id:'normal',kind:'benign',category:'normal',method:'GET',path:'/',headers:{},body:''}]};
function result(){return {plan,assessment:{target:config.host_names.length?'http://selected.example.test:15000':'http://127.0.0.1:15000',revision:7,score:76,direct_score:73,camouflage_score:10,evaluations:52,normal_passed:2,normal_total:2,elapsed_ms:12.1,pressure:{evaluations:600,limited:400,first_limited:101,elapsed_ms:2.2,lanes:[{name:'IPv4 单来源连续访问',evaluations:200,limited:100,first_limited:101}]},categories:[{category:'sqli',total:2,blocked:2,challenged:0,observed:0,missed:0}],advice:['允许地区内仍需检查攻击规则。'],limits:['隔离模拟，不连接源站。'],rows:[{id:'sql-query',category:'sqli',kind:'attack',client_ip:'1.2.3.47',expected:'block',outcome:'blocked',geo:{ok:true,country:'中国',province:'重庆市'},result:{req_id:'fixture',verdict:{action:'deny',reason:'sqli'},trace:[{stage:3,check:'sqli',verdict:{action:'deny',reason:'sqli'},elapsed_us:1}]}}]},camouflage:[{...camo.profiles[0],running:true,predicted:10,local:0}],listeners:[{family:'IPv4',expected:true,connected:true,detail:'本机 TCP 连接成功'},{family:'IPv6',expected:false,connected:true,detail:'本机 TCP 连接成功'}]};}

const report={checks:[],layouts:[],errors:[]};
const browser=await chromium.launch({channel:'chrome',headless:true});
const context=await browser.newContext({viewport:{width:1440,height:1000}});
await context.addInitScript(()=>{localStorage.setItem('access_token','isolated-assessment');localStorage.setItem('lang','zh_CN')});
await context.routeWebSocket('**/api/**',ws=>ws.close());
await context.route('**/api/**',async route=>{
  if(await handleTransportHandshake(route))return;
  const req=route.request(),endpoint=new URL(req.url()).pathname.split('/api/v1/')[1]||'';
  const ok=data=>route.fulfill({json:{code:0,data:routeSession(route).encrypt(JSON.stringify(data)),msg:''}});
  const error=msg=>route.fulfill({json:{code:1,msg}});
  if(endpoint==='pipeline/camouflage/config')return ok({config:camo,status:{profiles:[{id:'http',running:true}],local_cidrs:camo.local_cidrs}});
  if(endpoint==='pipeline/camouflage/profiles')return ok([{code:'http',name:'HTTP',group:'常用协议',transport:'tcp',default_version:'nginx/1.24.0',default_port:80}]);
  if(endpoint==='pipeline/camouflage/events')return ok([]);
  if(endpoint==='pipeline/ports')return deny?error('没有检测权限'):ok(serviceEmpty?[]:[config]);
  if(endpoint==='pipeline/assessment/sources')return ok(plan);
  if(endpoint==='pipeline/assessment'){
    lastInput=JSON.parse(routeSession(route).decrypt(req.postData()));polls=0;
    if(fail)return error('检测失败：请重试，草稿保留');
    return ok({job:{id:'fixture',state:'preparing',done:0,total:0}});
  }
  if(endpoint==='pipeline/assessment/jobs')return ok(delay||polls++===0?{state:'running',done:1,total:2}:{state:'complete',done:2,total:2,result:result()});
  if(endpoint==='pipeline/assessment/cancel'){cancelCount++;return ok({});}
  if(endpoint==='pipeline/overview')return ok({ports:[{config,stats:{},listener:{running:true}}],issues:{},punishment:{},log_queue:{}});
  if(endpoint==='pipeline/templates')return ok(templates);
  if(endpoint==='pipeline/regions')return ok([{country:'中国',province:'重庆市'}]);
  if(['pipeline/gate/users','pipeline/gate/sessions','pipeline/state','pipeline/cc/hits'].includes(endpoint))return ok([]);
  if(endpoint==='pipeline/defense/capabilities')return ok({});
  if(endpoint==='wafhost/ipgroup/options')return ok([]);
  if(endpoint==='iplocation/status')return ok({ipv4_file_size:100});
  return ok({list:[],total:0});
});
const page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));
const panel=()=>page.getByTestId('score-detection').filter({visible:true});
async function open(){await page.goto(`${origin}/#/waf-pipeline/protocol-defense?tab=scores`);await expect(panel().getByRole('heading',{name:'检查服务防御能力'})).toBeVisible();}
async function run(){await panel().getByRole('button',{name:/^(开始检测|重新检测)$/}).click();await expect(panel().locator('.score-result')).toBeVisible();await expect(panel().getByRole('button',{name:'重新检测',exact:true})).not.toHaveClass(/is-loading/);}
try{
  await open();
  await expect(panel()).toContainText('http://selected.example.test:15000');
  await expect(panel().locator('input[type=file]')).toHaveCount(0);
  await expect(panel().getByRole('button',{name:'从当前 IP 数据库取样'})).toHaveCount(0);
  await run();expect(lastInput.automatic).toBe(true);expect(lastInput.port).toBe(15000);
  await expect(panel()).toContainText('1 个库内城市存在样本缺口');await expect(panel()).toContainText('发现意外监听');await expect(panel()).toContainText('伪装画像 · 10 / 10 分');
  report.checks.push('one-click automatic plan, progress, actual count, coverage gap and unexpected listener');
  fail=true;await panel().getByRole('button',{name:'重新检测',exact:true}).click();await expect(panel()).toContainText('检测失败：请重试');
  fail=false;await run();delay=true;await panel().getByRole('button',{name:'重新检测',exact:true}).click();await expect(panel()).toContainText('已检查 1 / 2');await panel().getByRole('button',{name:'取消检测',exact:true}).click();await expect(panel()).toContainText('检测已取消');expect(cancelCount).toBeGreaterThan(0);delay=false;
  report.checks.push('failure/retry and server job cancellation');
  const popupPromise=context.waitForEvent('page');await panel().getByRole('link',{name:'自定义测试 ↗'}).click();const advanced=await popupPromise;await advanced.waitForLoadState();
  const custom=advanced.getByTestId('score-detection');await expect(custom.getByLabel('来源 IP 1',{exact:true})).toHaveValue('1.2.3.47');
  await expect(custom.getByLabel('重复请求次数')).toHaveValue('600');
  await custom.locator('input[type=file]').setInputFiles({name:'incident.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({detail_records:[{host:'private.invalid',url:'https://private.invalid/test',method:'GET',header:'Host: private.invalid\r\nCookie: SECRET\r\n',action:'阻止'}]}))});
  await expect(custom).toContainText('已导入 1 条');await custom.getByRole('button',{name:'开始评分检测',exact:true}).click();await expect(custom.locator('.assessment-summary')).toBeVisible();
  expect(JSON.stringify(lastInput)).not.toMatch(/private\.invalid|SECRET/);expect(lastInput.samples[0].kind).toBe('unknown');
  await custom.getByLabel('来源 IP 1',{exact:true}).fill('1.2.3.48');await expect(custom).toContainText('输入或草稿已变化');
  await custom.getByText('单条判定',{exact:true}).click();await custom.getByLabel('请求路径',{exact:true}).fill('/api/test');await custom.getByRole('button',{name:'检查当前判定',exact:true}).click();await expect(custom.getByRole('button',{name:'检查当前判定',exact:true})).not.toHaveClass(/is-loading/);expect(lastInput.samples[0].path).toBe('/api/test');
  report.checks.push('developer window inherits plan/service; imported identity redaction; single request and stale state');
  await custom.getByText('完整测试参数（仅本次，不保存服务）',{exact:true}).click();await custom.getByRole('button',{name:'载入当前策略以编辑'}).click();await expect(custom.getByLabel('测试策略 JSON',{exact:true})).toHaveValue(/selected.example.test/);
  for(const target of [page,advanced])for(const theme of ['light','dark'])for(const width of [1440,820,390]){
    await target.setViewportSize({width,height:1000});await target.evaluate(value=>{document.documentElement.classList.toggle('dark',value==='dark');document.documentElement.setAttribute('theme-mode',value);},theme);
    const overflow=await target.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1);expect(overflow).toBe(false);
    const kind=target===page?'automatic':'advanced';await target.screenshot({path:path.join(output,`${kind}-${theme}-${width}.png`),fullPage:true});report.layouts.push({kind,theme,width,overflow});
  }
  await advanced.close();await page.setViewportSize({width:1440,height:1100});
  const createPromise=context.waitForEvent('page');await panel().getByRole('link',{name:'添加伪装画像 ↗'}).click();const createPage=await createPromise;await expect(createPage.getByRole('dialog',{name:'添加伪装画像'})).toBeVisible();await createPage.close();
  await page.goto(`${origin}/#/waf-pipeline/ports`);await page.getByRole('button',{name:'配置服务',exact:true}).first().click();const editor=page.getByTestId('port-editor');await expect(editor).toBeVisible();
  await editor.getByRole('button',{name:'评分检测',exact:true}).click();await expect(panel()).toContainText('检测当前草稿');await run();expect(lastInput.config.revision).toBe(7);
  const draftPromise=context.waitForEvent('page');await panel().getByRole('link',{name:'自定义测试 ↗'}).click();const draftPage=await draftPromise;await expect(draftPage.getByTestId('score-detection')).toContainText('当前未保存草稿');await draftPage.close();
  const persistent=await page.evaluate(()=>JSON.stringify({...localStorage,...sessionStorage}));expect(persistent).not.toMatch(/selected.example.test|1\.2\.3\.47/);
  await page.getByRole('button',{name:'返回服务草稿',exact:true}).click();await expect(editor).toBeVisible();
  report.checks.push('add-profile deep link, unsaved draft handoff/return, no draft/sample persistence');
  await editor.getByRole('button',{name:'关闭',exact:true}).click();
  await page.goto(`${origin}/#/waf-pipeline/protocol-defense?tab=scores`);await page.reload();await expect(panel()).toBeVisible();serviceEmpty=true;await page.reload();await expect(panel()).toContainText('先添加要保护的服务');
  serviceEmpty=false;deny=true;await page.reload();await expect(panel()).toContainText('没有检测权限');deny=false;await panel().getByRole('button',{name:'重新读取服务'}).click();await expect(panel()).toContainText('selected.example.test');
  config.host_names=[];await page.reload();await expect(panel()).toContainText('http://127.0.0.1:15000');
  report.checks.push('empty services, forbidden read, recovery, IP fallback');expect(report.errors).toEqual([]);
}finally{fs.writeFileSync(path.join(output,'report.json'),JSON.stringify(report,null,2));await browser.close();}
console.log(JSON.stringify(report));
