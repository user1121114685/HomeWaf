import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';

const origin = process.env.OVERVIEW_TEST_ORIGIN || 'http://127.0.0.1:3001';
const output = new URL('../../artifacts/', import.meta.url);
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
const report = { checks: [], errors: [] };
let updateAvailable = false;
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-theme-verification');
  if (!localStorage.getItem('lang')) localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const path = new URL(route.request().url()).pathname;
  if (path.includes('handshake') || path.endsWith('/public/seckey')) return route.fulfill({ json: { code: 1, msg: 'Isolated legacy transport test' } });
  let data = { list: [], total: 0 }, msg = 'OK';
  if (path.endsWith('/sysinfo/checkversion')) {
    data = { need_update: updateAvailable, version: 'v2.0.0-dev', version_new: updateAvailable ? 'v2.0.1' : '', version_desc: 'Release verification', self_update_allowed: true };
    msg = updateAvailable ? '有新版本' : 'HomeWaf 更新源尚未发布正式版本，发布后即可检查更新';
  }
  if (path.endsWith('/systemconfig/list')) data = { list: [], total: 0 };
  return route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg } });
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
// Verify release availability without starting an upgrade. Appearance is covered by verify-console-browser.mjs.
try {
  await page.goto(origin + '/#/sys/SystemConfig');
  await expect(page.locator('.config-layout')).toBeVisible({ timeout: 20000 });
  await page.locator('.header-user-btn').click();
  await page.getByRole('menuitem', { name: '升级', exact: true }).click();
  await expect(page.getByText('HomeWaf 更新源尚未发布正式版本，发布后即可检查更新', { exact: true })).toBeVisible();
  await expect(page.locator('.el-dialog:visible')).toHaveCount(0);
  updateAvailable = true;
  await page.locator('.header-user-btn').click();
  await page.getByRole('menuitem', { name: '升级', exact: true }).click();
  await expect(page.locator('.el-dialog:visible')).toContainText('v2.0.1');
  report.checks.push('Empty release state shows an informational message; a newer release opens the update dialog');
  expect(report.errors).toEqual([]);
  fs.writeFileSync(new URL('update-theme-verification.json', output), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  console.error(JSON.stringify(report));
  await page.screenshot({ path: fileURLToPath(new URL('update-theme-failure.png', output)) });
  throw error;
} finally {
  await browser.close();
}
