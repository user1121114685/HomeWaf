// Exercise repeated edits against the real Vue app with isolated, in-memory
// management responses. No request from this browser reaches a live API.
import { chromium, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const origin = process.env.CONFIG_EDIT_TEST_ORIGIN || 'http://127.0.0.1:3001';
const output = path.join(root, 'artifacts', 'config-editability');
fs.mkdirSync(output, { recursive: true });
const clone = value => JSON.parse(JSON.stringify(value));
let templates = JSON.parse(execFileSync('go', ['run', './scripts/pipeline-fixtures'], { cwd: root, encoding: 'utf8', env: { ...process.env, CGO_ENABLED: '0' } }));
templates.push({ ...clone(templates[0]), id: 'user-edit', name: '可修改模板', builtin: false, version: 1 });
const configs = new Map(), users = new Map();
for (const port of [5000, 5001]) {
  const template = templates.find(template => template.id === (port === 5000 ? 'family-nas' : 'public-site'));
  configs.set(port, { schema_version: 1, port, name: `测试服务${port}`, protocol: 'http', listen_enabled: port === 5000, upstream: 'http://127.0.0.1:18080', host_names: [], policy: clone(template.policy), revision: 1, template: { id: template.id, version: 1 } });
  users.set(port, [{ username: 'owner', enabled: true, expires_at: 0, remark: '初始备注' }]);
}
const otherPort = JSON.stringify(configs.get(5001));
let failNextSave = false;
const report = { checks: [], errors: [], writes: [], origin };
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1050 }, colorScheme: 'light' });
await context.addInitScript(() => { localStorage.setItem('access_token', 'isolated-config-edit-test'); localStorage.setItem('lang', 'zh_CN'); });
await context.routeWebSocket('**/api/**', ws => ws.close());
await context.route('**/api/**', async route => {
  const request = route.request(), url = new URL(request.url()), endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = data => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = msg => route.fulfill({ json: { code: 1, msg, data: AesEncrypt('{}') } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint)) return fail('Isolated config-edit fixture');
  let input = {};
  if (request.postData()) { try { input = JSON.parse(AesDecrypt(request.postData())); } catch { try { input = JSON.parse(request.postData()); } catch {} } }
  const port = Number(url.searchParams.get('port') || input.port || input.config?.port);
  if (endpoint === 'iplocation/status') return ok({ ipv4_file_size: 11122714, ipv6_file_size: 37283761, ipv4_builtin: true, ipv6_builtin: true });
  if (endpoint === 'wafhost/ipgroup/options') return ok([]);
  if (!endpoint.startsWith('pipeline/')) return ok({ list: [], total: 0 });
  const action = endpoint.slice(9);
  if (request.method() === 'POST') report.writes.push({ action, port: port || undefined });
  if (action === 'overview') return ok({ ports: [...configs.values()].map(config => ({ config: clone(config), stats: {}, listener: { running: config.listen_enabled } })), issues: {} });
  if (action === 'templates') return ok(clone(templates));
  if (action === 'migration/legacy') return ok([]);
  if (action === 'regions') return ok([{ country: '中国', province: '浙江省' }]);
  if (action === 'gate/users') return ok(clone(users.get(port) || []));
  if (['gate/sessions', 'state', 'logs'].includes(action)) return ok([]);
  if (action === 'stats') return ok({ port: {}, punishment: {}, log_queue: {} });
  if (action === 'gate/users/save') {
    const list = clone(users.get(port)), existing = list.find(user => user.username === input.username);
    if (!existing) return fail('This fixture edits existing accounts only');
    const { password, totp_secret, ...values } = input;
    Object.assign(existing, values);
    users.set(port, list);
    return ok({});
  }
  if (action === 'ports/save') {
    if (failNextSave) { failNextSave = false; return fail('模拟保存失败，草稿应保留'); }
    const config = clone(input.config), old = configs.get(config.port);
    if (!old || old.revision !== config.revision) return fail('配置版本冲突');
    config.revision++;
    configs.set(config.port, config);
    return ok(clone(config));
  }
  if (action === 'templates/import-preview') return ok(input);
  if (action === 'templates/save') {
    const old = templates.find(template => template.id === input.id);
    const updated = { ...clone(input), version: (old?.version || 0) + 1 };
    templates = [...templates.filter(template => template.id !== input.id), updated];
    return ok(updated);
  }
  return fail('Unexpected isolated endpoint: ' + action);
});
const page = await context.newPage();
page.on('pageerror', error => report.errors.push(error.message));
const cards = () => page.getByTestId('service-cards');
const card = () => cards().locator('article').filter({ hasText: /测试服务5000|持续修改的服务/ });
const editor = () => page.getByTestId('port-editor');
async function openCommon() { await card().getByRole('button', { name: '配置服务', exact: true }).click(); await expect(editor()).toBeVisible(); }
async function savePort() { await editor().getByRole('button', { name: '保存端口配置', exact: true }).click(); await expect(editor()).toHaveCount(0); }
async function cancelPort() {
  await editor().getByRole('button', { name: '关闭', exact: true }).click();
  const box = page.locator('.el-message-box');
  await expect(box).toBeVisible();
  await box.locator('.el-message-box__btns button').last().click();
  await expect(editor()).toHaveCount(0);
}
try {
  await page.goto(origin + '/#/waf-pipeline/ports');
  await expect(card()).toBeVisible({ timeout: 30000 });
  await card().getByRole('button', { name: '蜜罐设置', exact: true }).click();
  await expect(editor().getByRole('tab', { name: '蜜罐设置', exact: true })).toHaveAttribute('aria-selected', 'true');
  await expect(editor().getByRole('button', { name: '显示高级配置', exact: true })).toBeVisible();
  const honey = editor().getByTestId('honeypot-settings');
  const setSwitch = async name => { const control = honey.getByRole('switch', { name, exact: true }); if (await control.getAttribute('aria-checked') !== 'true') await control.locator('xpath=..').click(); };
  await setSwitch('启用内置蜜罐');
  await honey.getByRole('button', { name: '添加蜜罐提示', exact: true }).click();
  await honey.getByLabel('蜜罐提示1标题', { exact: true }).fill('持续可修改的提示');
  await honey.getByLabel('蜜罐提示1内容', { exact: true }).fill('第一版内容');
  await honey.getByRole('spinbutton', { name: '蜜罐提示1权重', exact: true }).fill('70');
  await honey.getByRole('spinbutton', { name: '单来源命中多少次后丢弃', exact: true }).fill('5');
  await setSwitch('多来源触发本端口丢弃');
  await setSwitch('归档脱敏后的提交内容');
  await savePort();
  const lureID = configs.get(5000).policy.honeypot.lures[0].id;
  expect(configs.get(5000).listen_enabled).toBe(true);
  expect(configs.get(5000).policy.honeypot.lures[0].weight).toBe(70);
  expect(configs.get(5000).policy.honeypot.drop_after_hits).toBe(5);
  report.checks.push('running service exposes a permanent honeypot entry and saves without recreation or stopping');

  await page.reload();
  await card().getByRole('button', { name: '蜜罐设置', exact: true }).click();
  await expect(honey.getByLabel('蜜罐提示1标题', { exact: true })).toHaveValue('持续可修改的提示');
  await expect(honey.getByRole('spinbutton', { name: '蜜罐提示1权重', exact: true })).toHaveValue('70');
  await honey.getByLabel('蜜罐提示1内容', { exact: true }).fill('第二版内容');
  await honey.getByRole('button', { name: '添加蜜罐提示', exact: true }).click();
  await honey.getByLabel('蜜罐提示2标题', { exact: true }).fill('可删除的提示');
  await honey.getByLabel('蜜罐提示1标题', { exact: true }).fill('');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await honey.getByLabel('蜜罐提示1标题', { exact: true }).fill('持续可修改的提示');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeEnabled();
  failNextSave = true;
  await editor().getByRole('button', { name: '保存端口配置' }).click();
  await expect(editor().getByText('模拟保存失败，草稿应保留', { exact: true })).toBeVisible();
  await expect(honey.getByLabel('蜜罐提示1内容', { exact: true })).toHaveValue('第二版内容');
  expect(configs.get(5000).policy.honeypot.lures[0].message).toBe('第一版内容');
  await savePort();
  expect(configs.get(5000).policy.honeypot.lures[0].id).toBe(lureID);
  await card().getByRole('button', { name: '蜜罐设置', exact: true }).click();
  await honey.getByRole('button', { name: '移除蜜罐提示2', exact: true }).click();
  await savePort();
  expect(configs.get(5000).policy.honeypot.lures).toHaveLength(1);
  report.checks.push('honeypot reopens after refresh, edits/adds/removes prompts, validates values and preserves failed-save drafts');

  await openCommon();
  await editor().getByRole('button', { name: '配置蜜罐', exact: true }).click();
  await expect(editor().getByRole('tab', { name: '蜜罐设置', exact: true })).toHaveAttribute('aria-selected', 'true');
  await honey.getByLabel('蜜罐提示1内容', { exact: true }).fill('取消的内容');
  await cancelPort();
  expect(configs.get(5000).policy.honeypot.lures[0].message).toBe('第二版内容');
  report.checks.push('common settings links directly to honeypot and cancel leaves saved settings unchanged');

  await openCommon();
  await editor().getByLabel('服务名称', { exact: true }).fill('持续修改的服务');
  await editor().getByLabel('现有服务地址', { exact: true }).fill('http://127.0.0.1:18081');
  await savePort();
  await openCommon();
  await expect(editor().getByLabel('现有服务地址', { exact: true })).toHaveValue('http://127.0.0.1:18081');
  await editor().getByRole('button', { name: '设置连接参数', exact: true }).click();
  const routes = editor().getByLabel('同端口业务路由JSON', { exact: true });
  await routes.fill('[unfinished');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await editor().getByRole('button', { name: '收起高级配置', exact: true }).click();
  await editor().getByRole('button', { name: '定位未完成的配置', exact: true }).click();
  await expect(routes).toHaveValue('[unfinished');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await routes.fill('[]');
  await editor().getByRole('spinbutton', { name: '代理最大跳数', exact: true }).fill('12');
  await savePort();
  expect(configs.get(5000).policy.network.max_proxy_hops).toBe(12);
  expect(configs.get(5000).policy.honeypot.lures[0].message).toBe('第二版内容');
  report.checks.push('connection settings reopen and invalid proxy JSON survives advanced-panel unmount, can be located, corrected and saved');

  await openCommon();
  await editor().getByRole('button', { name: '管理访问账号', exact: true }).click();
  await editor().getByLabel('登录页标题', { exact: true }).fill('已修改的登录门');
  await editor().getByRole('button', { name: '修改 / 重设口令', exact: true }).click();
  const account = page.getByRole('dialog', { name: '端口账号', exact: true });
  await account.locator('label').filter({ hasText: '备注' }).locator('input').fill('账号可持续修改');
  await account.getByRole('button', { name: '立即保存账号', exact: true }).click();
  await expect(account).toBeHidden();
  await savePort();
  expect(users.get(5000)[0].remark).toBe('账号可持续修改');
  expect(configs.get(5000).policy.gate.title).toBe('已修改的登录门');
  report.checks.push('existing login account and gate settings update independently and remain saved');

  await openCommon();
  await editor().getByRole('button', { name: '调整防护规则', exact: true }).click();
  await editor().getByRole('spinbutton', { name: '每 IP 每秒请求', exact: true }).fill('45');
  const scripts = editor().getByLabel('自定义GRL规则JSON', { exact: true });
  await scripts.fill('{}');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await editor().getByRole('button', { name: '收起高级配置', exact: true }).click();
  await editor().getByRole('button', { name: '定位未完成的配置', exact: true }).click();
  await expect(scripts).toHaveValue('{}');
  await scripts.fill('[]');
  await editor().getByRole('tab', { name: '2 · IP 组', exact: true }).click();
  await editor().getByRole('button', { name: '添加条目', exact: true }).click();
  await editor().getByLabel('名单IP', { exact: true }).fill('192.0.2.20');
  await savePort();
  expect(configs.get(5000).policy.ruleset.qps.per_ip).toBe(45);
  expect(configs.get(5000).policy.ip_group.entries.at(-1).value).toBe('192.0.2.20');
  report.checks.push('rules and IP entries support repeat edits and retain unrelated policy sections');

  await page.getByRole('button', { name: '表格与批量管理', exact: true }).click();
  const row = page.getByTestId('ports-table').locator('.el-table__body-wrapper tbody tr').filter({ hasText: '5000' });
  await row.getByRole('button', { name: '蜜罐设置', exact: true }).click();
  await expect(editor().getByRole('tab', { name: '蜜罐设置', exact: true })).toHaveAttribute('aria-selected', 'true');
  await cancelPort();
  expect(JSON.stringify(configs.get(5001))).toBe(otherPort);
  report.checks.push('table and card entries both edit honeypot without changing another service');

  await page.getByRole('tab', { name: '模板管理', exact: true }).click();
  const templateCard = page.locator('.el-card').filter({ hasText: '可修改模板' });
  await templateCard.getByRole('button', { name: '编辑 / 重命名', exact: true }).click();
  const templateDialog = page.getByRole('dialog', { name: '用户模板 / 导入预览', exact: true });
  await templateDialog.getByLabel('模板名称', { exact: true }).fill('持续修改的模板');
  await templateDialog.getByRole('button', { name: '校验并预览', exact: true }).click();
  await templateDialog.getByRole('button', { name: '确认保存模板', exact: true }).click();
  await expect(templateDialog).toBeHidden();
  await page.reload();
  await page.locator('.el-card').filter({ hasText: '持续修改的模板' }).getByRole('button', { name: '编辑 / 重命名', exact: true }).click();
  await expect(templateDialog.getByLabel('模板名称', { exact: true })).toHaveValue('持续修改的模板');
  report.checks.push('user templates can be renamed, validated, saved and reopened after refresh');
  await templateDialog.getByRole('button', { name: '关闭', exact: true }).click();

  await page.getByRole('tab', { name: '我的服务', exact: true }).click();
  const cardViewButton = page.getByRole('button', { name: '卡片视图', exact: true });
  if (await cardViewButton.count()) await cardViewButton.click();
  await page.setViewportSize({ width: 390, height: 844 });
  await card().getByRole('button', { name: '蜜罐设置', exact: true }).click();
  await expect(honey.getByLabel('蜜罐提示1内容', { exact: true })).toHaveValue('第二版内容');
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(2);
  await page.screenshot({ path: path.join(output, 'honeypot-mobile.png'), animations: 'disabled' });
  await page.setViewportSize({ width: 1440, height: 1050 });
  await page.screenshot({ path: path.join(output, 'honeypot-desktop.png'), animations: 'disabled' });
  report.checks.push('mobile and desktop retain accessible honeypot edits without document overflow');
  expect(report.errors).toEqual([]);
  fs.writeFileSync(path.join(output, 'browser-report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), animations: 'disabled' });
  console.error(JSON.stringify(report, null, 2));
  throw error;
} finally { await browser.close(); }
