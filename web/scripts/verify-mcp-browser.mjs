// Built UI with isolated API fixtures; backend MCP/persistence is tested in Go.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const dist = path.join(root, 'web/dist');
const output = path.join(root, 'artifacts/mcp-browser');
fs.mkdirSync(output, { recursive: true });
const server = http.createServer((req, res) => {
  const target = decodeURIComponent((req.url || '/').split('?')[0]);
  const file = path.resolve(dist, `.${target === '/' ? '/index.html' : target}`);
  if (!file.startsWith(dist + path.sep) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.writeHead(404); res.end(); return; }
  res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' }[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
const errors = [], checks = [], requests = [];
let failedRead = true, failedCreate = false, enabled = true, rejectMCP = true, createdCount = 0;
const fixtureToken = 'fixture-mcp-secret-never-persist';
const toolList = [
  { name: 'log_detail', group: '日志分析', description: '读取请求详情、攻击载荷和命中原因。', read_only: true },
  { name: 'service_save', group: '服务配置', description: '保存完整服务配置，校验 revision。', read_only: false },
  { name: 'service_delete', group: '服务配置', description: '删除已停止的指定端口。', read_only: false, requires_confirmation: true },
];
await context.addInitScript(() => { window.__SAMWAF_SECURITY_PATH__ = '/console-fixture'; localStorage.setItem('access_token', 'isolated-mcp-ui-test'); localStorage.setItem('lang', 'zh_CN'); Object.defineProperty(crypto, 'randomUUID', { configurable: true, value: undefined }); });
await context.addInitScript(() => {
  window.__mcpCopiedConfig = '';
  window.__mcpClipboardFails = false;
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value) => {
    if (window.__mcpClipboardFails) throw new Error('Clipboard fixture denied');
    window.__mcpCopiedConfig = value;
  } } });
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const req = route.request(), endpoint = new URL(req.url()).pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg } });
  let input = {};
  if (req.postData()) { try { input = JSON.parse(AesDecrypt(req.postData())); } catch { try { input = JSON.parse(req.postData()); } catch {} } }
  if (endpoint.includes('handshake') || endpoint === 'public/seckey' || endpoint === 'sysinfo/checkversion') return fail('Isolated fixture');
  if (endpoint === 'oplatform/mcp') return failedRead ? fail('MCP 状态读取失败，请重试。') : ok({ endpoint: '/mcp', transport: 'Streamable HTTP', enabled, tools: toolList });
  if (endpoint === 'oplatform/key/add') {
    requests.push(input);
    await new Promise((resolve) => setTimeout(resolve, 150));
    if (failedCreate) return fail('模拟创建失败，输入保留');
    createdCount++;
    return ok({ id: 'created-key', api_key: fixtureToken });
  }
  if (endpoint === 'oplatform/key/list') return ok({ list: [{ id: 'created-key', key_name: '运维客户端', api_key: '****', mcp_access: 'write', status: 1 }], total: 1 });
  if (endpoint.includes('systemconfig') && req.method() === 'POST') { enabled = input.value === '1'; return ok({}); }
  if (endpoint.includes('systemconfig')) return ok({ item: 'open_platform_enabled', value: enabled ? '1' : '0' });
  if (endpoint.includes('account') || endpoint.includes('user')) return ok({ user_name: 'test', login_account: 'fixture', role: 'super_admin', roles: ['ALL_ROUTERS'] });
  return ok({ list: [], total: 0, roles: ['ALL_ROUTERS'] });
});
await context.route('**/mcp', async (route) => {
	if (new URL(route.request().url()).pathname !== '/console-fixture/mcp') return route.fallback();
  const body = route.request().postDataJSON();
  expect(route.request().headers().authorization).toBe(`Bearer ${fixtureToken}`);
  requests.push({ rpc: body.method });
  if (rejectMCP) return route.fulfill({ status: 401, json: { error: 'invalid credential' } });
  return route.fulfill({ json: { jsonrpc: '2.0', id: body.id, result: body.method === 'initialize' ? { protocolVersion: '2025-11-25', serverInfo: { name: 'HomeWaf', version: '1' }, capabilities: { tools: {} } } : { tools: toolList } } });
});
const page = await context.newPage();
page.on('pageerror', (error) => errors.push(error.message));
try {
  await page.goto(`${origin}/#/oplatform/mcp`);
  await expect(page.getByText('MCP 状态读取失败，请重试。', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '自动生成密钥', exact: true })).toBeDisabled();
  failedRead = false;
  await page.getByRole('button', { name: '刷新状态', exact: true }).click();
  await expect(page.getByRole('heading', { name: '连接客户端' })).toBeVisible();
  await expect(page.locator('#mcp-url')).toHaveValue(`${origin}/console-fixture/mcp`);
  checks.push('Failed read is explicit, retry loads the real page');
  expect(createdCount).toBe(0);
  await page.getByRole('button', { name: '自动生成密钥', exact: true }).evaluate((button) => { button.click(); button.click(); });
  await expect(page.locator('#mcp-token')).toHaveValue(fixtureToken);
  expect(createdCount).toBe(1);
  expect(requests.filter((item) => item.key_name)).toHaveLength(1);
  expect(requests[0].key_name).toMatch(/^MCP 运维连接 .+ UTC$/);
  expect(requests[0].mcp_access).toBe('write');
  expect(requests[0].rate_limit).toBe(120);
  await expect(page.getByRole('dialog', { name: '创建 MCP 专用密钥' })).toHaveCount(0);
  await page.getByRole('button', { name: '复制连接配置', exact: true }).click();
  expect(createdCount).toBe(1);
  await page.getByRole('button', { name: '复制配置', exact: true }).click();
  const copied = await page.evaluate(() => JSON.parse(window.__mcpCopiedConfig));
  expect(copied.mcpServers.homewaf.headers.Authorization).toBe(`Bearer ${fixtureToken}`);
  expect(copied.mcpServers.homewaf.url).toBe(`${origin}/console-fixture/mcp`);
  expect(createdCount).toBe(1);
  checks.push('One click generates and fills a named write credential; duplicate clicks do not issue duplicate keys; copy reuses it');
  await page.getByRole('button', { name: '清除本页密钥', exact: true }).click();
  failedCreate = true;
  await page.getByRole('button', { name: '生成并复制配置', exact: true }).click();
  await expect(page.getByText('模拟创建失败，输入保留', { exact: true })).toBeVisible();
  await expect(page.locator('#mcp-token')).toHaveValue('');
  expect(createdCount).toBe(1);
  failedCreate = false;
  await page.evaluate(() => { window.__mcpClipboardFails = true; });
  await page.getByRole('button', { name: '生成并复制配置', exact: true }).click();
  await expect(page.locator('#mcp-token')).toHaveValue(fixtureToken);
  await expect(page.getByText('复制失败，密钥已保留。请再次点击“复制配置”，或分别复制地址和密钥。', { exact: true })).toBeVisible();
  expect(createdCount).toBe(2);
  await page.evaluate(() => { window.__mcpClipboardFails = false; });
  await page.getByRole('button', { name: '复制配置', exact: true }).click();
  expect(createdCount).toBe(2);
  checks.push('Copy without a key generates it automatically; generation failure is recoverable and clipboard retry never regenerates a credential');
  await page.getByRole('button', { name: '自定义密钥', exact: true }).click();
  await page.getByRole('textbox', { name: '连接名称', exact: true }).fill('');
  await page.getByRole('button', { name: '创建密钥', exact: true }).click();
  await expect(page.getByText('请填写连接名称。', { exact: true })).toBeVisible();
  await page.getByRole('textbox', { name: '连接名称', exact: true }).fill('运维客户端');
  await page.getByRole('radio', { name: '只读排障', exact: true }).press('Space');
  await expect(page.getByRole('radio', { name: '只读排障', exact: true })).toBeChecked();
  failedCreate = true;
  await page.getByRole('button', { name: '创建密钥', exact: true }).click();
  await expect(page.getByText('模拟创建失败，输入保留')).toBeVisible();
  await expect(page.getByRole('textbox', { name: '连接名称', exact: true })).toHaveValue('运维客户端');
  failedCreate = false;
  await page.getByRole('button', { name: '创建密钥', exact: true }).click();
  await expect(page.locator('#mcp-token')).toHaveValue(fixtureToken);
  await expect.poll(() => createdCount).toBe(3);
  expect(requests.filter((item) => item.key_name).at(-1).mcp_access).toBe('read');
  await page.getByRole('button', { name: '验证连接', exact: true }).click();
  await expect(page.getByText('认证失败：检查密钥用途、启用状态、有效期和开放 API 开关。')).toBeVisible();
  await expect(page.locator('#mcp-token')).toHaveValue(fixtureToken);
  rejectMCP = false;
  await page.getByRole('button', { name: '验证连接', exact: true }).click();
  await expect(page.getByText(/连接通过：HomeWaf/)).toBeVisible();
  expect(requests.filter((item) => item.rpc).every((item) => ['initialize', 'tools/list'].includes(item.rpc))).toBe(true);
  expect(await page.evaluate((secret) => JSON.stringify({ ...localStorage, ...sessionStorage }).includes(secret), fixtureToken)).toBe(false);
  checks.push('Create validates input, retains failed drafts, submits dedicated write key once; connection verification retries without writing protection');
  await page.locator('#mcp-filter').fill('not-present');
  await expect(page.getByText('没有匹配的功能，请调整搜索。')).toBeVisible();
  await page.locator('#mcp-filter').fill('');
  await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 10000 });
  for (const [width, height, theme] of [[1440, 1000, 'light'], [820, 1100, 'light'], [390, 844, 'light'], [390, 844, 'dark']]) {
    await page.setViewportSize({ width, height });
    await page.evaluate((theme) => { document.documentElement.classList.toggle('dark', theme === 'dark'); document.documentElement.setAttribute('theme-mode', theme); document.documentElement.setAttribute('data-theme', theme); }, theme);
    await page.evaluate(() => { document.activeElement?.blur(); window.scrollTo(0, 0); });
    await page.screenshot({ path: path.join(output, `${width}-${theme}.png`), fullPage: true });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await expect(page.locator('#mcp-url')).toBeVisible();
    await expect(page.getByRole('button', { name: '验证连接', exact: true })).toBeVisible();
  }
  checks.push('Desktop/tablet/phone and dark mode have no page overflow; controls and error/empty states are reachable');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole('link', { name: '管理密钥', exact: true }).click();
  await expect(page.getByText('MCP · 排障与配置', { exact: true })).toBeVisible();
  await page.goto(`${origin}/#/oplatform/mcp`);
  await expect(page.locator('#mcp-token')).toHaveValue('');
  expect(createdCount).toBe(3);
  checks.push('Key management shows scope; leaving and reopening clears the transient credential');
  expect(errors).toEqual([]);
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify({ checks, errors, createdCount }, null, 2));
  console.log(JSON.stringify({ checks, errors, createdCount }, null, 2));
} finally { await browser.close(); await new Promise((resolve) => server.close(resolve)); }
