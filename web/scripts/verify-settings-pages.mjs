import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const origin = process.env.OVERVIEW_TEST_ORIGIN || 'http://127.0.0.1:3001';
const key = fs
  .readFileSync(new URL('../../.cache/screen-preview/data/.keys/cli_api_key', import.meta.url), 'utf8')
  .trim();
const output = new URL('../../artifacts/', import.meta.url);
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1100 }, colorScheme: 'light' });
const errors = [],
  checked = [],
  rejected = [];
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-readonly-ui-verification');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const request = route.request(),
    url = new URL(request.url()),
    path = url.pathname.split('/api/v1/')[1] || '';
  if (
    path.includes('handshake') ||
    path === 'public/seckey' ||
    path === 'sysinfo/checkversion' ||
    path === 'sysinfo/announcement'
  )
    return route.fulfill({ json: { code: 1, msg: 'Not needed for display verification' } });
  const readPost = path === 'systemconfig/list';
  if (
    (request.method() !== 'GET' && !readPost) ||
    /\/(?:del|delete|update|restart|reset|save|rollback)(?:\/|$)/i.test(path)
  ) {
    rejected.push(path);
    return route.fulfill({ status: 403, json: { code: 1, msg: 'Display verification permits read-only requests' } });
  }
  const response = await fetch('http://127.0.0.1:26666' + url.pathname + url.search, {
    method: readPost ? 'POST' : 'GET',
    headers: { 'X-API-Key': key, 'Content-Type': 'application/json' },
    body: readPost ? AesDecrypt(request.postData()) : undefined,
    signal: AbortSignal.timeout(20000),
  });
  const envelope = await response.json();
  if (envelope.code === 0) envelope.data = AesEncrypt(JSON.stringify(envelope.data));
  await route.fulfill({ json: envelope });
});
const page = await context.newPage();
page.on('pageerror', (error) => errors.push(error.message));
try {
  for (const [path, selector, name] of [
    ['/sys/SystemConfig', '.config-layout', 'system-config'],
    ['/sys/VpConfig', '.vpconfig-page', 'management-settings'],
    ['/sys/IPLocation', '.iplocation-page, .ipdb-page, .iplocation-manager, .el-main', 'ip-database'],
    ['/dashboard/overview', '.site-overview', 'site-statistics'],
  ]) {
    await page.goto(origin + '/#' + path);
    await expect(page.locator(selector).first()).toBeVisible({ timeout: 20000 });
    await expect(page.locator('.hw-loading-mask')).toHaveCount(0, { timeout: 25000 });
    await expect(page).not.toHaveURL(/login/);
    if (path === '/sys/SystemConfig') await expect(page.locator('.config-row').first()).toBeVisible();
    if (path === '/sys/VpConfig') {
      const label = await page.locator('#vp-sec-ip .el-form-item__label').boundingBox();
      const field = await page.locator('#vp-sec-ip .el-textarea').boundingBox();
      expect(Math.abs(label.width - 180)).toBeLessThan(1);
      expect(field.x).toBeGreaterThanOrEqual(label.x + label.width);
    }
    if (path === '/dashboard/overview') {
      await expect(page.locator('.site-table-card .hw-card-title')).toContainText('站点实时安全明细');
      await expect(page.locator('.site-table-card .hw-card-actions')).toContainText('点击数据行');
      const cards = await page
        .locator('.summary-cards .el-col')
        .evaluateAll((items) => items.map((el) => el.getBoundingClientRect().toJSON()));
      expect(cards.length).toBe(6);
      expect(Math.abs(cards[0].top - cards[2].top)).toBeLessThan(1);
      expect(cards[3].top).toBeGreaterThan(cards[0].bottom);
    }
    await page.screenshot({ path: fileURLToPath(new URL(`ui-${name}-light.png`, output)) });
    await page.getByRole('button', { name: '系统设置', exact: true }).click();
    await page.locator('.setting-theme-modes label:has(input[value="dark"])').click();
    await page.locator('.setting-drawer-container .el-drawer__close-btn').click();
    await expect(page.locator('.setting-drawer-container')).toBeHidden();
    await expect(page.locator('html')).toHaveClass(/dark/);
    await page.screenshot({ path: fileURLToPath(new URL(`ui-${name}-dark.png`, output)) });
    await page.getByRole('button', { name: '系统设置', exact: true }).click();
    await page.locator('.setting-theme-modes label:has(input[value="light"])').click();
    await page.locator('.setting-drawer-container .el-drawer__close-btn').click();
    await expect(page.locator('.setting-drawer-container')).toBeHidden();
    checked.push(path);
  }
  expect(errors).toEqual([]);
  const result = { checked, errors, blockedWrites: rejected };
  fs.writeFileSync(new URL('settings-pages-verification.json', output), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
} catch (error) {
  console.error(JSON.stringify({ checked, errors, blockedWrites: rejected }));
  await page.screenshot({ path: fileURLToPath(new URL('settings-pages-failure.png', output)) });
  throw error;
} finally {
  await browser.close();
}
