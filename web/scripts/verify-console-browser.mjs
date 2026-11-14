// Critical UI regression: every registered route must mount, and the mobile
// navigation must remain operable. Uses only the isolated preview fixture.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
const origin = process.env.CONSOLE_TEST_ORIGIN || 'http://127.0.0.1:3118';
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw new Error('Use a local isolated fixture');
const output = new URL('../../artifacts/console-refresh/', import.meta.url);
fs.mkdirSync(output, { recursive: true });
const report = { routes: [], errors: [], checks: [], consoleErrors: [] };
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
  const page = await context.newPage();
  await context.routeWebSocket('**/api/**', (socket) => socket.close());
  let current = 'startup';
  page.on('pageerror', (error) => report.errors.push({ route: current, message: error.message }));
  page.on('console', (message) => {
    if (message.type() === 'error') {
      report.consoleErrors.push({ route: current, message: message.text() });
      if (/\b(TypeError|ReferenceError|SyntaxError)\b/.test(message.text()))
        report.errors.push({ route: current, message: message.text().split('\n')[0] });
    }
  });
  await page.goto(origin + '/?appearance=light#/dashboard/start');
  await expect(page.getByTestId('workbench')).toBeVisible();
  const routes = await page.evaluate(() =>
    window.vm.$router
      .getRoutes()
      .filter((r) => r.components?.default && !r.children.length && !r.redirect && !r.path.includes(':'))
      .map((r) => ({ path: r.path, name: String(r.name) })),
  );
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    for (const route of routes.filter((r) => r.name !== 'login')) {
      current = route.path;
      await page.evaluate((path) => window.vm.$router.push(path), route.path);
      await page.waitForTimeout(250);
      const result = await page.evaluate(() => ({
        h1: [...document.querySelectorAll('h1')]
          .filter((el) => el.getClientRects().length)
          .map((el) => el.textContent.trim()),
        overflow: document.documentElement.scrollWidth > innerWidth + 2,
        rendered: !!document.querySelector('.console-page > *, .security-screen, .help-document'),
      }));
      report.routes.push({ ...route, width, ...result });
    }
  }
  current = 'mobile navigation';
  await page.goto(origin + '/?appearance=light#/dashboard/start');
  for (const width of [1024, 820, 768, 390, 320]) {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1024 });
    await expect(page.getByTestId('workbench')).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBeTruthy();
    report.checks.push(`workbench fits ${width}px`);
  }
  await page.getByRole('button', { name: '展开或收起导航' }).click();
  await expect(page.locator('.console-sidebar')).toBeVisible();
  await expect(page.locator('.console-body')).toHaveAttribute('inert', '');
  await page.getByRole('textbox', { name: '查找功能' }).fill('证书');
  await expect(page.locator('.nav-group > a').first()).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.locator('.console-sidebar')).toBeHidden();
  await expect(page.locator('.sidebar-toggle')).toBeFocused();
  report.checks.push('mobile menu search, focus, Escape and inert background');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole('button', { name: '系统设置', exact: true }).click();
  const preferences = page.locator('.console-preferences');
  await preferences.locator('label:has(input[value="dark"])').click();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await preferences.locator('.el-drawer__close-btn').click();
  await page.evaluate(() => history.replaceState({}, '', location.pathname + location.hash));
  await page.reload();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await page.getByRole('button', { name: '系统设置', exact: true }).click();
  await preferences.locator('label:has(input[value="auto"])').click();
  await expect(page.locator('html')).not.toHaveClass(/dark/);
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(page.locator('html')).toHaveClass(/dark/);
  await preferences.locator('.el-drawer__close-btn').click();
  report.checks.push('theme persists and system appearance changes update the console');
  await page.keyboard.press('Control+k');
  const quickSearch = page.getByRole('textbox', { name: '输入功能名称' });
  await expect(quickSearch).toBeFocused();
  await quickSearch.fill('证书');
  await quickSearch.press('ArrowDown');
  await expect(page.locator('.search-results a').first()).toBeFocused();
  await page.keyboard.press('Escape');
  report.checks.push('command search opens by keyboard and focuses matching permitted routes');
  current = 'login';
  await page.setViewportSize({ width: 320, height: 844 });
  await page.goto(origin + '/?appearance=light#/login');
  await expect(page.locator('input[autocomplete="username"]')).toBeVisible();
  const password = page.locator('input[autocomplete="current-password"]');
  await expect(password).toHaveAttribute('type', 'password');
  await page.getByRole('button', { name: '显示密码' }).click();
  await expect(password).toHaveAttribute('type', 'text');
  await expect(page.getByRole('button', { name: '隐藏密码' })).toBeFocused();
  expect(await page.locator('h1').count()).toBe(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBeTruthy();
  report.checks.push('labeled login fields and keyboard-operable password visibility');
  await context.close();
} finally {
  await browser.close();
  fs.writeFileSync(new URL('browser-report.json', output), JSON.stringify(report, null, 2));
}
const faults = report.routes.filter((route) => !route.rendered || route.h1.length !== 1 || route.overflow);
console.log(
  JSON.stringify({ routes: report.routes.length, checks: report.checks, faults, errors: report.errors }, null, 2),
);
if (faults.length || report.errors.length) process.exitCode = 1;
