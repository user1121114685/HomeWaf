// Real login-page navigation in a fresh browser; all API requests are isolated fixtures.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';
const origin = process.env.CONSOLE_TEST_ORIGIN || 'http://127.0.0.1:3001';
const forcePassword = process.env.CONSOLE_TEST_FORCE_PASSWORD === '1';
let passwordChanged = false;
const changedNotice = process.env.CONSOLE_TEST_CHANGED_NOTICE === '1';
const output = fileURLToPath(new URL('../../artifacts/login-navigation/', import.meta.url));
fs.mkdirSync(output, { recursive: true });
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
const report = { checks: [], errors: [], warnings: [], navigation: [] };
await context.addInitScript(() => {
  localStorage.setItem('lang', 'zh_CN');
  window.__navigationDocumentID = crypto.randomUUID();
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const endpoint = new URL(route.request().url()).pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, msg: '', data: AesEncrypt(JSON.stringify(data)) } });
  if (endpoint.includes('handshake') || endpoint === 'public/seckey' || endpoint === 'sysinfo/checkversion')
    return route.fulfill({ json: { code: 1, msg: 'Isolated fixture' } });
  if (endpoint === 'public/login')
    return ok({
      access_token: 'isolated-login-navigation',
      need_change_password: forcePassword && !passwordChanged,
      login_notice: {
        current_ip: '127.0.0.1',
        current_area: '测试环境',
        current_time: '2026-11-14 12:00:00',
        is_changed: changedNotice,
        last_ip: '192.0.2.1',
        last_area: '上次测试环境',
        last_time: '2026-11-13 12:00:00',
      },
    });
  if (endpoint === 'account/changemypwd') {
    passwordChanged = true;
    return ok({});
  }
  if (endpoint === 'pipeline/overview') return ok({ ports: [], issues: {}, punishment: {}, log_queue: {} });
  if (endpoint === 'pipeline/templates' || endpoint === 'pipeline/regions' || endpoint === 'pipeline/migration/legacy')
    return ok([]);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
page.on('console', (message) => {
  if (message.type() === 'warning') report.warnings.push(message.text());
});
try {
  await page.goto(origin + '/#/login');
  await page.locator('.login-password input[type="text"]').last().fill('navigation-test');
  await page.locator('.login-password input[type="password"]').fill('navigation-test-password');
  const documentID = await page.evaluate(() => window.__navigationDocumentID);
  await page.locator('.login-password button[type="submit"]').click();
  if (forcePassword) {
    const dialog = page.getByRole('dialog', { name: '修改密码', exact: true });
    await expect(dialog).toBeVisible();
    const fields = dialog.locator('input[type="password"]');
    await fields.nth(0).fill('changed-navigation-password');
    await fields.nth(1).fill('changed-navigation-password');
    await dialog.locator('button[type="submit"]').click();
    report.checks.push('Forced password change completes before protected navigation');
  }
  await expect(page.getByTestId('workbench')).toBeVisible({ timeout: 30000 });
  report.checks.push('Fresh login succeeds without preinstalled token');
  await expect(page.locator('.el-notification').getByText('127.0.0.1', { exact: true })).toBeVisible();
  if (changedNotice) {
    await page.locator('.el-notification').getByRole('button', { name: '查看登录历史' }).click();
    await expect(page).toHaveURL(/account\/LoginHistory/);
    await expect(page.locator('.el-notification')).toHaveCount(0);
    await page.getByRole('navigation', { name: '常用功能' }).getByRole('link', { name: '工作台', exact: true }).click();
    report.checks.push('Login notice footer opens history and closes without render errors');
  }
  for (const [label, testID] of [
    ['服务接入', 'pipeline-page'],
    ['工作台', 'workbench'],
    ['策略模板', 'pipeline-page'],
    ['工作台', 'workbench'],
  ]) {
    await page.getByRole('navigation', { name: '常用功能' }).getByRole('link', { name: label, exact: true }).click();
    await expect(page.getByTestId(testID)).toBeVisible();
    report.navigation.push({ label, url: page.url() });
    expect(await page.evaluate(() => window.__navigationDocumentID)).toBe(documentID);
  }
  report.checks.push('Navigation works immediately after login without any document reload');
  await page.getByRole('button', { name: '查看全部功能' }).click();
  await page.getByRole('textbox', { name: '搜索全部功能' }).fill('IP数据库');
  await expect(page.locator('.nav-all-functions a').first()).toBeVisible();
  await page.locator('.nav-all-functions a').first().click();
  await expect(page).toHaveURL(/sys\/IPLocation/);
  await expect(page.locator('.test-section')).toBeVisible();
  report.checks.push('Full navigation renders IP location after login');
  await page.getByRole('button', { name: '账户菜单' }).click();
  await page.getByRole('menuitem', { name: /退出/ }).click();
  await expect(page.locator('.login-password')).toBeVisible();
  await page.locator('.login-password input[type="text"]').last().fill('navigation-test');
  await page.locator('.login-password input[type="password"]').fill('navigation-test-password');
  await page.locator('.login-password button[type="submit"]').click();
  await expect(page.getByTestId('workbench')).toBeVisible();
  await page.getByRole('navigation', { name: '常用功能' }).getByRole('link', { name: '服务接入', exact: true }).click();
  await expect(page.getByTestId('pipeline-page')).toBeVisible();
  expect(await page.evaluate(() => window.__navigationDocumentID)).toBe(documentID);
  report.checks.push('Logout and repeat login keep navigation usable without refresh');
  expect(report.errors).toEqual([]);
  console.log(
    JSON.stringify(
      { ...report, warnings: report.warnings.map((w) => w.split('\n')[0]).filter((w, i, a) => a.indexOf(w) === i) },
      null,
      2,
    ),
  );
} catch (error) {
  report.failure = String(error);
  report.state = await page.evaluate(() => ({
    url: location.href,
    inert: document.body.inert,
    overlays: [...document.querySelectorAll('.el-overlay')].map((el) => ({
      display: getComputedStyle(el).display,
      visibility: getComputedStyle(el).visibility,
    })),
    text: document.body.innerText.slice(0, 1500),
  }));
  await page.screenshot({ path: path.join(output, 'failure.png') });
  console.error(
    JSON.stringify(
      { ...report, warnings: report.warnings.map((w) => w.split('\n')[0]).filter((w, i, a) => a.indexOf(w) === i) },
      null,
      2,
    ),
  );
  throw error;
} finally {
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  await browser.close();
}
