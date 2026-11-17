// Isolated encrypted API fixtures exercise the real log-detail UI.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const origin = process.env.LOG_TEST_ORIGIN || 'http://127.0.0.1:3002';
const output = path.join(root, 'artifacts', 'request-diagnostics');
fs.mkdirSync(output, { recursive: true });
const report = { checks: [], layouts: [], errors: [] };
let state = 'available', failDetail = false, requests = 0;
const sample = {
  req_uuid: 'ssrf-request', action: '放行', rule: 'ssrf', log_only_mode: 1, status_code: 200,
  method: 'POST', url: '/webapi/entry.cgi?token=PRIVATE-QUERY', host_code: 'pipeline-15000',
  host: 'http://nas.example.test', src_ip: '192.168.1.20', net_src_ip: '192.168.1.20',
  create_time: '2026-11-17 12:00:00', shard_name: 'local_log.db', user_agent: 'Synology Photos',
  header: 'Cookie: PRIVATE-COOKIE', cookies: 'PRIVATE-COOKIE', body: 'token=PRIVATE-BODY',
  time_spent: 20, risk_level: 3, guest_identification: '未识别访客',
};
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, permissions: ['clipboard-read', 'clipboard-write'] });
await context.addInitScript(() => { localStorage.setItem('access_token', 'isolated-diagnostics'); localStorage.setItem('lang', 'zh_CN'); });
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const url = new URL(route.request().url());
  const endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint)) return fail('Isolated fixture');
  if (endpoint === 'waflog/attack/detail') {
    requests++;
    const id = url.searchParams.get('req_uuid');
    if (failDetail) return fail('没有查看这条请求的权限');
    if (id === 'slow-request') await new Promise((resolve) => setTimeout(resolve, 600));
    const category = id === 'csrf-request' ? 'csrf' : 'ssrf';
    if (id === 'host-rejected') return ok({ ...sample, req_uuid: id, action: '阻止', rule: 'host_not_allowed', log_only_mode: 0, status_code: 403, diagnostics: { state: 'available', hits: [], body_truncated: false } });
    return ok({ ...sample, req_uuid: id, rule: category, diagnostics: {
      state, body_truncated: false,
      hits: state === 'available' ? [{ category, rule_id: category, source: category === 'csrf' ? 'header' : 'cookie', code: category === 'csrf' ? 'missing_origin_observe' : 'internal_url', observe: true, severity: 'high' }] : [],
    } });
  }
  if (endpoint === 'waflog/attack/list') return ok({ list: [sample], total: 1, shards: [], took_ms: 1 });
  if (endpoint === 'wafhost/host/allhost') return ok([{ value: sample.host_code, label: '群晖 · :15000', source: 'pipeline', port: 15000 }]);
  if (endpoint === 'waflog/attack/allsharedb') return ok([{ label: '实时库', value: 'local_log.db' }]);
  if (endpoint === 'uipreference/get') return ok({});
  if (endpoint === 'waflog/attack/alliptag') return ok([]);
  if (endpoint === 'waflog/exports/capabilities') return ok({ allowed: false, enabled: false, formats: [] });
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
const diagnostics = () => page.getByTestId('request-diagnostics');
const open = async (id = 'ssrf-request', theme = 'light') => {
  const target = `${origin}/?appearance=${theme}#/waf/wafattacklogdetail?req_uuid=${id}%23local_log.db`;
  if (page.url() === target) await page.reload();
  else await page.goto(target);
  await expect(diagnostics()).toBeVisible();
};
try {
  await page.goto(`${origin}/#/waf/wafvisitlog`);
  const row = page.getByTestId('visit-log-page').locator('.el-table__body-wrapper tbody tr').first();
  await expect(row).toBeVisible();
  await expect(row).toContainText('仅观察');
  await expect(row).not.toContainText('异常');
  await row.getByRole('button', { name: '详情', exact: true }).click();
  await expect(diagnostics()).toContainText('Cookie');
  await expect(diagnostics()).toContainText('internal_url');
  await diagnostics().getByRole('button', { name: '复制排查信息' }).click();
  const copied = await page.evaluate(() => navigator.clipboard.readText());
  expect(copied).toContain('ssrf-request');
  expect(copied).toContain('internal_url');
  expect(copied).not.toContain('PRIVATE-');
  expect(copied).not.toContain('192.168.1.20');
  report.checks.push('request list opens diagnostics and copies metadata without request values');

  await open('csrf-request');
  await expect(page.locator('.vd-rule-tag')).toHaveText('观察记录 · csrf');
  await expect(page.locator('.vd-hero-title')).toContainText('仅观察，已放行');
  await expect(page.locator('.vd-rule-tag')).not.toHaveClass(/el-tag--danger/);
  await expect(diagnostics()).toContainText('原生 App');
  await expect(diagnostics()).toContainText('仅观察，未由此项拦截');
  report.checks.push('CSRF missing-origin observation explains native-app compatibility');
  await open('host-rejected');
  await expect(diagnostics()).toContainText('已在主机校验阶段拒绝');
  await expect(page.getByTestId('visitor-identity')).toHaveText('未识别访客');
  await expect(page.locator('.vd-hero')).toHaveClass(/vd-hero--error/);
  await expect(page.locator('.detail-base')).not.toContainText('正常访客');
  report.checks.push('host rejection keeps blocked outcome and never labels an unknown visitor as normal');
  for (const missingState of ['missing', 'unavailable', 'invalid']) {
    state = missingState;
    await open();
    await expect(diagnostics().locator('li')).toHaveCount(0);
    await expect(diagnostics().getByRole('button', { name: '重新读取' })).toBeVisible();
    state = 'available';
    await diagnostics().getByRole('button', { name: '重新读取' }).click();
    await expect(diagnostics()).toContainText('internal_url');
  }
  report.checks.push('missing, invalid and unavailable records show honest states and recover on reload');

  failDetail = true;
  await page.goto(`${origin}/#/waf/wafattacklogdetail?req_uuid=ssrf-request`);
  await expect(page.getByText('没有查看这条请求的权限', { exact: true }).first()).toBeVisible();
  await expect(diagnostics()).toHaveCount(0);
  failDetail = false;
  await page.getByRole('button', { name: '重新读取', exact: true }).click();
  await expect(diagnostics()).toBeVisible();
  report.checks.push('denied or failed detail reads expose no stale diagnostics and support retry');

  const before = requests;
  await page.evaluate(() => { window.location.hash = '/waf/wafattacklogdetail?req_uuid=slow-request'; });
  await expect.poll(() => requests).toBeGreaterThan(before);
  await page.evaluate(() => { window.location.hash = '/waf/wafattacklogdetail?req_uuid=csrf-request'; });
  await expect(diagnostics()).toContainText('missing_origin_observe');
  await page.waitForTimeout(700);
  await expect(diagnostics()).toContainText('missing_origin_observe');
  report.checks.push('older request responses cannot replace a newer request diagnostic');

  for (const theme of ['light', 'dark']) {
    await page.evaluate((mode) => localStorage.setItem('homewaf.console.preferences', JSON.stringify({ mode })), theme);
    await open('csrf-request', theme);
    await expect(page.locator('html')).toHaveAttribute('theme-mode', theme);
    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      await diagnostics().scrollIntoViewIfNeeded();
      const layout = await page.evaluate(() => {
        const panel = document.querySelector('[data-testid="request-diagnostics"]');
        const button = [...panel.querySelectorAll('button')].find((item) => item.textContent.includes('复制排查信息'));
        const rect = panel.getBoundingClientRect(), target = button.getBoundingClientRect();
        return { mode: document.documentElement.getAttribute('theme-mode'), background: getComputedStyle(panel).backgroundColor, overflow: document.documentElement.scrollWidth > innerWidth + 2, left: rect.left, right: rect.right, buttonWidth: target.width, buttonHeight: target.height };
      });
      expect(layout.overflow).toBe(false);
      expect(layout.left).toBeGreaterThanOrEqual(0);
      expect(layout.right).toBeLessThanOrEqual(width + 1);
      expect(layout.buttonHeight).toBeGreaterThanOrEqual(24);
      await diagnostics().screenshot({ path: path.join(output, `${theme}-${width}.png`) });
      report.layouts.push({ theme, width, ...layout });
    }
  }
  expect(report.errors).toEqual([]);
  console.log(JSON.stringify(report, null, 2));
} finally {
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
  await browser.close();
}
