// Verify password masking with synthetic credentials and fully isolated APIs.
import { chromium, expect } from '@playwright/test';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const origin = process.env.CONSOLE_TEST_ORIGIN || 'http://127.0.0.1:3118';
const output = new URL('../../artifacts/password-toggle/', import.meta.url);
fs.mkdirSync(output, { recursive: true });
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  await context.addInitScript(() => { localStorage.removeItem('access_token'); localStorage.setItem('lang', 'zh_CN'); });
  await context.routeWebSocket('**/api/**', ws => ws.close());
  let loginRequests = 0;
  await context.route('**/api/**', async route => {
    const endpoint = new URL(route.request().url()).pathname.split('/api/v1/')[1] || '';
    if (endpoint === 'public/login') {
      loginRequests++;
      return route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify({ access_token: 'isolated-password-test', need_change_password: true })), msg: '' } });
    }
    return route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg: 'Isolated password fixture' } });
  });
  const page = await context.newPage();
  await page.goto(origin + '/#/login');
  const input = page.locator('input[autocomplete="current-password"]');
  const field = page.locator('.el-input').filter({ has: input });
  const toggle = field.locator('.login-password-toggle');
  await input.fill('synthetic-visibility-test');
  for (const dark of [false, true]) {
    await page.evaluate(value => document.documentElement.classList.toggle('dark', value), dark);
    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      await expect(input).toHaveAttribute('type', 'password');
      await expect(toggle).toHaveCount(1);
      await expect(field.locator('.el-input__password')).toHaveCount(0);
      await toggle.focus();
      await page.keyboard.press('Enter');
      await expect(input).toHaveAttribute('type', 'text');
      await expect(toggle).toHaveAttribute('aria-pressed', 'true');
      await expect(input).toHaveValue('synthetic-visibility-test');
      await page.keyboard.press('Space');
      await expect(input).toHaveAttribute('type', 'password');
      await expect(toggle).toHaveAttribute('aria-pressed', 'false');
      await expect(field.locator('.el-input__password')).toHaveCount(0);
      await field.screenshot({ path: fileURLToPath(new URL(`${dark ? 'dark' : 'light'}-${width}.png`, output)) });
    }
  }
  expect(loginRequests).toBe(0);
  console.log('PASS: one reveal control in both states; keyboard toggles preserve the value without submitting');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.locator('input[autocomplete="username"]').fill('visibility-test');
  await page.locator('.login-password button[type="submit"]').click();
  const dialog = page.getByRole('dialog', { name: '修改密码', exact: true });
  await expect(dialog).toBeVisible();
  const defaults = dialog.locator('input[type="password"]');
  await defaults.nth(0).fill('synthetic-new-password');
  await defaults.nth(1).fill('synthetic-new-password');
  await expect(dialog.locator('.el-input__password')).toHaveCount(2);
  await dialog.locator('.el-input__password').first().click();
  await expect(dialog.locator('input[type="text"]')).toHaveCount(1);
  console.log('PASS: other password fields retain their default reveal controls');
} finally { await browser.close(); }
