// Exercise the real bundled update dialog; every API is an isolated response.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';
import { listenLoopbackForTest } from '../../scripts/test-listener.mjs';

const root = fileURLToPath(new URL('../..', import.meta.url));
const dist = path.join(root, 'web/dist');
const output = path.join(root, 'artifacts/update-browser');
await fs.mkdir(output, { recursive: true });
const server = http.createServer(async (req, res) => {
  const pathname = new URL(req.url, 'http://localhost').pathname;
  const file = path.resolve(dist, '.' + (pathname === '/' ? '/index.html' : decodeURIComponent(pathname)));
  if (!file.startsWith(dist + path.sep)) { res.writeHead(404).end(); return; }
  try {
    const data = await fs.readFile(file);
    res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css', '.json': 'application/json', '.svg': 'image/svg+xml' }[path.extname(file)] || 'application/octet-stream');
    res.end(data);
  } catch { res.writeHead(404).end(); }
});
await listenLoopbackForTest(server);
const browser = await chromium.launch({ headless: true, ...(process.env.CI ? {} : { channel: 'chrome' }) });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-update-test');
  localStorage.setItem('lang', 'zh_CN');
  localStorage.removeItem('lastUpdatePopupTime');
});
await context.routeWebSocket('**/api/**', socket => socket.close());
const fullCommit = '47f4857bfeb7788b9a7f2e8d111825ec5072c838';
let updateAllowed = true, version = 'v2.0.3', failure = 'server', running = false, history = true;
const calls = [], errors = [];
await context.route('**/api/**', async route => {
  const request = route.request();
  const url = new URL(request.url());
  const endpoint = url.pathname.split('/api/v1/')[1] || '';
  const reply = (code, data, msg = '') => route.fulfill({ json: { code, data: AesEncrypt(JSON.stringify(data)), msg } });
  if (endpoint.includes('handshake') || endpoint === 'public/seckey') return reply(1, {}, 'Isolated fixture');
  if (endpoint === 'sysinfo/checkversion') return reply(0, {
    need_update: true, version: 'v2.0.1', version_new: version,
    version_desc: `HomeWaf ${version}\n## Changelog\n* ${fullCommit} fix: 修复升级按钮`,
    version_notes: history ? [
      { version, items: ['修复升级按钮', '<img src=x onerror="window.__releaseXss=1">'] },
      { version: 'v2.0.2', items: ['改进证书管理'] },
    ] : undefined,
    self_update_allowed: updateAllowed, container: updateAllowed ? '' : 'docker',
  });
  if (endpoint === 'sysinfo/update') {
    calls.push(url.searchParams.get('channel'));
    await new Promise(resolve => setTimeout(resolve, 300));
    if (failure === 'network') return route.abort('failed');
    if (failure === 'server') return reply(1, {}, '模拟更新请求失败，可重试');
    running = true;
    return reply(0, {}, '已发起升级');
  }
  if (endpoint === 'sysinfo/updateprogress') return reply(0, {
    state: running ? 'running' : 'idle', stage: 'download', stage_index: 3, percent: 20,
    downloaded: 200, total: 1000, speed: 100, from_version: 'v2.0.1', to_version: version,
    started_at: Date.now(), stages: [],
  });
  if (endpoint === 'sysinfo/version') return reply(0, { version: 'v2.0.1' });
  if (['account/info', 'account/current', 'account/detail'].includes(endpoint)) return reply(0, { user_name: 'fixture', name: 'fixture' });
  if (endpoint.includes('allhost') || endpoint.includes('options')) return reply(0, []);
  return reply(0, { list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', error => errors.push(error.message));
const origin = `http://127.0.0.1:${server.address().port}`;
const dialog = () => page.getByTestId('software-update-dialog');
const open = async () => {
  await page.goto(origin + '/?scenario=' + Date.now() + '#/dashboard/base');
  if (version.includes('beta')) await page.locator('.header-update-btn').click();
  await expect(dialog()).toBeVisible();
};
const summary = [];
try {
  await open();
  await expect(dialog().locator('.el-dialog__footer button')).toHaveText(['取消', '更新']);
  await expect(dialog().locator('.software-update-notes h4')).toHaveText(['v2.0.3', 'v2.0.2']);
  await expect(dialog()).not.toContainText(fullCommit);
  await expect(dialog().getByRole('link', { name: '更多', exact: true })).toHaveCount(0);
  await expect(dialog().locator('img')).toHaveCount(0);
  expect(await page.evaluate(() => window.__releaseXss)).toBeUndefined();
  for (const width of [1440, 768, 390]) for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width, height: 1000 });
    await page.evaluate(theme => {
      document.documentElement.dataset.theme = theme;
      document.documentElement.classList.toggle('dark', theme === 'dark');
    }, theme);
    const bounds = await dialog().boundingBox();
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(width + 1);
    await page.screenshot({ path: path.join(output, `update-${width}-${theme}.png`), animations: 'disabled' });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await dialog().getByRole('button', { name: '取消', exact: true }).click();
  await expect(dialog()).toBeHidden();
  expect(calls.length).toBe(0);
  await page.locator('.header-update-btn').click();
  await dialog().getByRole('button', { name: '更新', exact: true }).evaluate(button => { button.click(); button.click(); });
  await expect(dialog().getByRole('button', { name: '取消', exact: true })).toBeDisabled();
  await expect(dialog().getByText('模拟更新请求失败，可重试')).toBeVisible();
  expect(calls).toEqual(['official']);
  failure = 'network';
  await dialog().getByRole('button', { name: '更新', exact: true }).click();
  await expect(dialog().getByText('更新请求失败，请检查网络后重试。')).toBeVisible();
  failure = '';
  await dialog().getByRole('button', { name: '更新', exact: true }).click();
  await expect(dialog()).toBeHidden();
  await expect(page.getByRole('dialog').getByText('正在升级', { exact: true })).toBeVisible();
  expect(calls).toEqual(['official', 'official', 'official']);
  summary.push('Cancel does not update; repeated clicks send one request; server/network failures preserve retry; success opens progress.');
  running = false; updateAllowed = false;
  await open();
  await expect(dialog().getByRole('button', { name: '更新', exact: true })).toHaveCount(0);
  await expect(dialog().getByText('容器环境不支持应用内升级', { exact: true })).toBeVisible();
  await expect(dialog()).not.toContainText('更新完成后将自动平滑重启');
  await dialog().getByRole('button', { name: '取消', exact: true }).click();
  expect(calls.length).toBe(3);
  updateAllowed = true; history = false;
  await open();
  await expect(dialog()).toContainText('修复升级按钮');
  await expect(dialog()).not.toContainText(fullCommit);
  summary.push('Version groups and legacy notes render as text without commit hashes or More links; containers remain protected.');
  await dialog().getByRole('button', { name: '取消', exact: true }).click();
  version = 'v2.0.4-beta.1';
  await open();
  await dialog().getByRole('button', { name: '更新', exact: true }).click();
  const confirmation = page.getByRole('dialog', { name: '确认更新测试版本', exact: true });
  await expect(confirmation).toContainText('测试版本');
  await confirmation.getByRole('button', { name: '取消', exact: true }).click();
  expect(calls.length).toBe(3);
  summary.push('Prerelease updates retain their second confirmation.');
  expect(errors).toEqual([]);
  await fs.writeFile(path.join(output, 'report.json'), JSON.stringify({ summary, calls, errors }, null, 2));
  console.log(summary.join('\n'));
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png') });
  throw error;
} finally {
  await browser.close();
  await new Promise(resolve => server.close(resolve));
}
