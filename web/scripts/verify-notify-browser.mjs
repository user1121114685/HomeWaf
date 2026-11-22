// Real Vue forms against isolated in-memory responses. No notification or
// configuration request from this browser reaches a live management API.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import { handleTransportHandshake, routeSession } from './transport-fixture.mjs';

const root = fileURLToPath(new URL('../..', import.meta.url));
const dist = path.join(root, 'web', 'dist');
const output = path.join(root, 'artifacts', 'notify-verification');
fs.mkdirSync(output, { recursive: true });
const catalogue = JSON.parse(fs.readFileSync(path.join(root, 'internal/jobs/wafnotify/channels/catalog.json'), 'utf8'));
const mime = {
  '.html': 'text/html',
  '.js': 'application/javascript',
  '.css': 'text/css',
  '.svg': 'image/svg+xml',
  '.json': 'application/json',
};
const server = http.createServer((req, res) => {
  const requested = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
  const file = path.resolve(dist, '.' + (requested === '/' ? '/index.html' : requested));
  if (!file.startsWith(dist + path.sep) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
    res.writeHead(404);
    res.end();
    return;
  }
  res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-notification-test');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (socket) => socket.close());
const rows = [],
  subscriptions = [],
  writes = [],
  errors = [];
let failLogList = false, failChannelList = false,
  failSubscriptionSave = false,
  failGlobal = false;
const globalThrottle = {
  mode: 'aggregate',
  debug_mode: false,
  throttle: {
    aggregate_window_sec: 60,
    aggregate_max_detail: 10,
    cooldown_steps_sec: [60, 300],
    cooldown_reset_sec: 1800,
    max_per_hour: 100,
    dedup_keys: ['message_type'],
  },
};
const mask = (row) => {
  const result = structuredClone(row),
    config = JSON.parse(result.config_json || '{}');
  result.has_config_secrets = {};
  for (const field of catalogue.find((item) => item.type === row.type).fields)
    if (field.secret) {
      result.has_config_secrets[field.key] = !!config[field.key];
      delete config[field.key];
    }
  result.config_json = JSON.stringify(config);
  return result;
};
await context.route('**/api/**', async (route) => {
  if (await handleTransportHandshake(route)) return;
  const request = route.request(),
    endpoint = new URL(request.url()).pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: routeSession(route).encrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: -1, data: routeSession(route).encrypt('{}'), msg } });
  if (endpoint === 'sysinfo/checkversion')
    return fail('Isolated notification fixture');
  let input = {};
  if (request.postData()) {
    input = JSON.parse(routeSession(route).decrypt(request.postData()));
  }
  if (endpoint === 'notify/channel/types') return ok(catalogue);
  if (endpoint === 'notify/channel/list')
    return failChannelList
      ? fail('模拟通知渠道读取失败')
      : ok({ list: rows.filter((row) => !input.type || row.type === input.type).map(mask), total: rows.length });
  if (endpoint === 'notify/channel/add') {
    rows.push({ ...input, id: `channel-${rows.length}`, create_time: '2026-11-14 10:00:00' });
    writes.push({ action: 'add', type: input.type });
    return ok({});
  }
  if (endpoint === 'notify/channel/edit') {
    const old = rows.find((row) => row.id === input.id),
      submitted = JSON.parse(input.config_json),
      current = JSON.parse(old.config_json);
    for (const field of catalogue.find((item) => item.type === input.type).fields)
      if (field.secret) {
        if (submitted[field.key] === '__SAMWAF_CLEAR__') submitted[field.key] = '';
        else if (!submitted[field.key] && old.type === input.type) submitted[field.key] = current[field.key];
      }
    Object.assign(old, input, { config_json: JSON.stringify(submitted) });
    writes.push({ action: 'edit', type: input.type });
    return ok({});
  }
  if (endpoint === 'notify/channel/test') {
    writes.push({ action: 'test' });
    return ok({});
  }
  if (endpoint === 'notify/subscription/list') return ok({ list: subscriptions, total: subscriptions.length });
  if (endpoint === 'notify/subscription/add') {
    subscriptions.push({ ...input, id: 'sub-' + subscriptions.length });
    return ok({});
  }
  if (endpoint === 'notify/subscription/edit') {
    Object.assign(
      subscriptions.find((item) => item.id === input.id),
      input,
    );
    writes.push({ action: 'subscription-edit', channel: input.channel_id });
    return ok({});
  }
  if (endpoint === 'notify/subscription/config') {
    if (failSubscriptionSave) return fail('模拟订阅保存失败');
    const old = subscriptions.find((item) => item.id === input.id);
    Object.assign(old, {
      throttle_mode: input.throttle_mode,
      throttle_json: JSON.stringify(input.throttle),
      filter_json: JSON.stringify(input.filter),
      title_template: input.title_template,
      content_template: input.content_template,
    });
    writes.push({ action: 'subscription-config', id: input.id });
    return ok({});
  }
  if (endpoint === 'notify/subscription/templatevars')
    return ok({ vars: [{ name: 'domain', desc: '域名' }], default_title: '默认标题', default_content: '默认消息' });
  if (endpoint === 'notify/subscription/preview') {
    writes.push({ action: 'preview' });
    return ok({ title: input.title_template || '默认标题', content: '模拟预览内容', is_fallback: false });
  }
  if (endpoint === 'notify/subscription/dryrun') {
    writes.push({ action: 'dryrun', id: input.id });
    return ok({ action: 'send', would_send: true, reason_text: '隔离配置允许发送', hour_used: 1 });
  }
  if (endpoint === 'notify/subscription/test') {
    writes.push({ action: 'subscription-test', id: input.id });
    return ok({});
  }
  if (endpoint === 'notify/globalthrottle') return failGlobal ? fail('模拟全局配置读取失败') : ok(globalThrottle);
  if (endpoint === 'notify/globalthrottle/update') {
    Object.assign(globalThrottle, input);
    writes.push({ action: 'global-config' });
    return ok({});
  }
  if (endpoint === 'notify/log/list' && failLogList) return fail('模拟发送记录读取失败');
  if (endpoint === 'notify/log/list')
    return ok({
      list: [
        {
          id: 'log-1',
          channel_name: '测试 bark',
          channel_type: 'bark',
          message_type: 'system_error',
          message_title: '系统错误',
          status: 1,
        },
      ],
      total: 1,
    });
  if (endpoint.startsWith('notify/') && request.method() !== 'GET' && !endpoint.endsWith('/list'))
    return fail('Unexpected notification mutation: ' + endpoint);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (error) => errors.push(error.message));
const dialog = () => page.locator('.el-dialog:visible');
const choose = async (select, name) => {
  await select.click();
  await page.getByRole('option', { name, exact: true }).click();
};
try {
  await page.goto(origin + '/#/notify/channel');
  await expect(page.getByRole('button', { name: '添加渠道', exact: true })).toBeVisible();
  const samples = {
    appKey: 'app-key',
    appSecret: 'app-secret',
    userIds: 'user-1',
    corpID: 'corp-id',
    agentID: '1001',
    secret: 'corp-secret',
    appId: 'app-id',
    receiveId: 'receiver',
    server: 'https://api.day.app/fake-device-key',
    apiToken: '123:bot-token',
    chatId: '-1001',
    token: 'push-token',
    webServer: 'https://gotify.example.com/message?token=fake-token',
    address: 'broker.example.com',
    outMessageTopic: 'homewaf/alerts',
  };
  for (const type of [
    'bark',
    'telegram',
    'pushplus',
    'gotify',
    'dingtalk_app',
    'wechatwork_app',
    'feishu_app',
    'socket',
  ]) {
    const definition = catalogue.find((item) => item.type === type);
    await page.getByRole('button', { name: '添加渠道', exact: true }).click();
    await dialog().locator('input').first().fill(`测试 ${type}`);
    await choose(dialog().locator('.el-select').first(), definition.name);
    for (const field of definition.fields) {
      if (field.required && !field.default && samples[field.key])
        await dialog().getByTestId(`provider-field-${field.key}`).locator('input').fill(samples[field.key]);
    }
    if (type === 'bark') {
      await dialog().getByTestId('provider-field-group').locator('input').fill('HomeWaf');
      await dialog().screenshot({ path: path.join(output, 'bark-create.png') });
    }
    await dialog().getByRole('button', { name: '确认', exact: true }).click();
    await expect(dialog()).toHaveCount(0);
    expect(rows.some((row) => row.type === type)).toBe(true);
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
  expect(JSON.parse(rows.find((row) => row.type === 'bark').config_json).server).toBe(samples.server);
  await row.locator('.el-switch').click();
  await expect.poll(() => rows.find((row) => row.type === 'bark').status).toBe(0);
  expect(JSON.parse(rows.find((row) => row.type === 'bark').config_json).server).toBe(samples.server);
  await row.getByText('测试', { exact: true }).click();
  await expect.poll(() => writes.some((write) => write.action === 'test')).toBe(true);
  rows.splice(1);
  rows[0].status = 1;
  subscriptions.push({ id: 'subscription-1', channel_id: rows[0].id, message_type: 'system_error', status: 1 });
  await page.goto(origin + '/#/notify/subscription');
  await expect(page.locator('th')).toHaveCount(2);
  await expect(page.locator('th').filter({ hasText: 'Bark' })).toBeVisible();
  await expect(page.locator('th').filter({ hasText: '钉钉群机器人' })).toHaveCount(0);
  await expect(page.locator('th').filter({ hasText: 'dingtalk-title' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Bark订阅批量操作', exact: true }).click();
  await expect(page.locator('.el-dropdown-menu:visible')).toBeVisible();
  await expect(page.locator('.el-dropdown-menu:visible')).toContainText('仅影响 Bark 渠道的全部消息类型');
  await page.getByRole('menuitem', { name: '暂停已有订阅', exact: true }).click();
  await expect(dialog()).toContainText('此操作将禁用该渠道的所有订阅。');
  await dialog().getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => subscriptions[0].status).toBe(0);
  await expect(page.getByText('测试 bark', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('预留类型：尚未接入周报定时任务')).toBeVisible();
  await expect(dialog()).toHaveCount(0);
  await page.getByRole('button', { name: '设置测试 bark订阅', exact: true }).click();
  const drawer = page.locator('.el-drawer:visible');
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole('tab', { name: '过滤条件', exact: true })).toHaveAttribute('aria-selected', 'true');
  await drawer.getByLabel('只发这些域名', { exact: true }).fill('app.example.com');
  await drawer.getByRole('tab', { name: '消息模板', exact: true }).click();
  await drawer.getByLabel('标题模板', { exact: true }).fill('保留的模板标题');
  await drawer.getByRole('button', { name: '刷新预览', exact: true }).click();
  await expect(drawer.getByText('模拟预览内容')).toBeVisible();
  await drawer.getByRole('tab', { name: '测试与预览', exact: true }).click();
  await expect(drawer).toContainText('规则试算使用服务端已保存的订阅');
  await drawer.getByRole('button', { name: /干跑|试算/ }).click();
  await expect(drawer).toContainText('隔离配置允许发送');
  failSubscriptionSave = true;
  await drawer.getByRole('button', { name: '确认', exact: true }).click();
  await expect(drawer.getByText('模拟订阅保存失败')).toBeVisible();
  await drawer.getByRole('tab', { name: '过滤条件', exact: true }).click();
  await expect(drawer.getByLabel('只发这些域名', { exact: true })).toHaveValue('app.example.com');
  for (const width of [1440, 768, 390])
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width, height: 1050 });
      await page.evaluate((theme) => document.documentElement.classList.toggle('dark', theme === 'dark'), theme);
      for (const tab of ['过滤条件', '频率控制', '消息模板', '测试与预览']) {
        await drawer.getByRole('tab', { name: tab, exact: true }).click();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBe(true);
        await page.screenshot({
          path: path.join(output, `drawer-${tab}-${width}-${theme}.png`),
          animations: 'disabled',
        });
      }
    }
  await page.setViewportSize({ width: 1440, height: 1100 });
  failSubscriptionSave = false;
  await drawer.getByRole('button', { name: '确认', exact: true }).click();
  await expect(drawer).toHaveCount(0);
  expect(JSON.parse(subscriptions[0].filter_json).domains).toEqual(['app.example.com']);
  expect(subscriptions[0].title_template).toBe('保留的模板标题');
  expect(subscriptions[0].status).toBe(0);
  failGlobal = true;
  await page.getByRole('button', { name: '全局默认', exact: true }).click();
  await expect(dialog().getByText('模拟全局配置读取失败')).toBeVisible();
  await expect(dialog().getByRole('button', { name: '确认', exact: true })).toBeDisabled();
  failGlobal = false;
  await dialog().getByRole('button', { name: '重试读取配置' }).click();
  await expect(dialog().getByRole('button', { name: '确认', exact: true })).toBeEnabled();
  await dialog().screenshot({ path: path.join(output, 'global-throttle.png'), animations: 'disabled' });
  await dialog().getByRole('button', { name: '关闭', exact: true }).click();
  expect(writes.some((write) => write.action === 'global-config')).toBe(false);
  await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 10000 });
  await page.screenshot({ path: path.join(output, 'subscriptions.png'), fullPage: true });
  const releaseRow = page.locator('.el-table__row').filter({ hasText: '新版本发布' });
  await releaseRow.getByRole('button', { name: '订阅到 测试 bark' }).click();
  await expect.poll(() => subscriptions.find(sub => sub.message_type === 'software_update')?.status).toBe(1);
  await page.reload();
  await expect(releaseRow.getByRole('switch')).toBeChecked();
  await releaseRow.locator('.el-switch').click();
  await expect.poll(() => subscriptions.find(sub => sub.message_type === 'software_update')?.status).toBe(0);
  for (const width of [768, 390, 320]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBe(true);
    await expect(page.locator('.subscription-event-card').first()).toContainText('订阅到');
    await page.screenshot({ path: path.join(output, 'subscriptions-' + width + '.png'), fullPage: true });
  }
  await page.goto(origin + '/#/notify/log');
  await expect(
    page.locator('.el-table__row').filter({ hasText: '系统错误' }).getByText('Bark', { exact: true }),
  ).toBeVisible();
  failLogList = true;
  await page.reload();
  await expect(page.getByText('模拟发送记录读取失败', { exact: true })).toBeVisible();
  await expect(page.locator('.el-table__row')).toHaveCount(0);
  failLogList = false;
  await page.getByRole('button', {name: '重试', exact: true}).click();
  await expect(page.locator('.el-table__row').filter({hasText: '系统错误'})).toBeVisible();
  await page.evaluate(() => localStorage.setItem('homewaf.console.preferences', JSON.stringify({ mode: 'dark' })));
  await page.goto(origin + '/#/notify/subscription');
  await page.reload();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await expect(page.locator('.subscription-event-card').first()).toBeVisible();
  await page.screenshot({ path: path.join(output, 'subscriptions-dark-320.png'), fullPage: true });
  rows.length = 0;
  await page.reload();
  await expect(page.getByText('先添加一个通知渠道', { exact: true })).toBeVisible();
  failChannelList = true;
  await page.reload();
  await expect(page.getByText('模拟通知渠道读取失败', { exact: true })).toBeVisible();
  await expect(page.getByText('先添加一个通知渠道', { exact: true })).toHaveCount(0);
  expect(errors).toEqual([]);
  const report = {
    providerCount: catalogue.length,
    created: writes.filter((write) => write.action === 'add').map((write) => write.type),
    writes,
    errors,
    checks: [
      'create all eight providers',
      'edit without losing masked secret',
      'toggle status without losing secret',
      'test action',
      'only configured Bark column; explicit batch menu and real confirmation body',
      'desktop and mobile layouts; dark, empty and error states',
      'log channel labels',
      'new release subscription can be enabled, reopened and paused',
      'subscription filter/template draft survives failure; preview and saved-config dry-run remain distinct',
      'global config read failure blocks writes, retries successfully and cancellation writes nothing',
      'four subscription panes at three widths in both themes',
    ],
  };
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
