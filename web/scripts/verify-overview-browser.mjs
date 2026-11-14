import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';
import { verifyThemeSettings, verifySystemMonitor } from './verify-theme-ui.mjs';

const origin = process.env.OVERVIEW_TEST_ORIGIN || 'http://127.0.0.1:3001';
const live = process.env.OVERVIEW_LIVE === '1';
const output = new URL('../../artifacts/', import.meta.url);
fs.mkdirSync(output, { recursive: true });
const apiKey = live
  ? fs.readFileSync(new URL('../../.cache/screen-preview/data/.keys/cli_api_key', import.meta.url), 'utf8').trim()
  : '';
const endpoints = [
  'wafstatsiteoverview',
  'wafstatsumdayrange',
  'wafstatsumdaytopiprange',
  'analysis/wafanalysisdaycountryrange',
  'wafstatqpstrend',
];
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1920, height: 1200 }, colorScheme: 'dark' });
const report = { live, errors: [], checks: [], requests: [] };
let mode = 'normal';
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-overview-verification');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const request = route.request(),
    url = new URL(request.url()),
    path = url.pathname.split('/api/v1/')[1] || '';
  if (path.includes('handshake')) return route.fulfill({ json: { code: 1, msg: 'Isolated legacy transport test' } });
  if (path === 'sysinfo/checkversion') return route.fulfill({ json: { code: 1, msg: 'No update in test' } });
  let data = { list: [], total: 0 };
  if (path === 'monitor/system_info')
    data = {
      cpu: { usage_percent: 23, model_name: 'UI verification CPU', cores: 8 },
      memory: { usage_percent: 48, total: '16 GB', used: '7.7 GB' },
      disk: Array.from({ length: 12 }, (_, i) => ({ mount_point: '/disk-' + i, usage_percent: 15 + i * 5 })),
    };
  if (endpoints.includes(path)) {
    report.requests.push({ path, params: Object.fromEntries(url.searchParams) });
    if (mode === 'error') return route.fulfill({ status: 503, json: { code: -6, msg: 'Test outage' } });
    if (live) {
      const response = await fetch('http://127.0.0.1:26666' + url.pathname + url.search, {
        headers: { 'X-API-Key': apiKey },
        signal: AbortSignal.timeout(15000),
      });
      const result = await response.json();
      if (result.code !== 0) throw new Error('Live statistics failed: ' + path);
      return route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(result.data)) } });
    }
    const empty = mode === 'empty';
    if (path === 'wafstatsiteoverview')
      data = {
        total_ip: empty ? 0 : 572,
        total_uv: empty ? 0 : 572,
        total_pv: empty ? 0 : 20100,
        total_attack: empty ? 0 : 8600,
        total_in_mb: empty ? 0 : 124.7,
        total_out_mb: empty ? 0 : 3400,
        site_list: empty
          ? []
          : [
              {
                host_code: 'demo-site',
                host: 'demo.example.test',
                host_remark: 'Test site',
                total_count: 20100,
                attack_count: 8600,
                normal_count: 11500,
                traffic_in_mb: 124.7,
                traffic_out_mb: 3400,
                uv_count: 572,
                ip_count: 572,
                avg_time_ms: 12.5,
              },
            ],
      };
    if (path === 'wafstatsumdayrange') {
      const start = url.searchParams.get('start_day'),
        end = url.searchParams.get('end_day');
      const begin = new Date(`${start.slice(0, 4)}-${start.slice(4, 6)}-${start.slice(6)}T00:00:00`);
      const last = new Date(`${end.slice(0, 4)}-${end.slice(4, 6)}-${end.slice(6)}T00:00:00`);
      const attack = {},
        allowed = {};
      let i = 0;
      while (begin <= last) {
        const key = [
          begin.getFullYear(),
          String(begin.getMonth() + 1).padStart(2, '0'),
          String(begin.getDate()).padStart(2, '0'),
        ].join('');
        attack[key] = empty ? 0 : [150, 300, 180, 45, 500, 330, 2200][i % 7];
        allowed[key] = empty ? 0 : [700, 1300, 620, 550, 1400, 700, 5000][i % 7];
        begin.setDate(begin.getDate() + 1);
        i++;
      }
      data = { AttackCountOfRange: attack, NormalCountOfRange: allowed };
    }
    if (path === 'analysis/wafanalysisdaycountryrange')
      data = empty
        ? []
        : [
            { name: '中国', value: url.searchParams.get('attack_type') ? 5200 : 19000 },
            { name: '美国', value: 570 },
            { name: '荷兰', value: 216 },
            { name: '比利时', value: 128 },
            { name: '新加坡', value: 29 },
            { name: '英国', value: 29 },
            { name: '法国', value: 23 },
          ];
    if (path === 'wafstatsumdaytopiprange')
      data = {
        AttackIPOfRange: empty
          ? []
          : Array.from({ length: 7 }, (_, i) => ({
              ip: '203.0.113.' + (10 + i),
              count: 600 - i * 60,
              ip_belong: '中国',
              ip_tags: [],
            })),
        NormalIPOfRange: empty
          ? []
          : Array.from({ length: 7 }, (_, i) => ({
              ip: '198.51.100.' + (10 + i),
              count: 2000 - i * 100,
              ip_belong: '美国',
              ip_tags: [],
            })),
      };
    if (path === 'wafstatqpstrend')
      data = {
        Current: empty ? 0 : 28,
        Max: empty ? 0 : 75,
        Points: Array.from({ length: 60 }, (_, i) => ({
          T: Math.floor(Date.now() / 1000) - 59 + i,
          V: empty ? 0 : i % 13 === 0 ? 75 : 10 + (i % 25),
        })),
      };
  }
  if (path === 'upgradenotice/summary') data = { pending_count: 0, popup_items: [], need_popup: false };
  if (path === 'statsysinfo') data = { is_default_account: false, is_empty_otp: false, is_empty_host: false };
  await route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)) } });
});
const page = await context.newPage();
page.on('pageerror', (e) => report.errors.push(e.message));
const root = page.locator('.traffic-analysis');
async function shot(name) {
  const size = page.viewportSize();
  await page.setViewportSize({ width: size.width, height: 2300 });
  await root.screenshot({ path: fileURLToPath(new URL(name, output)) });
  await page.setViewportSize(size);
}
async function loaded() {
  await expect(root).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('.updated-at')).toContainText('更新于');
}
async function noOverflow(width) {
  await page.setViewportSize({ width, height: 1200 });
  await expect
    .poll(() => root.evaluate((el) => el.scrollWidth <= el.clientWidth + 1), { message: `Overflow at ${width}px` })
    .toBe(true);
  const bad = await root.evaluate(
    (el) =>
      [...el.querySelectorAll('.traffic-panel, .metric-group')].filter(
        (p) => p.getBoundingClientRect().right > el.getBoundingClientRect().right + 1,
      ).length,
  );
  expect(bad).toBe(0);
  report.checks.push(`No panel overflow at ${width}px`);
}
try {
  await page.goto(origin + '/#/dashboard/base');
  await expect(root).toBeVisible();
  await loaded();
  const versionDialog = page.getByRole('dialog', { name: '有新版本啦' });
  if (await versionDialog.isVisible()) await versionDialog.getByRole('button', { name: '取消', exact: true }).click();
  await expect(page.getByRole('heading', { name: '来源地区', exact: true })).toBeVisible();
  await verifyThemeSettings(page, root, output, report);
  await verifySystemMonitor(page, output, report);
  await expect(page.locator('.traffic-map, .geography-panel, .map-switches')).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^(3D|2D|世界|中国|放大地图|复位地图)$/ })).toHaveCount(0);
  expect(await page.locator('.traffic-chart canvas').count()).toBe(5);
  expect(await page.locator('.primary-metrics .metric-number').allTextContents()).not.toContain('—');
  await shot(live ? 'overview-live-desktop.png' : 'overview-desktop.png');
  report.checks.push('Standalone source-region ranking replaces geography; five traffic charts still render');

  if (!live) {
    await expect(page.locator('.region-row')).toHaveCount(7);
    await expect(page.locator('.region-row').first()).toContainText('19,000');
    await expect(page.locator('.region-ranking')).toContainText('新加坡');
  }
  await page.getByRole('button', { name: '仅拦截', exact: true }).click();
  await expect(page.locator('.ranking-caption')).toContainText('拦截次数');
  if (!live) await expect(page.locator('.region-row').first()).toContainText('5,200');
  await shot(live ? 'overview-live-regions-blocked.png' : 'overview-regions-blocked.png');
  await page.getByRole('button', { name: '访问', exact: true }).click();
  await expect(page.locator('.ranking-caption')).toContainText('请求次数');
  report.checks.push('Source regions show requests and blocked rankings, including countries without map geometry');

  await page.getByRole('combobox', { name: '统计时间范围' }).selectOption('30d');
  await loaded();
  const dates = (await page.locator('.traffic-status').innerText()).match(/\d{4}-\d{2}-\d{2}/g);
  expect(new Date(dates[1]) - new Date(dates[0])).toBe(29 * 86400000);
  await page.getByRole('combobox', { name: '统计时间范围' }).selectOption('custom');
  await page.getByLabel('开始日期', { exact: true }).fill(dates[1]);
  await page.getByLabel('结束日期', { exact: true }).fill(dates[1]);
  await page.getByRole('button', { name: '应用', exact: true }).click();
  await loaded();
  const latest = report.requests.filter((r) => r.path === 'wafstatsumdayrange').at(-1);
  expect(latest.params.start_day).toBe(latest.params.end_day);
  await page.getByRole('combobox', { name: '统计时间范围' }).selectOption('7d');
  await loaded();
  report.checks.push('Presets and custom date range reach statistics APIs');

  for (const width of [1440, 1024, 768, 390]) await noOverflow(width);
  await shot(live ? 'overview-live-mobile.png' : 'overview-mobile.png');
  await page.setViewportSize({ width: 1920, height: 1200 });

  if (!live) {
    const before = await page.locator('.primary-metrics .metric-number').first().textContent();
    mode = 'error';
    await page.getByRole('button', { name: '刷新流量分析' }).click();
    await expect(page.locator('.load-warning')).toBeVisible();
    expect(await page.locator('.primary-metrics .metric-number').first().textContent()).toBe(before);
    mode = 'empty';
    await page.getByRole('button', { name: '刷新流量分析' }).click();
    await loaded();
    await expect(page.locator('.primary-metrics .metric-number').first()).toHaveText('0');
    await expect(page.locator('.region-ranking .panel-empty')).toHaveText('暂无地区数据');
    expect(await root.innerText()).not.toMatch(/NaN|Infinity|undefined/);
    await shot('overview-empty.png');
    mode = 'normal';
    await page.getByRole('button', { name: '刷新流量分析' }).click();
    await loaded();
    report.checks.push('Failed refresh preserves marked stale data; empty responses show truthful zeros');
  }
  await page.locator('.ip-panel .log-button').first().click();
  await expect(page).toHaveURL(/wafvisitlog/);
  expect(page.url()).toContain('action=');
  expect(page.url()).toContain('src_ip=');
  expect(page.url()).toContain('date_begin=');
  report.checks.push('IP drilldown carries IP, action and selected dates');
  await page.goto(origin + '/#/dashboard/base');
  await loaded();
  await expect(page.getByRole('heading', { name: '来源地区', exact: true })).toBeVisible();
  expect(report.errors).toEqual([]);
  fs.writeFileSync(
    new URL(live ? 'overview-live-verification.json' : 'overview-verification.json', output),
    JSON.stringify(report, null, 2),
  );
  console.log(JSON.stringify({ live, checks: report.checks, errors: report.errors }, null, 2));
} catch (error) {
  console.error('Page errors:', report.errors);
  await page.screenshot({ path: fileURLToPath(new URL('overview-test-failure.png', output)), fullPage: true });
  throw error;
} finally {
  await browser.close();
}
