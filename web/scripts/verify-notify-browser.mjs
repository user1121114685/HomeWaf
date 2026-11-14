// Real Vue forms against isolated in-memory responses. No notification or
// configuration request from this browser reaches a live management API.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const dist = path.join(root, 'web', 'dist');
const output = path.join(root, 'artifacts', 'notify-verification');
fs.mkdirSync(output, { recursive: true });
const catalogue = JSON.parse(fs.readFileSync(path.join(root, 'wafnotify/channels/catalog.json'), 'utf8'));
const mime = { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.json': 'application/json' };
const server = http.createServer((req, res) => {
  const requested = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
  const file = path.resolve(dist, '.' + (requested === '/' ? '/index.html' : requested));
  if (!file.startsWith(dist + path.sep) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.writeHead(404); res.end(); return; }
  res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
await context.addInitScript(() => { localStorage.setItem('access_token', 'isolated-notification-test'); localStorage.setItem('lang', 'zh_CN'); });
await context.routeWebSocket('**/api/**', socket => socket.close());
const rows = [], writes = [], errors = [];
const mask = row => {
  const result = structuredClone(row), config = JSON.parse(result.config_json || '{}');
  result.has_config_secrets = {};
  for (const field of catalogue.find(item => item.type === row.type).fields) if (field.secret) { result.has_config_secrets[field.key] = !!config[field.key]; delete config[field.key]; }
  result.config_json = JSON.stringify(config);
  return result;
};
await context.route('**/api/**', async route => {
  const request = route.request(), endpoint = new URL(request.url()).pathname.split('/api/v1/')[1] || '';
  const ok = data => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = msg => route.fulfill({ json: { code: -1, data: AesEncrypt('{}'), msg } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint)) return fail('Isolated notification fixture');
  let input = {};
  if (request.postData()) { try { input = JSON.parse(AesDecrypt(request.postData())); } catch { try { input = JSON.parse(request.postData()); } catch {} } }
  if (endpoint === 'notify/channel/types') return ok(catalogue);
  if (endpoint === 'notify/channel/list') return ok({ list: rows.filter(row => !input.type || row.type === input.type).map(mask), total: rows.length });
  if (endpoint === 'notify/channel/add') { rows.push({ ...input, id: `channel-${rows.length}`, create_time: '2026-11-14 10:00:00' }); writes.push({ action: 'add', type: input.type }); return ok({}); }
  if (endpoint === 'notify/channel/edit') {
    const old = rows.find(row => row.id === input.id), submitted = JSON.parse(input.config_json), current = JSON.parse(old.config_json);
    for (const field of catalogue.find(item => item.type === input.type).fields) if (field.secret) {
      if (submitted[field.key] === '__SAMWAF_CLEAR__') submitted[field.key] = '';
      else if (!submitted[field.key] && old.type === input.type) submitted[field.key] = current[field.key];
    }
    Object.assign(old, input, { config_json: JSON.stringify(submitted) });
    writes.push({ action: 'edit', type: input.type }); return ok({});
  }
  if (endpoint === 'notify/channel/test') { writes.push({ action: 'test' }); return ok({}); }
  if (endpoint === 'notify/log/list') return ok({ list: [{ id: 'log-1', channel_name: '测试 bark', channel_type: 'bark', message_type: 'system_error', message_title: '系统错误', status: 1 }], total: 1 });
  if (endpoint.startsWith('notify/') && request.method() !== 'GET' && !endpoint.endsWith('/list')) return fail('Unexpected notification mutation: ' + endpoint);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', error => errors.push(error.message));
const dialog = () => page.locator('.el-dialog:visible');
const choose = async (select, name) => { await select.click(); await page.getByRole('option', { name, exact: true }).click(); };
try {
  await page.goto(origin + '/#/notify/channel');
  await expect(page.getByRole('button', { name: '添加渠道', exact: true })).toBeVisible();
  const samples = { appKey: 'app-key', appSecret: 'app-secret', userIds: 'user-1', corpID: 'corp-id', agentID: '1001', secret: 'corp-secret', appId: 'app-id', receiveId: 'receiver', server: 'https://api.day.app/fake-device-key', apiToken: '123:bot-token', chatId: '-1001', token: 'push-token', webServer: 'https://gotify.example.com/message?token=fake-token', address: 'broker.example.com', outMessageTopic: 'homewaf/alerts' };
  for (const type of ['bark', 'telegram', 'pushplus', 'gotify', 'dingtalk_app', 'wechatwork_app', 'feishu_app', 'socket']) {
    const definition = catalogue.find(item => item.type === type);
    await page.getByRole('button', { name: '添加渠道', exact: true }).click();
    await dialog().locator('input').first().fill(`测试 ${type}`);
    await choose(dialog().locator('.el-select').first(), definition.name);
    for (const field of definition.fields) {
      if (field.required && !field.default && samples[field.key]) await dialog().getByTestId(`provider-field-${field.key}`).locator('input').fill(samples[field.key]);
    }
    if (type === 'bark') {
      await dialog().getByTestId('provider-field-group').locator('input').fill('HomeWaf');
      await dialog().screenshot({ path: path.join(output, 'bark-create.png') });
    }
    await dialog().getByRole('button', { name: '确认', exact: true }).click();
    await expect(dialog()).toHaveCount(0);
    expect(rows.some(row => row.type === type)).toBe(true);
  }
  await page.reload();
  const row = page.locator('.el-table__row').filter({ hasText: '测试 bark' });
  await row.getByText('编辑', { exact: true }).click();
  await expect(dialog().getByTestId('provider-field-server').locator('input')).toHaveValue('');
  await expect(dialog().getByTestId('provider-field-server')).toContainText('已配置');
  await expect(dialog().getByTestId('provider-field-group').locator('input')).toHaveValue('HomeWaf');
  await dialog().getByTestId('provider-field-group').locator('input').fill('重要告警');
  await dialog().getByRole('button', { name: '确认', exact: true }).click();
  await expect(dialog()).toHaveCount(0);
  expect(JSON.parse(rows.find(row => row.type === 'bark').config_json).server).toBe(samples.server);
  await row.locator('.el-switch').click();
  await expect.poll(() => rows.find(row => row.type === 'bark').status).toBe(0);
  expect(JSON.parse(rows.find(row => row.type === 'bark').config_json).server).toBe(samples.server);
  await row.getByText('测试', { exact: true }).click();
  await expect.poll(() => writes.some(write => write.action === 'test')).toBe(true);
  await page.goto(origin + '/#/notify/subscription');
  await expect(page.locator('th').filter({ hasText: '钉钉群机器人' })).toBeVisible();
  await expect(page.locator('th').filter({ hasText: 'dingtalk-title' })).toHaveCount(0);
  await page.getByRole('button', { name: '钉钉群机器人批量操作', exact: true }).click();
  await expect(page.locator('.el-dropdown-menu:visible')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByText('测试 bark', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('预留类型：尚未接入周报定时任务')).toBeVisible();
  await page.screenshot({ path: path.join(output, 'subscriptions.png'), fullPage: true });
  await page.goto(origin + '/#/notify/log');
  await expect(page.locator('.el-table__row').filter({ hasText: '系统错误' }).getByText('Bark', { exact: true })).toBeVisible();
  expect(errors).toEqual([]);
  const report = { providerCount: catalogue.length, created: rows.map(row => row.type), writes, errors, checks: ['create all eight providers', 'edit without losing masked secret', 'toggle status without losing secret', 'test action', 'subscription columns and event availability', 'log channel labels'] };
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  console.error(JSON.stringify({ errors, writes, visibleText: (await page.locator('body').innerText()).slice(-5000) }));
  throw error;
} finally {
  await browser.close();
  server.close();
}
