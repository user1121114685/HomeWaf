// Isolated journey verification; management requests never reach a running WAF.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';
const root = fileURLToPath(new URL('../..', import.meta.url));
const origin = process.env.DIAGNOSTICS_TEST_ORIGIN || 'http://127.0.0.1:3118';
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw new Error('An isolated local fixture is required');
const output = path.join(root, 'artifacts', 'service-diagnostics');
fs.mkdirSync(output, { recursive: true });
const templates = JSON.parse(
  execFileSync('go', ['run', './scripts/pipeline-fixtures'], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, CGO_ENABLED: '0' },
  }),
);
const clone = (v) => JSON.parse(JSON.stringify(v));
const configs = [5000, 8080].map((port, index) => ({
  schema_version: 1,
  revision: 1,
  port,
  name: index ? '个人博客' : '家庭相册',
  protocol: 'http',
  host_code: `pipeline-${port}`,
  upstream: `http://192.0.2.${20 + index}:${port}`,
  host_names: [],
  listen_enabled: !index,
  policy: clone(templates.find((item) => item.id === 'public-site').policy),
  template: { id: 'public-site', version: 1 },
}));
let state = [
  {
    id: 'shared-ban',
    kind: 'ban',
    port_id: 'pipeline-5000',
    prefix: '198.51.100.8/32',
    scope: 'global',
    reason: '扫描器命中',
    expires_at: Math.floor(Date.now() / 1000) + 3600,
  },
  {
    id: 'local-drop',
    kind: 'drop',
    port_id: 'pipeline-5000',
    prefix: '192.0.2.44/32',
    scope: 'port',
    reason: '高频异常连接',
    expires_at: Math.floor(Date.now() / 1000) + 3600,
  },
];
const trace = {
  req_id: 'request-denied',
  verdict: { action: 'deny', reason: 'blacklist:custom' },
  trace: [{ stage: 2, check: 'ip_group', verdict: { action: 'deny', reason: 'blacklist:custom' }, elapsed_us: 3 }],
};
let events = [
  {
    id: 'event-denied',
    time_ms: Date.now(),
    kind: 'request',
    req_id: 'request-denied',
    client_ip: '198.51.100.8',
    verdict: 'deny',
    reason: 'blacklist:custom',
    detail_json: JSON.stringify(trace),
  },
  {
    id: 'event-observe',
    time_ms: Date.now() - 1000,
    kind: 'request',
    req_id: 'request-observe',
    client_ip: '192.0.2.5',
    verdict: 'allow',
    observe: true,
    reason: 'sqli',
    detail_json: 'broken',
  },
  {
    id: 'event-state',
    time_ms: Date.now() - 2000,
    kind: 'state',
    client_ip: '192.0.2.6/32',
    verdict: 'ban',
    reason: 'released',
    detail_json: JSON.stringify({ event: 'released', record: { reason: '扫描器命中' } }),
  },
];
const report = { checks: [], errors: [], writes: [], requests: [], layouts: [] };
let failure = '',
  failWrite = false,
  delayedPort = 0,
  missing = false;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
await context.addInitScript(() => {
  if (!['http:', 'https:'].includes(location.protocol)) return;
  localStorage.setItem('access_token', 'isolated-diagnostic-fixture');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/*', (socket) => socket.close());
await context.route('**/*', async (route) => {
  const request = route.request(),
    url = new URL(request.url());
  if (url.origin !== origin) return route.abort();
  if (!url.pathname.startsWith('/api/')) return route.continue();
  const endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint))
    return fail('Isolated fixture');
  let input = {};
  if (request.postData()) {
    try {
      input = JSON.parse(AesDecrypt(request.postData()));
    } catch {
      input = JSON.parse(request.postData());
    }
  }
  const port = Number(url.searchParams.get('port') || input.port || input.config?.port);
  report.requests.push({ endpoint, port });
  if (endpoint === 'pipeline/' + failure)
    return fail(failure === 'overview' ? '没有访问此服务的权限，请联系管理员。' : '测试读取失败，请重试');
  if (endpoint === 'pipeline/overview')
    return ok({
      ports: missing
        ? []
        : configs.map((config) => ({
            config,
            listener: { running: config.listen_enabled },
            locked: false,
            stats: { today_hits: 3, today_denied: 1 },
          })),
      issues: {},
    });
  if (endpoint === 'pipeline/templates') return ok(templates);
  if (
    endpoint === 'pipeline/migration/legacy' ||
    endpoint === 'pipeline/regions' ||
    endpoint === 'wafhost/ipgroup/options'
  )
    return ok([]);
  if (endpoint === 'pipeline/logs') {
    const data = port === 5000 ? clone(events) : [];
    if (delayedPort === port) await new Promise((resolve) => setTimeout(resolve, 900));
    return ok(data);
  }
  if (endpoint === 'pipeline/stats')
    return ok({
      port: {
        today_requests: port === 5000 ? 1250 : 0,
        today_denied: 1,
        today_hits: 3,
        requests: 1400,
        denied: 6,
        hits: 15,
      },
      protocol: { connections: 2, tls_errors: 0 },
      upstream: { requests_60s: 12, failures_60s: 0 },
      punishment: { queued: 2, capacity: 4096, quota_dropped: 0, persist_errors: 0 },
      log_queue: { queued: 0, capacity: 8192, dropped_low: 3 },
      admission: { in_flight: 2, capacity: 256, rejected: 0 },
    });
  if (endpoint === 'pipeline/state') return ok(port === 5000 ? state : []);
  if (endpoint === 'pipeline/state/manage') {
    report.writes.push(clone(input));
    if (failWrite) return fail('测试写入失败，未更新处罚');
    if (input.port !== 5000) return fail('Wrong service');
    const record = state.find((row) => row.id === input.id);
    if (!record) return fail('Record not found');
    if (input.action === 'permanent') record.expires_at = 0;
    else {
      if (input.action === 'whitelist')
        configs[0].policy.ip_group.entries.push({
          id: 'trusted-test',
          type: 'allow',
          trust: 'trusted',
          value: record.prefix,
        });
      state = state.filter((row) => row.id !== input.id);
    }
    await new Promise((resolve) => setTimeout(resolve, 150));
    return ok({});
  }
  if (endpoint === 'pipeline/ports/save') {
    report.writes.push({ action: 'save', config: clone(input.config) });
    const index = configs.findIndex((config) => config.port === input.config.port);
    configs[index] = { ...clone(input.config), revision: configs[index].revision + 1 };
    return ok(configs[index]);
  }
  if (endpoint === 'pipeline/defense') return ok({ configured: true, mode: 'reset', enabled: true });
  if (endpoint === 'pipeline/defense/capabilities') return ok({ reset_supported: true, drop_supported: false });
  if (endpoint === 'pipeline/honeypot') return ok({ shared: { configured: false } });
  if (endpoint === 'pipeline/ports/check') return ok({ warnings: [] });
  if (endpoint.startsWith('pipeline/gate/')) return ok([]);
  if (endpoint === 'upgradenotice/summary') return ok({ pending_count: 0, popup_items: [], need_popup: false });
  if (endpoint === 'statsysinfo') return ok({ is_default_account: false, is_empty_otp: false, is_empty_host: false });
  if (endpoint === 'wafhost/host/allhost')
    return ok(configs.map((config) => ({ value: config.host_code, label: config.name, source: 'pipeline' })));
  if (endpoint === 'wafstatsiteoverview')
    return ok({
      total_ip: 0,
      total_uv: 0,
      total_pv: 0,
      total_attack: 0,
      total_in_mb: 0,
      total_out_mb: 0,
      site_list: [],
    });
  if (endpoint === 'wafstatsumdayrange') return ok({ AttackCountOfRange: {}, NormalCountOfRange: {} });
  if (endpoint === 'wafstatsumdaytopiprange') return ok({ AttackIPOfRange: [], NormalIPOfRange: [] });
  if (endpoint === 'analysis/wafanalysisdaycountryrange') return ok([]);
  if (endpoint === 'wafstatqpstrend') return ok({ Current: 0, Max: 0, Points: [] });
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
const diagnostics = page.getByTestId('service-diagnostics');
const punishmentPage = page.getByTestId('service-punishments-page');
const punishments = page.getByTestId('service-punishments');
async function goto(port = 5000, tab = 'events') {
  await page.goto('about:blank');
  await page.goto(`${origin}/#/waf-pipeline/ports/${port}/diagnostics?tab=${tab}`);
  await expect(page.getByTestId('service-diagnostics')).toBeVisible();
}
async function gotoPunishments(port = 5000, returnTo = '/waf-pipeline/ports') {
  await page.goto('about:blank');
  await page.goto(
    `${origin}/#/waf-pipeline/ports/${port}/punishments?returnTo=${encodeURIComponent(returnTo)}`,
  );
  await expect(punishmentPage).toBeVisible();
  await expect(punishments).toBeVisible();
}
async function confirm(action) {
  const box = page.locator('.el-message-box');
  await expect(box).toBeVisible();
  await box.getByRole('button', { name: action, exact: true }).click();
}
async function check(name, fn) {
  await fn();
  report.checks.push(name);
  console.log('PASS ' + name);
}
try {
  await check('home, dashboard root, legacy workbench and brand resolve to traffic overview', async () => {
    for (const destination of ['/', '/dashboard', '/dashboard/start']) {
      await page.goto(origin + '/#' + destination);
      await expect(page).toHaveURL(/#\/dashboard\/base$/);
      await expect(page.locator('.dashboard-page')).toBeVisible();
    }
    await expect(page.locator('.workspace-nav').getByRole('link', { name: '工作台', exact: true })).toHaveCount(0);
    await expect(page.locator('.console-brand')).toHaveAttribute('href', '#/dashboard/base');
  });
  await check('service list exposes current punishments and keeps logs and diagnostics in the more menu', async () => {
    await page.goto(origin + '/#/waf-pipeline/ports');
    await expect(page.getByRole('button', { name: '当前处罚', exact: true })).toHaveCount(2);
    await expect(page.getByRole('button', { name: '运行诊断', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '访问记录', exact: true })).toHaveCount(0);
    await page.getByRole('textbox', { name: '搜索服务', exact: true }).fill('家庭');
    const familyCard = page.locator('.service-card').filter({ hasText: '家庭相册' });
    await familyCard.getByRole('button', { name: '更多服务操作' }).click();
    await expect(page.getByRole('menuitem', { name: '查看访问记录', exact: true })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: '运行诊断', exact: true })).toBeVisible();
    await page.getByRole('menuitem', { name: '运行诊断', exact: true }).click();
    await expect(page).toHaveURL(/ports\/5000\/diagnostics/);
    await expect(page.getByRole('heading', { name: '家庭相册', exact: true })).toBeVisible();
    await page.getByRole('button', { name: '返回我的服务', exact: true }).click();
    await expect(page.getByRole('textbox', { name: '搜索服务', exact: true })).toHaveValue('家庭');
    await familyCard.getByRole('button', { name: '当前处罚', exact: true }).click();
    await expect(page).toHaveURL(/ports\/5000\/punishments/);
    await expect(punishmentPage.getByRole('heading', { name: '家庭相册 · 当前处罚', exact: true })).toBeVisible();
    await page.getByRole('button', { name: '返回我的服务', exact: true }).click();
    await expect(page.getByRole('textbox', { name: '搜索服务', exact: true })).toHaveValue('家庭');
    await page.getByRole('button', { name: '表格与批量管理', exact: true }).click();
    const familyRow = page.getByTestId('ports-table').locator('.el-table__body-wrapper tbody tr').filter({ hasText: '5000' });
    await expect(familyRow.getByRole('button', { name: '当前处罚', exact: true })).toBeVisible();
    await expect(familyRow.getByRole('button', { name: '运行诊断', exact: true })).toHaveCount(0);
    await expect(familyRow.getByRole('button', { name: '访问记录', exact: true })).toHaveCount(0);
    await familyRow.getByRole('button', { name: '更多服务操作' }).click();
    await expect(page.getByRole('menuitem', { name: '查看访问记录', exact: true })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: '运行诊断', exact: true })).toBeVisible();
    await page.getByRole('menuitem', { name: '查看访问记录', exact: true }).click();
    await expect(page).toHaveURL(/host_code=pipeline-5000/);
    await goto();
    await page.goto(origin + '/#/waf-pipeline/ports?search=%E5%AE%B6%E5%BA%AD&view=table');
    await page
      .getByTestId('ports-table')
      .locator('.el-table__body-wrapper tbody tr')
      .filter({ hasText: '5000' })
      .getByRole('button', { name: '更多服务操作' })
      .click();
    await page.getByRole('menuitem', { name: '运行诊断', exact: true }).click();
    await expect(page).toHaveURL(/ports\/5000\/diagnostics/);
    await page.getByRole('button', { name: '返回我的服务', exact: true }).click();
    await expect(page.getByTestId('ports-table')).toBeVisible();
    await expect(page.getByRole('textbox', { name: '搜索服务', exact: true })).toHaveValue('家庭');
  });
  await check('event filters, readable trace and malformed detail recovery', async () => {
    await goto();
    await expect(diagnostics.getByText('1,250', { exact: false }).first()).toBeVisible();
    await diagnostics.getByRole('textbox', { name: '查找来源或原因' }).fill('198.51.100.8');
    await expect(diagnostics.locator('.events-table .el-table__row')).toHaveCount(1);
    await diagnostics
      .locator('.events-table')
      .getByRole('button', { name: '\u67e5\u770b\u8be6\u60c5', exact: true })
      .first()
      .click();
    await expect(diagnostics.locator('.events-table').getByTestId('decision-trace')).toContainText(
      '命中手动 IP 黑名单',
    );
    await diagnostics.getByRole('textbox', { name: '查找来源或原因' }).fill('no-match');
    await expect(diagnostics.getByText('没有匹配的记录，请调整筛选条件。')).toBeVisible();
    await diagnostics.getByRole('textbox', { name: '查找来源或原因' }).fill('request-observe');
    await diagnostics
      .locator('.events-table')
      .getByRole('button', { name: '\u67e5\u770b\u8be6\u60c5', exact: true })
      .first()
      .click();
    await expect(diagnostics.locator('.events-table').getByTestId('diagnostic-event-detail')).toContainText(
      '详细数据缺失或格式无法解析',
    );
    await diagnostics.getByRole('textbox', { name: '查找来源或原因' }).fill('');
  });
  await check('failed refresh preserves stale results and supports retry', async () => {
    failure = 'logs';
    await diagnostics.getByRole('button', { name: '刷新事件', exact: true }).click();
    await expect(diagnostics.getByRole('alert').filter({ hasText: '事件读取失败，当前保留上次结果' })).toBeVisible();
    await expect(diagnostics.locator('.events-table .el-table__row')).toHaveCount(3);
    failure = '';
    await diagnostics.getByRole('button', { name: '刷新事件', exact: true }).click();
    await expect(diagnostics.getByRole('alert').filter({ hasText: '事件读取失败' })).toHaveCount(0);
  });
  await check('current punishments use a dedicated service page with exact scoped writes', async () => {
    await gotoPunishments(5000, '/waf-pipeline/ports?search=家庭&view=table');
    const shared = punishments.getByRole('row').filter({ hasText: '198.51.100.8/32' });
    await shared.getByRole('button', { name: '释放', exact: true }).click();
    await expect(page.locator('.el-message-box')).toContainText('修改或释放会同时影响其他服务');
    await confirm('取消');
    expect(report.writes).toHaveLength(0);
    failWrite = true;
    await shared.getByRole('button', { name: '释放', exact: true }).click();
    await confirm('确认操作');
    await expect(punishments.getByRole('alert').filter({ hasText: '测试写入失败' })).toBeVisible();
    await expect(shared).toBeVisible();
    failWrite = false;
    await punishments.getByRole('button', { name: '刷新处罚记录' }).click();
    await shared.getByRole('button', { name: '转为永久' }).click();
    await confirm('确认操作');
    await expect(shared).toContainText('永久');
    await shared.getByRole('button', { name: '设为可信并释放' }).click();
    await expect(page.locator('.el-message-box')).toContainText('可信名单仅写入当前服务');
    await confirm('确认操作');
    await expect(shared).toHaveCount(0);
    expect(report.writes.slice(0, 3)).toEqual([
      { port: 5000, id: 'shared-ban', action: 'release' },
      { port: 5000, id: 'shared-ban', action: 'permanent' },
      { port: 5000, id: 'shared-ban', action: 'whitelist' },
    ]);
    const local = punishments.getByRole('row').filter({ hasText: '192.0.2.44/32' });
    await local.getByRole('button', { name: '释放', exact: true }).click();
    await confirm('确认操作');
    await expect(
      punishments.locator('.punishment-table').getByText('当前服务没有处罚记录', { exact: true }),
    ).toBeVisible();
    await page.goto(`${origin}/#/waf-pipeline/ports/5000/diagnostics?tab=state`);
    await expect(punishmentPage).toBeVisible();
    await expect(page).toHaveURL(/ports\/5000\/punishments/);
  });
  await check('service metrics and shared resources are separated, missing values stay unknown', async () => {
    await goto(5000, 'metrics');
    await expect(diagnostics.getByRole('tab', { name: '运行指标', exact: true })).toHaveAttribute('aria-selected', 'true');
    await expect(diagnostics.locator('[data-scope="service"]')).toHaveCount(2);
    await expect(diagnostics.locator('[data-scope="instance"]')).toContainText('多个服务共享');
    await expect(diagnostics.getByText('未提供', { exact: true }).first()).toBeVisible();
  });
  await check(
    'DSM lock identifies all accounts and releases only the selected service after confirmation',
    async () => {
      const lock = {
        id: 'synology_login_lockdown|port:5000|',
        port_id: 'port:5000',
        kind: 'synology_login_lockdown',
        scope: 'port',
        prefix: '',
        hits: 10,
        expires_at: Math.floor(Date.now() / 1000) + 10800,
      };
      state = [lock];
      await gotoPunishments();
      await punishments.getByRole('button', { name: '刷新处罚记录' }).click();
      const writesBefore = report.writes.length;
      const row = punishments.getByRole('row').filter({ hasText: '群晖登录锁定' });
      await expect(row).toContainText('所有 DSM 用户');
      await expect(row).toContainText('DSM 认证失败累计 10 次');
      await row.getByRole('button', { name: '解除锁定' }).click();
      await expect(page.locator('.el-message-box')).toContainText('恢复本服务所有用户的 DSM 登录');
      await confirm('取消');
      expect(report.writes.length).toBe(writesBefore);
      await page.setViewportSize({ width: 390, height: 844 });
      const card = punishments.locator('.punishment-cards article').filter({ hasText: '所有 DSM 用户' });
      await expect(card).toContainText('到期时间');
      await card.getByRole('button', { name: '解除锁定' }).click();
      await confirm('确认操作');
      await expect(card).toHaveCount(0);
      expect(report.writes.slice(writesBefore)).toEqual([{ port: 5000, id: lock.id, action: 'release' }]);
      await page.setViewportSize({ width: 1440, height: 1000 });
    },
  );
  await check('desktop, tablet, phone and dark mode preserve usable content', async () => {
    state = [
      {
        id: 'mobile-state',
        kind: 'ban',
        port_id: 'pipeline-5000',
        prefix: '203.0.113.50/32',
        scope: 'global',
        reason: '扫描器命中',
        expires_at: Math.floor(Date.now() / 1000) + 3600,
      },
    ];
    for (const mode of ['light', 'dark']) {
      for (const width of [1440, 768, 390]) {
        await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
        await goto(5000);
        await page.evaluate(
          (mode) => window.vm.$store.dispatch('setting/changeTheme', { ...window.vm.$store.state.setting, mode }),
          mode,
        );
        for (const tab of ['请求与事件', '运行指标']) {
          await page.getByRole('tab', { name: new RegExp('^' + tab) }).click();
          await expect
            .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
            .toBeLessThanOrEqual(2);
          if (width === 390 && tab === '请求与事件') {
            await expect(diagnostics.locator('.event-cards')).toContainText('命中手动 IP 黑名单');
            if ((await diagnostics.locator('.event-cards details').first().getAttribute('open')) === null)
              await diagnostics.locator('.event-cards summary').first().click();
            await expect(diagnostics.locator('.event-cards .mobile-trace').first()).toBeVisible();
          }
          await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 10000 });
          await page.locator('#diagnostic-service-name').focus();
          await page.evaluate(() => window.scrollTo(0, 0));
          await page.screenshot({
            path: path.join(output, `${mode}-${width}-${tab}.png`),
            fullPage: true,
            animations: 'disabled',
          });
          report.layouts.push({ mode, width, tab });
        }
        await gotoPunishments();
        await page.evaluate(
          (mode) => window.vm.$store.dispatch('setting/changeTheme', { ...window.vm.$store.state.setting, mode }),
          mode,
        );
        await punishments.getByRole('button', { name: '刷新处罚记录' }).click();
        if (width > 1000)
          await expect(punishments.getByRole('row').filter({ hasText: '203.0.113.50/32' })).toBeVisible();
        else await expect(punishments.locator('.punishment-cards article').filter({ hasText: '203.0.113.50/32' })).toBeVisible();
        if (width === 390) {
          const mobile = punishments.locator('.punishment-cards');
          await expect(mobile.getByRole('button', { name: '释放', exact: true })).toBeVisible();
          await mobile.getByRole('button', { name: '释放', exact: true }).click();
          await expect(page.locator('.el-message-box')).toContainText('共享记录');
          await confirm('取消');
        }
        await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 10000 });
        await page.evaluate(() => window.scrollTo(0, 0));
        await page.screenshot({
          path: path.join(output, `${mode}-${width}-当前处罚.png`),
          fullPage: true,
          animations: 'disabled',
        });
        report.layouts.push({ mode, width, tab: '当前处罚' });
      }
    }
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await check('configuration detour returns to the same service and tab after save', async () => {
    await goto(5000, 'metrics');
    await page.getByRole('button', { name: '配置服务', exact: true }).click();
    const editor = page.getByTestId('port-editor');
    await expect(editor).toBeVisible();
    const name = editor.getByRole('textbox', { name: '服务名称', exact: true });
    await name.fill('家庭相册（已核对）');
    await editor.getByRole('button', { name: '保存端口配置', exact: true }).click();
    await expect(editor).toBeHidden();
    await page.getByRole('link', { name: '返回运行诊断', exact: true }).click();
    await expect(page.getByRole('heading', { name: '家庭相册（已核对）', exact: true })).toBeVisible();
    await expect(page.getByRole('tab', { name: '运行指标', exact: true })).toHaveAttribute('aria-selected', 'true');
    await page.getByRole('button', { name: '配置服务', exact: true }).click();
    await expect(page.getByTestId('port-editor').getByRole('textbox', { name: '服务名称', exact: true })).toHaveValue(
      '家庭相册（已核对）',
    );
    await page
      .getByTestId('port-editor')
      .getByRole('textbox', { name: '服务名称', exact: true })
      .fill('尚未保存的修改');
    await page.evaluate(() => {
      void window.vm.$router.push('/waf-pipeline/ports/5000/diagnostics?tab=metrics');
    });
    await expect(page.locator('.el-message-box')).toContainText('尚未保存');
    await page.locator('.el-message-box__btns button').first().click();
    await expect(page.getByTestId('port-editor').getByRole('textbox', { name: '服务名称', exact: true })).toHaveValue(
      '尚未保存的修改',
    );
    await page.getByRole('dialog').locator('.el-dialog__headerbtn').click();
    await expect(page.locator('.el-message-box')).toContainText('尚未保存');
    await page.locator('.el-message-box__btns button').last().click();
    await page.getByRole('link', { name: '返回运行诊断', exact: true }).click();
  });
  await check('legacy diagnostic links and service-scoped access records remain reachable', async () => {
    await page.goto(origin + '/#/waf-pipeline/ports?inspect=5000');
    await expect(page).toHaveURL(/ports\/5000\/diagnostics/);
    await page.getByRole('button', { name: '查看访问记录', exact: true }).click();
    await expect(page).toHaveURL(/host_code=pipeline-5000/);
  });
  await check('first-load failure, missing service, empty records and invalid port stay distinct', async () => {
    failure = 'logs';
    await goto();
    await expect(diagnostics.getByText('事件读取失败，请重试。', { exact: true })).toBeVisible();
    failure = '';
    await diagnostics.getByRole('button', { name: '重试读取事件', exact: true }).click();
    await expect(diagnostics.locator('.events-table .el-table__row')).toHaveCount(3);
    failure = 'stats';
    await goto(5000, 'metrics');
    await expect(diagnostics.getByText('暂无可用指标，请重试读取。')).toBeVisible();
    await expect(diagnostics.locator('.diagnostic-totals')).toContainText('未提供');
    failure = '';
    await diagnostics.getByRole('button', { name: '重试读取指标', exact: true }).click();
    failure = 'overview';
    await goto();
    await expect(diagnostics).toContainText('没有访问此服务的权限');
    await expect(diagnostics.getByTestId('service-punishments')).toHaveCount(0);
    failure = '';
    await diagnostics.getByRole('button', { name: '重试读取服务', exact: true }).click();
    await expect(diagnostics.getByRole('heading', { name: '家庭相册（已核对）', exact: true })).toBeVisible();
    missing = true;
    await goto();
    await expect(diagnostics.getByRole('heading', { name: '未找到此服务' })).toBeVisible();
    missing = false;
    await goto(8080);
    await expect(diagnostics.getByText('最近记录中没有请求或状态变化。')).toBeVisible();
    await expect(diagnostics.getByText('未启用', { exact: true })).toBeVisible();
    await goto(70000);
    await expect(diagnostics).toContainText('服务端口无效');
  });
  await check('late responses cannot mix data between services', async () => {
    delayedPort = 5000;
    await goto(5000);
    await page.evaluate(() => window.vm.$router.push('/waf-pipeline/ports/8080/diagnostics'));
    await expect(diagnostics.getByRole('heading', { name: '个人博客', exact: true })).toBeVisible();
    await page.waitForTimeout(1100);
    await expect(diagnostics.getByText('最近记录中没有请求或状态变化。')).toBeVisible();
    await expect(diagnostics.locator('.events-table .el-table__row')).toHaveCount(0);
    delayedPort = 0;
  });
  expect(report.errors).toEqual([]);
} catch (error) {
  report.failure = error.stack;
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  process.exitCode = 1;
} finally {
  fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(report, null, 2));
  console.log(
    JSON.stringify(
      { checks: report.checks, errors: report.errors, layouts: report.layouts.length, failure: report.failure },
      null,
      2,
    ),
  );
  await browser.close();
}
