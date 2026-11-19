// Reproduce a stale startup CPU peak with WebSocket pushes disconnected.
// Run against the isolated console fixture, never a live management API.
import { chromium, expect } from '@playwright/test';
import { handleTransportHandshake, routeSession } from './transport-fixture.mjs';

const origin = process.env.CONSOLE_TEST_ORIGIN || 'http://127.0.0.1:3118';
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw new Error('Use a local isolated fixture');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let releasePending;
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  await context.addInitScript(() => {
    localStorage.setItem('access_token', 'isolated-monitor-verification');
    localStorage.setItem('lang', 'zh_CN');
  });
  await context.routeWebSocket('**/api/**', ws => ws.close());
  let requests = 0;
  let active = 0;
  let maxActive = 0;
  let mode = 'normal';
  let cpu = 8;
  await context.route('**/api/**', async route => {
    if (await handleTransportHandshake(route)) return;
    const endpoint = new URL(route.request().url()).pathname.split('/api/v1/')[1] || '';
    if (endpoint !== 'monitor/system_info') {
      return route.fulfill({ json: { code: 1, data: routeSession(route).encrypt('{}'), msg: 'Isolated monitor fixture' } });
    }
    requests++;
    active++;
    maxActive = Math.max(maxActive, active);
    const sample = requests === 1 ? 100 : cpu;
    const failure = mode === 'error';
    try {
      if (mode === 'pending') await new Promise(resolve => { releasePending = resolve; });
      await route.fulfill({ json: {
        code: failure ? 1 : 0,
        msg: failure ? 'Simulated monitor outage' : '',
        data: routeSession(route).encrypt(JSON.stringify({ cpu: { usage_percent: sample }, memory: { usage_percent: 32 }, disk: [] })),
      } });
    } finally { active--; }
  });
  const page = await context.newPage();
  await page.goto(origin + '/#/waf/wafvisitlog');
  const trigger = page.locator('.system-monitor-trigger');
  const reading = trigger.locator('.monitor-item-inline .value').first();
  await expect(reading).toHaveText('100%');
  await expect(reading).toHaveText('8%', { timeout: 15000 });
  console.log('PASS: CPU recovers from a 100% startup sample without WebSocket pushes');

  mode = 'pending';
  cpu = 17;
  await trigger.click();
  await expect.poll(() => active).toBe(1);
  const pendingCount = requests;
  await page.waitForTimeout(11000);
  expect(requests).toBe(pendingCount);
  expect(maxActive).toBe(1);
  mode = 'normal';
  releasePending();
  await expect(reading).toHaveText('17%');
  console.log('PASS: opening the panel refreshes; slow samples never overlap');

  const panel = page.locator('.system-monitor-panel:visible');
  mode = 'error';
  await panel.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(reading).toHaveText('—');
  mode = 'pending';
  cpu = 9;
  await panel.getByRole('button', { name: '重试', exact: true }).click();
  await expect.poll(() => active).toBe(1);
  await expect(reading).toHaveText('—');
  mode = 'normal';
  releasePending();
  await expect(reading).toHaveText('9%');
  console.log('PASS: failures and pending retries do not present the old CPU as current');

  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
    document.dispatchEvent(new Event('visibilitychange'));
  });
  const hiddenCount = requests;
  await page.waitForTimeout(11000);
  expect(requests).toBe(hiddenCount);
  cpu = 12;
  await page.evaluate(() => {
    delete document.visibilityState;
    document.dispatchEvent(new Event('visibilitychange'));
  });
  await expect(reading).toHaveText('12%');
  console.log('PASS: hidden tabs pause polling and returning tabs refresh immediately');

  mode = 'pending';
  cpu = 99;
  await panel.getByRole('button', { name: '刷新', exact: true }).click();
  await expect.poll(() => active).toBe(1);
  await page.evaluate(() => window.vm.$router.push('/login'));
  await expect(trigger).toHaveCount(0);
  const afterUnmount = requests;
  releasePending();
  await expect.poll(() => active).toBe(0);
  await page.waitForTimeout(11000);
  expect(requests).toBe(afterUnmount);
  const stored = await page.evaluate(() => window.vm.$store.state.stats.currentSystemMonitor.cpu.usage_percent);
  expect(stored).toBe(12);
  console.log('PASS: unmount clears polling and ignores outstanding responses');
} finally {
  releasePending?.();
  await browser.close();
}
