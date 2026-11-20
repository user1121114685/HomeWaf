// Real Vue controls, isolated encrypted API responses; never touches live data.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { fixtureSession, handleTransportHandshake, routeSession } from './transport-fixture.mjs';
const root = fileURLToPath(new URL('../..', import.meta.url));
const origin = process.env.LOG_TEST_ORIGIN || 'http://127.0.0.1:3001';
const output = path.join(root, 'artifacts', 'visit-log-review');
fs.mkdirSync(output, { recursive: true });
const report = { checks: [], errors: [], layouts: [] };
let preference = null,
  failNotes = false,
  noImportantItems = false,
  failPreferenceSave = false,
  failNextList = false,
  failConfig = false;
const queries = [],
  configWrites = [];
const settings = {
  record_log_type: 'all',
  record_max_req_body_length: '4096',
  record_max_res_body_length: '4096',
  record_resp: '0',
  record_all_src_byte_info: '0',
  delete_history_log_day: '180',
  log_db_size: '100000',
  db_file_size: '100',
  log_persist_enable: '1',
  batch_insert: '100',
  ip_tag_db: '0',
  access_log_mode: 'db',
  access_log_retention_days: '30',
  exclude_ip_log: '',
};
const services = [
  {
    value: 'pipeline-15000',
    label: '群晖 · :15000',
    nickname: '群晖',
    source: 'pipeline',
    port: 15000,
    global_host: 0,
  },
  { value: 'global', label: '全局网站', global_host: 1, source: 'host' },
];
const rows = [
  {
    req_uuid: 'normal-request',
    action: '放行',
    rule: '',
    log_only_mode: 0,
    status_code: 200,
    url: '/photos',
    user_agent: 'Safari',
  },
  {
    req_uuid: 'blocked-request',
    action: '阻止',
    rule: 'ip_block',
    log_only_mode: 0,
    status_code: 403,
    url: '/login',
    user_agent: 'Firefox',
  },
  {
    req_uuid: 'observed-request',
    action: '放行',
    rule: 'ssrf',
    log_only_mode: 1,
    status_code: 200,
    url: '/api/entry.cgi',
    user_agent: 'Firefox',
  },
  {
    req_uuid: 'failed-request',
    action: '放行',
    rule: '',
    log_only_mode: 0,
    status_code: 502,
    url: '/albums',
    user_agent: 'Safari',
  },
].map((row, i) => ({
  ...row,
  host_code: 'pipeline-15000',
  host: 'http://nas.example.test',
  src_ip: '192.168.1.' + (20 + i),
  create_time: '2026-11-14 19:22:0' + i,
  time_spent: 20 + i,
  guest_identification: '正常访客',
  country: 'Reserved',
  risk_level: row.rule ? 3 : 0,
}));
const abnormal = (row) => row.action !== '放行' || (row.rule && row.log_only_mode !== 1) || row.status_code >= 400;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-log-review');
  localStorage.setItem('lang', 'zh_CN');
});
let liveSocket;
await context.routeWebSocket('**/api/**', (ws) => { liveSocket = ws; });
await context.route('**/api/**', async (route) => {
  if (await handleTransportHandshake(route)) return;
  const request = route.request(),
    url = new URL(request.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: routeSession(route).encrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, data: routeSession(route).encrypt('{}'), msg } });
  if (endpoint === 'sysinfo/checkversion')
    return fail('Isolated fixture');
  let input = {};
  if (request.postData()) {
    input = JSON.parse(routeSession(route).decrypt(request.postData()));
  }
  if (endpoint === 'waflog/services') return ok(services);
  if (endpoint === 'upgradenotice/summary') return failNotes ? fail('模拟重要待办读取失败') : noImportantItems ? ok({ high_pending_count: 0, popup_items: null }) : ok({
    high_pending_count: 1,
    popup_items: [{ notice_id: 'fresh_manage_trusted_proxies', title: '请勿将 HomeWaf 管理端口暴露在公网', page: '/sys/VpConfig?section=proxy' }],
  });
  if (endpoint === 'uipreference/get') return ok(preference ? { id: 'pref', pref_json: preference } : {});
  if (endpoint === 'uipreference/save') {
    if (failPreferenceSave) return fail('模拟列配置保存失败');
    preference = input.pref_json;
    return ok({});
  }
  if (endpoint === 'systemconfig/getdetailByItem') {
    const item = url.searchParams.get('item');
    return ok({ id: item, item, value: settings[item] || '', category: 'logs', type: 'text', title: item });
  }
  if (endpoint === 'systemconfig/edit') {
    configWrites.push(input);
    if (failConfig) return fail('模拟设置保存失败');
    settings[input.item] = input.value;
    return ok({});
  }
  if (endpoint === 'waflog/attack/allsharedb') return ok([{ file_name: 'local_log.db', is_current: true, cnt: 4 }]);
  if (endpoint === 'waflog/attack/list') {
    queries.push(input);
    if (failNextList) {
      failNextList = false;
      return fail('模拟日志读取失败');
    }
    let list = rows.filter(
      (row) =>
        (!input.host_code || row.host_code === input.host_code) &&
        (!input.request_outcome || (input.request_outcome === 'blocked' ? row.action === '阻止' : !!abnormal(row) === (input.request_outcome === 'abnormal'))) &&
        (!input.req_uuid || row.req_uuid === input.req_uuid) &&
        (!input.src_ip || row.src_ip === input.src_ip) &&
        (!input.url_keyword || row.url.includes(input.url_keyword)),
    );
    if (input.filter_by === 'user_agent') list = list.filter((row) => row.user_agent.includes(input.filter_value));
    if (noImportantItems && input.request_outcome === 'blocked') list = [];
    if (input.url_keyword === '/slow') {
      await new Promise((resolve) => setTimeout(resolve, 700));
      list = [rows[0]];
    }
    return ok({ list, total: list.length, shards: [], took_ms: 3 });
  }
  if (endpoint === 'ai/label/by_uuids') return ok({});
  if (endpoint === 'waflog/exports/capabilities')
    return ok({ allowed: true, enabled: true, formats: ['jsonl', 'csv'] });
  if (endpoint === 'waflog/exports') return ok({ list: [], total: 0 });
  if (endpoint === 'waflog/attack/alliptag') return ok([{ value: '', label: '全部规则' }]);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('console', (message) => {
  if (message.type() === 'error') report.errors.push(message.text().slice(0, 500));
});
page.on('response', (response) => {
  if (response.status() >= 400 && !response.url().includes('/api/'))
    report.errors.push(response.status() + ' ' + response.url());
});
page.on('pageerror', (error) => report.errors.push(error.message));
const visit = () => page.getByTestId('visit-log-page');
const tableRows = () => visit().locator('.el-table__body-wrapper tbody tr');
const headers = () => visit().locator('.el-table__header-wrapper th .cell').allTextContents();
const openColumns = () => page.locator('.page-heading-actions').getByRole('button', { name: '显示列', exact: true }).click();
const dialog = page.getByRole('dialog', { name: '显示列与顺序' });
const order = () => dialog.locator('.column-name').allTextContents();
const saveColumns = () => dialog.getByRole('button', { name: '保存', exact: true }).click();
const query = () => visit().getByRole('button', { name: '查询', exact: true }).click();
const showAll = async () => {
  await visit().getByRole('combobox', { name: '访问情况', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: '全部访问', exact: true }).click();
  await query();
};
const check = (name) => report.checks.push(name);
try {
  await page.goto(origin + '/?appearance=light#/waf/wafvisitlog');
  await expect(tableRows()).toHaveCount(2);
  expect(queries.at(-1).request_outcome).toBe('abnormal');
  await expect(visit().locator('.el-form-item').filter({ has: page.getByRole('combobox', { name: '访问情况' }) })).toContainText('异常');
  await showAll();
  await expect(tableRows()).toHaveCount(4);
  expect((await headers()).map((x) => x.trim())).toEqual([
    '访问情况',
    '时间',
    '服务',
    '来源 IP',
    '访问路径',
    '响应码',
    '命中规则',
    '操作',
  ]);
  await expect(tableRows().nth(1).getByText('ip_block', { exact: true })).toBeVisible();
  await expect(tableRows().first().getByRole('cell').nth(6)).toHaveText('—');
  await expect(tableRows().nth(0)).toContainText('正常');
  await expect(tableRows().nth(2)).toContainText('仅观察');
  await expect(tableRows().nth(2)).toContainText('已放行，保留检测记录');
  await expect(tableRows().nth(3)).toContainText('响应失败');
  check('recommended columns and normal/blocked/observe/HTTP-failure states');
  await visit().getByRole('combobox', { name: '服务', exact: true }).click();
  await page.getByRole('option', { name: '群晖 · :15000', exact: true }).click();
  await visit().getByRole('combobox', { name: '访问情况', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: '异常', exact: true }).click();
  await query();
  await expect(tableRows()).toHaveCount(2);
  expect(queries.at(-1).host_code).toBe('pipeline-15000');
  expect(queries.at(-1).request_outcome).toBe('abnormal');
  await visit().getByRole('button', { name: '重置', exact: true }).click();
  await expect(tableRows()).toHaveCount(2);
  expect(queries.at(-1).host_code).toBe('');
  expect(queries.at(-1).request_outcome).toBe('abnormal');
  check('service identity, server-side outcome filters, complete reset');
  await openColumns();
  await expect(dialog.getByRole('checkbox', { name: '访客身份', exact: true })).toHaveCount(0);
  await expect(dialog.getByRole('checkbox', { name: '操作', exact: true })).toBeDisabled();
  await dialog.locator('.column-choices').getByText('响应码', { exact: true }).click();
  await expect(dialog.getByRole('checkbox', { name: '响应码', exact: true })).not.toBeChecked();
  const sortRows = dialog.getByRole('listitem');
  await sortRows.filter({ has: page.getByText('命中规则', { exact: true }) }).locator('[draggable]').dragTo(sortRows.first(), {
    targetPosition: { x: 48, y: 4 },
  });
  await expect.poll(async () => (await order())[0]).toBe('命中规则');
  const position = dialog.getByRole('combobox', { name: '访问路径的位置', exact: true });
  await position.press('End');
  await expect(position).toBeFocused();
  await expect.poll(async () => (await order()).at(-1)).toBe('访问路径');
  await saveColumns();
  await expect.poll(async () => (await headers())[0].trim()).toBe('命中规则');
  expect((await headers()).at(-1).trim()).toBe('访问路径');
  expect((await headers()).some((x) => x.includes('响应码'))).toBe(false);
  await expect.poll(() => preference && JSON.parse(preference).columns).toEqual(['rule', 'outcome', 'create_time', 'service', 'src_ip', 'op', 'url']);
  await openColumns();
  await dialog.getByRole('button', { name: '恢复推荐列' }).click();
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  expect((await headers())[0].trim()).toBe('命中规则');
  await page.reload();
  await expect(tableRows()).toHaveCount(2);
  await expect.poll(async () => (await headers())[0].trim()).toBe('命中规则');
  expect((await headers()).some((x) => x.includes('响应码'))).toBe(false);
  check('drag across multiple rows, jump by keyboard, cancel, save and reload');
  preference = JSON.stringify({ v: 3, columns: ['url', 'guest_identification', 'header', 'op'] });
  await page.reload();
  await expect.poll(async () => (await headers()).map(x => x.trim())).toEqual(['访问路径', '命中规则', '操作']);
  await openColumns();
  await expect(dialog.getByRole('checkbox', { name: '请求头', exact: true })).toHaveCount(0);
  const rulePosition = dialog.getByRole('combobox', { name: '命中规则的位置', exact: true });
  await rulePosition.selectOption('0');
  failPreferenceSave = true;
  await saveColumns();
  await expect(page.getByText('列配置已保存到本地，同步服务端失败', { exact: true })).toBeVisible();
  expect(JSON.parse(preference).columns[1]).toBe('guest_identification');
  const cached = await page.evaluate(() => JSON.parse(localStorage.getItem('attack_table_display_columns')).columns);
  expect(cached).toEqual(['rule', 'url', 'header', 'op']);
  failPreferenceSave = false;
  await openColumns();
  await saveColumns();
  await expect.poll(() => JSON.parse(preference).columns).toEqual(cached);
  check('old account preferences migrate in place, inactive-view fields survive edits, failed sync can be retried');
  await openColumns();
  await dialog.getByRole('button', { name: '恢复推荐列' }).click();
  await dialog.locator('.column-choices').getByText('User-Agent', { exact: true }).click();
  await expect(dialog.getByRole('checkbox', { name: 'User-Agent', exact: true })).toBeChecked();
  await saveColumns();
  await showAll();
  await visit().getByRole('button', { name: '筛选User-Agent', exact: true }).click();
  const filterPanel = page.locator('.hw-column-filter-panel:visible');
  await filterPanel.getByRole('textbox').fill('Firefox');
  await filterPanel.getByRole('button', { name: '应用', exact: true }).click();
  await expect(tableRows()).toHaveCount(2);
  expect(queries.at(-1).filter_by).toBe('user_agent');
  await visit().getByRole('button', { name: '重置', exact: true }).click();
  await expect(tableRows()).toHaveCount(2);
  expect(queries.at(-1).filter_by).toBe('');
  check('text column filters apply and reset through the shared table');
  const pathInput = visit().getByRole('textbox', { name: '访问路径', exact: true });
  await pathInput.fill('/slow');
  await pathInput.press('Enter');
  await expect.poll(() => queries.at(-1).url_keyword).toBe('/slow');
  await visit().getByRole('button', { name: '重置', exact: true }).click();
  await page.waitForTimeout(850);
  await expect(tableRows()).toHaveCount(2);
  check('older requests cannot overwrite a newer reset result');
  failNextList = true;
  await query();
  await expect(visit()).toContainText('模拟日志读取失败');
  await expect(tableRows()).toHaveCount(0);
  await visit().getByRole('button', { name: '重试', exact: true }).click();
  await expect(tableRows()).toHaveCount(2);
  check('request failure clears stale data and offers retry');
  await visit().getByRole('combobox', { name: '时间范围', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: '自定义时间', exact: true }).click();
  await expect(visit().locator('.el-date-editor--datetimerange')).toBeVisible();
  await visit().locator('.el-date-editor--datetimerange input').first().click();
  await expect(page.locator('.hw-date-range-popup:visible .el-picker-panel__shortcut')).toHaveCount(8);
  await page.keyboard.press('Escape');
  check('time picker includes time-of-day and working date shortcuts');
  await visit().getByRole('button', { name: '重置', exact: true }).click();
  await page.locator('.page-heading-actions').getByRole('button', { name: '日志设置', exact: true }).click();
  const drawer = page.getByRole('dialog', { name: '日志设置', exact: true });
  await expect(drawer.getByRole('button', { name: '保存设置', exact: true })).toBeDisabled();
  const retention = drawer.getByRole('spinbutton', { name: '访问日志保留天数', exact: true });
  await expect(retention).toHaveValue('30');
  await retention.fill('31');
  await retention.press('Tab');
  failConfig = true;
  await drawer.getByRole('button', { name: '保存设置', exact: true }).click();
  await expect(page.getByText('部分设置未保存，请重试。', { exact: true })).toBeVisible();
  expect(settings.access_log_retention_days).toBe('30');
  failConfig = false;
  await drawer.getByRole('button', { name: '保存设置', exact: true }).click();
  await expect.poll(() => settings.access_log_retention_days).toBe('31');
  expect(configWrites.every((write) => write.item === 'access_log_retention_days')).toBe(true);
  await drawer.getByRole('button', { name: '关闭', exact: true }).click();
  check('configuration numbers load, only changed values save, business failures never report success');
  await openColumns();
  await dialog.getByRole('button', { name: '恢复推荐列' }).click();
  await saveColumns();
  for (const theme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: theme });
    await page.goto(origin + '/#/waf/wafvisitlog');
    await expect(page.locator('html')).toHaveAttribute('theme-mode', theme);
    await expect(tableRows()).toHaveCount(2);
    await expect(page.locator('.el-dialog:visible')).toHaveCount(0);
    await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 10000 });
    for (const width of [1440, 1024, 768, 390, 320]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      await page.evaluate(() => window.scrollTo(0, 0));
      const layout = await page.evaluate(() => {
        const controls = [
          ...document.querySelectorAll(
            '.visit-filter-grid:first-child > .el-form-item .el-input__wrapper, .visit-filter-grid:first-child > .el-form-item .el-select__wrapper',
          ),
        ].map((el) => {
          const r = el.getBoundingClientRect();
          return { w: r.width, h: r.height };
        });
        return { overflow: document.documentElement.scrollWidth > innerWidth + 2, controls };
      });
      report.layouts.push({ theme, width, ...layout });
      expect(layout.overflow).toBe(false);
      expect(layout.controls.every((control) => Math.abs(control.h - 40) < 1)).toBe(true);
      const sameTrack = width <= 520 ? layout.controls.slice(1) : layout.controls;
      expect(Math.max(...sameTrack.map((x) => x.w)) - Math.min(...sameTrack.map((x) => x.w))).toBeLessThan(
        2,
      );
      if (width <= 520) {
        expect(layout.controls[0].w).toBeGreaterThan(sameTrack[0].w * 1.9);
        expect(sameTrack.every(control => control.w >= 100)).toBe(true);
      }
      if (width <= 1023) {
        await expect(visit().locator('.visit-compact-record').first()).toContainText('ip_block');
        await expect(visit().locator('.visit-compact-record').first()).toContainText('群晖');
        await expect(visit().locator('.visit-compact-record').first()).toContainText('/login');
        await expect(visit().locator('.visit-compact-record').first()).toContainText('192.168.1.21');
        expect(await visit().locator('.el-table__body-wrapper .el-scrollbar__wrap').evaluate(el => el.scrollHeight <= el.clientHeight + 2)).toBe(true);
      }
      await page.screenshot({ path: path.join(output, theme + '-' + width + '.png'), fullPage: true });
      await openColumns();
      const fitsDialog = await dialog.locator('.column-settings').evaluate(el => el.scrollWidth <= el.clientWidth + 1);
      expect(fitsDialog).toBe(true);
      await dialog.screenshot({ path: path.join(output, 'columns-' + theme + '-' + width + '.png') });
      if (width === 390) {
        await rulePosition.selectOption('0');
        await saveColumns();
        await expect(visit().locator('.visit-compact-record dt').first()).toHaveText('命中规则');
        await openColumns();
        await dialog.getByRole('button', { name: '恢复推荐列' }).click();
        await saveColumns();
      } else {
        await dialog.getByRole('button', { name: '取消', exact: true }).click();
      }
    }
  }
  check('aligned filter controls, full-width mobile service scope and no page overflow in both themes');
  await expect(page.locator('.console-header .search-trigger-btn')).toHaveCount(0);
  await page.keyboard.press('Control+k');
  await expect(page.getByRole('dialog', { name: '查找功能' })).toBeVisible();
  await page.keyboard.press('Escape');
  const bell = page.getByRole('button', { name: '通知', exact: true });
  const panel = page.getByRole('region', { name: '通知', exact: true });
  await bell.click();
  const task = panel.getByRole('link', { name: '请勿将 HomeWaf 管理端口暴露在公网' });
  await expect(task).toHaveAttribute('href', /VpConfig\?section=proxy/);
  await expect(panel.getByRole('link', { name: /ip_block/ })).toHaveAttribute('href', /req_uuid=blocked-request/);
  await expect(panel.getByText('ssrf')).toHaveCount(0);
  const bounds = await panel.boundingBox();
  expect(bounds.x).toBeGreaterThanOrEqual(0);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(320);
  const socket = fixtureSession(new URL(liveSocket.url()).searchParams.get('keyid'));
  liveSocket.send(JSON.stringify({ msg_code: '200', msg_cmd_type: 'Info', msg_data: socket.encrypt(JSON.stringify({
    message_id: 'system-check', message_type: '运维', message_data: '测试证书续期提醒', message_unread_status: true,
  })) }));
  await expect(panel.getByText('测试证书续期提醒')).toBeVisible();
  await panel.getByRole('button', { name: '设为已读', exact: true }).click();
  await expect(panel.getByText('测试证书续期提醒')).toHaveCount(0);
  failNotes = true;
  await panel.getByRole('button', { name: '刷新' }).click();
  await expect(panel).toContainText('模拟重要待办读取失败');
  await expect(panel.getByRole('link', { name: /ip_block/ })).toBeVisible();
  failNotes = false;
  await panel.getByRole('button', { name: '重试' }).click();
  await expect(task).toBeVisible();
  for (const [width, theme] of [[390, 'dark'], [768, 'light'], [1440, 'dark']]) {
    await page.setViewportSize({ width, height: 880 });
    await page.emulateMedia({ colorScheme: theme });
    await expect(page.locator('html')).toHaveAttribute('theme-mode', theme);
    await expect(task).toBeVisible();
    await panel.screenshot({ path: path.join(output, 'notifications-' + width + '-' + theme + '.png') });
  }
  noImportantItems = true;
  await panel.getByRole('button', { name: '刷新' }).click();
  await expect(panel.getByText('没有待处理的重要升级事项。')).toBeVisible();
  await expect(panel.getByText('这段时间没有拦截记录。')).toBeVisible();
  await expect(panel.getByRole('link', { name: /管理端口仅供内网访问/ })).toBeVisible();
  noImportantItems = false;
  await panel.getByRole('button', { name: '刷新' }).click();
  await expect(panel.getByRole('link', { name: /ip_block/ })).toBeVisible();
  await panel.getByRole('link', { name: '查看记录' }).click();
  await expect(tableRows()).toHaveCount(1);
  expect(queries.at(-1).request_outcome).toBe('blocked');
  await page.goto(origin + '/#/waf/wafvisitlog?req_uuid=normal-request');
  await expect(tableRows().first()).toContainText('/photos');
  expect(queries.at(-1).request_outcome).toBeUndefined();
  await page.goto(origin + '/#/waf/wafvisitlog');
  await expect(tableRows()).toHaveCount(2);
  check('bell loads pending tasks and actual blocks, handles live messages, retries and drill-downs; plain route restores abnormal');
  expect(report.errors).toEqual([]);
} catch (error) {
  report.page = (await page.locator('body').innerText()).slice(0, 3000);
  report.failure = error.stack;
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  throw error;
} finally {
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
  await browser.close();
}
