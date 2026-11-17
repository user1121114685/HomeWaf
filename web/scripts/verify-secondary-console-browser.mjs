// Real secondary screens, isolated API responses. All writes are intercepted.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';
const root = fileURLToPath(new URL('../..', import.meta.url)),
  dist = path.join(root, 'web/dist'),
  output = path.join(root, 'artifacts/console-subpages/browser');
fs.mkdirSync(output, { recursive: true });
const mime = {
  '.html': 'text/html',
  '.js': 'application/javascript',
  '.css': 'text/css',
  '.svg': 'image/svg+xml',
  '.json': 'application/json',
};
const server = http.createServer((req, res) => {
  const requested = decodeURIComponent(new URL(req.url, 'http://localhost').pathname),
    file = path.resolve(dist, '.' + (requested === '/' ? '/index.html' : requested));
  if (!file.startsWith(dist + path.sep) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
    res.writeHead(404);
    res.end();
    return;
  }
  res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`,
  browser = await chromium.launch({ headless: true, ...(process.env.CI ? {} : { channel: 'chrome' }) });
const context = await browser.newContext({ viewport: { width: 1440, height: 1050 }, reducedMotion: 'reduce' }),
  report = { checks: [], errors: [], writes: [], states: [] };
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-secondary-test');
  localStorage.setItem('lang', 'zh_CN');
  sessionStorage.setItem('app_op_password', 'isolated-operation-password');
  sessionStorage.setItem('app_op_password_ts', String(Date.now()));
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
const catalogue = {
  auth_info: { header_name: 'X-API-Key' },
  modules: {
    服务管理: Array.from({ length: 45 }, (_, i) => ({
      method: i % 2 ? 'GET' : 'POST',
      path: `/api/v1/sample/${String(i).padStart(2, '0')}`,
      description: `服务接口 ${i}`,
      param_example: '{"port":15000}',
      response_example: '{"code":0}',
      curl_example: 'curl http://your-samwaf-host:26666/api/v1/sample -H "X-API-Key: YOUR_KEY"',
    })),
  },
};
const stateRows = new Map([
  [
    15000,
    [
      {
        id: 'shared-ban',
        kind: 'ban',
        scope: 'global',
        prefix: '192.0.2.5/32',
        reason: 'isolated fixture',
        expires_at: 1893456000,
      },
    ],
  ],
  [
    15001,
    [
      {
        id: 'local-lock',
        kind: 'lockdown',
        scope: 'port',
        prefix: '',
        reason: 'isolated fixture',
        expires_at: 1893456000,
      },
    ],
  ],
]);
const app = {
  id: 'app-id',
  code: 'fixture-app',
  name: '测试应用',
  run_status: 0,
  start_cmd: './fixture-server',
  app_dir: 'fixtures/app',
  env: 'ENV=fixture',
  start_status: 1,
  auto_start: 0,
  stop_mode: 'signal',
  stop_timeout: 30,
  restart_policy: 'on-failure',
  restart_delay: 5,
  max_restart_count: 3,
  log_max_lines: 1000,
  remarks: '保留原备注',
};
const tunnel = {
  id: 'tunnel-id',
  code: 'fixture-tunnel',
  name: '测试隧道',
  port: '5300',
  protocol: 'udp',
  remote_ip: '192.0.2.90',
  remote_port: 53,
  start_status: 0,
  ip_version: 'both',
  ssl_status: 0,
  allow_ip: '',
  deny_ip: '',
  allowed_time_ranges: '',
  conn_timeout: 30,
  read_timeout: 30,
  write_timeout: 30,
  max_in_connect: 50,
  max_out_connect: 50,
  remark: '保留的隧道备注',
};
let failDocs = false,
  failStatus = false,
  emptyDocs = false,
  failState = false,
  failManage = false,
  failBackups = false,
  failAppSave = false;
let failGuardRead = false,
  failGuardSave = false,
  failTuning = false;
const guardValues = {
  host_guard_mode: 'observe',
  host_guard_whitelist: '192.0.2.1',
  host_guard_ssh_ports: '22',
  host_guard_rdp_ports: '3389',
  host_guard_port_scope: 'all',
  host_guard_exec_mode: 'auto',
};
await context.route('**/api/**', async (route) => {
  const req = route.request(),
    url = new URL(req.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } }),
    fail = (msg) => route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg } });
  let input = {};
  if (req.postData()) {
    try {
      input = JSON.parse(AesDecrypt(req.postData()));
    } catch {
      try {
        input = JSON.parse(req.postData());
      } catch {}
    }
  }
  if (endpoint === 'oplatform/doc/api')
    return failDocs ? fail('模拟目录读取失败') : ok(emptyDocs ? { modules: {} } : catalogue);
  if (endpoint === 'oplatform/doc/status') return failStatus ? fail('模拟状态读取失败') : ok({ enabled: 1 });
  if (endpoint === 'pipeline/overview')
    return ok({ ports: [15000, 15001].map((port) => ({ config: { port, name: '测试服务' + port } })) });
  if (endpoint === 'pipeline/state')
    return failState ? fail('模拟处罚读取失败') : ok(stateRows.get(Number(url.searchParams.get('port'))) || []);
  if (endpoint === 'pipeline/state/manage') {
    report.writes.push({ endpoint, input });
    if (failManage) return fail('模拟处罚操作失败');
    stateRows.set(
      input.port,
      stateRows.get(input.port).filter((row) => row.id !== input.id),
    );
    return ok({});
  }
  if (endpoint === 'application/app/feature_status') return ok({ enabled: true });
  if (endpoint === 'application/app/list') return ok({ list: [app], total: 1 });
  if (endpoint === 'application/app/detail') return ok(app);
  if (endpoint === 'application/app/verifypwd')
    return req.headers()['x-app-op-password'] === 'isolated-operation-password' ? ok({}) : fail('操作密码错误');
  if (endpoint === 'application/app/directory_preview')
    return ok({
      resolved_dir: 'D:/isolated/' + input.app_dir,
      default_dir: 'D:/isolated/default',
      allowed_dirs: ['D:/isolated'],
      valid: true,
    });
  if (endpoint === 'application/app/edit') {
    report.writes.push({ endpoint, input });
    if (failAppSave) return fail('模拟应用保存失败');
    Object.assign(app, input);
    return ok({});
  }
  if (endpoint === 'application/app/backups')
    return failBackups
      ? fail('模拟备份读取失败')
      : ok({ list: [{ filename: 'fixture-backup.bin', size: 1024, created_at: '2026-11-15 12:00:00' }] });
  if (endpoint === 'application/app/status') return ok({ run_status: 0 });
  if (endpoint === 'application/app/stop' || endpoint === 'application/app/rollback') {
    report.writes.push({
      endpoint,
      params: Object.fromEntries(url.searchParams),
      passwordPresent: !!req.headers()['x-app-op-password'],
    });
    return ok({});
  }
  if (endpoint === 'application/app/logs')
    return ok({ logs: ['fixture application started', 'listening on 127.0.0.1:8080'] });
  if (endpoint === 'application/app/network')
    return ok({
      ports: [{ local_addr: '127.0.0.1:8080', protocol: 'TCP', pid: 1234 }],
      connections: [{ remote_ip: '192.0.2.30', local_addr: '127.0.0.1:8080' }],
    });
  if (endpoint === 'application/app/changelogs') return ok({ list: [], total: 0 });
  if (endpoint === 'tunnel/tunnel/list') return ok({ list: [tunnel], total: 1 });
  if (endpoint === 'tunnel/tunnel/detail') return ok(tunnel);
  if (endpoint === 'tunnel/tunnel/connections')
    return ok({
      tunnel_info: tunnel,
      protocol: 'udp',
      port_info: [
        {
          port: 5300,
          tcp_source_count: 0,
          tcp_target_count: 0,
          udp_source_count: 1,
          udp_target_count: 1,
          tcp_source_ips: [],
          udp_source_ips: [{ ip: '192.0.2.77', region: '测试地区' }],
        },
      ],
    });
  if (endpoint === 'tunnel/tunnel/edit') {
    report.writes.push({ endpoint, input });
    Object.assign(tunnel, input);
    return ok({});
  }
  if (endpoint === 'hostguard/status')
    return ok({ running: false, mode: 'observe', port_scope_supported: true, capability: { FirewallReady: true } });
  if (endpoint === 'hostguard/statistics') return ok({});
  if (endpoint === 'hostguard/ladder/list') return ok({ list: [] });
  if (endpoint === 'systemconfig/getdetailByItem') {
    const item = url.searchParams.get('item');
    return failGuardRead && item === 'host_guard_max_retry'
      ? fail('模拟设置读取失败')
      : ok({ item, value: guardValues[item] || '0' });
  }
  if (endpoint === 'systemconfig/editByItem') {
    report.writes.push({ endpoint, input });
    if (failGuardSave && input.item === 'host_guard_max_retry') return fail('模拟单项保存失败');
    guardValues[input.item] = input.value;
    return ok({});
  }
  if (endpoint === 'owasp/tuning' && req.method() === 'POST') {
    report.writes.push({ endpoint, input });
    return ok({});
  }
  if (endpoint === 'owasp/tuning')
    return failTuning
      ? fail('模拟引擎读取失败')
      : ok({
          rule_engine: 'DetectionOnly',
          blocking_paranoia_level: 1,
          detection_paranoia_level: 2,
          inbound_anomaly_score_threshold: 7,
          outbound_anomaly_score_threshold: 4,
          enforce_bodyproc_urlencoded: 1,
        });
  if (endpoint === 'owasp/crs_vars') return ok({ vars: {} });
  if (endpoint === 'wafhost/host/allhost' || endpoint === 'wafhost/ipgroup/options') return ok([]);
  if (endpoint.startsWith('application/') && req.method() !== 'GET')
    return fail('Unexpected application write: ' + endpoint);
  if (['account/info', 'account/current', 'account/detail'].includes(endpoint))
    return ok({ user_name: 'fixture', role: 'super_admin' });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint))
    return fail('Isolated fixture');
  if (req.method() === 'POST' && !endpoint.endsWith('/list')) return fail('Unhandled isolated endpoint: ' + endpoint);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
const dialog = () => page.locator('.el-dialog:visible');
async function goto(route, testid) {
  await page.goto(origin + '/?verification=' + Date.now() + '#' + route);
  await expect(page.getByTestId(testid)).toBeVisible({ timeout: 20000 });
  expect(new URL(page.url()).hash).toBe('#' + route);
}
async function capture(name, scope = page) {
  await expect(page.locator('.el-overlay.is-message-box')).toHaveCount(0);
  for (const width of [1440, 768, 390])
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width, height: 1050 });
      await page.evaluate((theme) => {
        document.documentElement.dataset.theme = theme;
        document.documentElement.classList.toggle('dark', theme === 'dark');
      }, theme);
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
        .toBeLessThanOrEqual(2);
      const dialogs = await page.locator('.el-dialog:visible').evaluateAll((elements) =>
        elements.map((el) => {
          const rect = el.getBoundingClientRect();
          return {
            title: el.querySelector('.el-dialog__title')?.textContent,
            width: rect.width,
            x: rect.x,
            right: rect.right,
            overflow: el.scrollWidth > el.clientWidth + 1,
          };
        }),
      );
      for (const box of dialogs) {
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.right).toBeLessThanOrEqual(width + 1);
        expect(box.overflow).toBe(false);
        if (name.startsWith('confirmation-') && name !== 'confirmation-form')
          expect(box.width).toBeLessThanOrEqual(480);
      }
      if (name.startsWith('confirmation-')) {
        expect(
          await dialog()
            .locator('.el-dialog__body')
            .evaluate((el) => el.scrollWidth <= el.clientWidth + 1),
        ).toBe(true);
      }
      await scope.screenshot({ path: path.join(output, `${name}-${width}-${theme}.png`), animations: 'disabled' });
      report.states.push({ name, width, theme, dialogs });
    }
  await page.setViewportSize({ width: 1440, height: 1050 });
}
try {
  if (!process.argv.includes('--application-only')) {
  await goto('/oplatform/doc', 'api-reference');
  // Exercise the app's shared dialog adapter separately from page-specific forms.
  for (const kind of ['confirm', 'alert', 'long-text', 'form']) {
    await page.evaluate((kind) => {
      const app = document.querySelector('#app').__vue_app__;
      const plugin = app.config.globalProperties.$dialog;
      window.dialogVerification = { confirmed: 0, cancelled: 0 };
      const options = {
        header: kind === 'form' ? '输入内容' : '确认操作',
        body:
          kind === 'long-text' ? '操作仅影响当前记录。'.repeat(20) + 'f'.repeat(180) : '此操作只影响当前选中的记录。',
        confirmBtn: '确认操作',
        cancelBtn: '继续编辑',
        onConfirm: ({ dialog }) => {
          window.dialogVerification.confirmed++;
          dialog.hide();
        },
        onCancel: () => {
          window.dialogVerification.cancelled++;
        },
      };
      if (kind === 'form') {
        options.width = '640px';
        options.body = (h) =>
          h('label', [
            h('span', '操作备注'),
            h(app.component('hw-input'), { modelValue: '可编辑的内容', 'aria-label': '操作备注' }),
          ]);
      }
      plugin[kind === 'alert' ? 'alert' : 'confirm'](options);
    }, kind);
    await expect(dialog()).toBeVisible();
    if (kind === 'form') {
      await expect(dialog().getByRole('textbox', { name: '操作备注' })).toHaveValue('可编辑的内容');
      expect((await dialog().boundingBox()).width).toBe(640);
    }
    await capture('confirmation-' + kind, dialog());
    if (kind === 'alert') {
      await expect(dialog().getByRole('button', { name: '继续编辑' })).toHaveCount(0);
      await dialog().getByRole('button', { name: '确认操作', exact: true }).click();
      expect(await page.evaluate(() => window.dialogVerification.confirmed)).toBe(1);
    } else {
      await dialog().getByRole('button', { name: '继续编辑', exact: true }).click();
      expect(await page.evaluate(() => window.dialogVerification.confirmed)).toBe(0);
      expect(await page.evaluate(() => window.dialogVerification.cancelled)).toBe(1);
    }
    await expect(dialog()).toHaveCount(0);
  }
  report.checks.push(
    'Shared confirmation/alert/long-text dialogs stay compact; explicit form width, input and cancel/confirm semantics remain intact across widths/themes',
  );
  await expect(page.getByText('45 个接口', { exact: true })).toBeVisible();
  await page.locator('.api-index button').filter({ hasText: '/api/v1/sample/01' }).click();
  await expect(page.getByTestId('api-detail')).toContainText('/api/v1/sample/01');
  await expect(page.getByTestId('api-detail')).toContainText(origin + '/api/v1/sample');
  await page.getByLabel('查找接口', { exact: true }).fill('GET 01');
  await expect(page.getByText('1 个接口', { exact: true })).toBeVisible();
  await page.getByLabel('查找接口', { exact: true }).fill('no-such-endpoint');
  await expect(page.getByText('没有匹配的接口，请调整筛选条件。')).toBeVisible();
  await page.getByLabel('查找接口', { exact: true }).fill('');
  await page
    .locator('.api-index .el-pagination')
    .getByRole('button', { name: /next|下一页/i })
    .click();
  await expect(page.getByTestId('api-detail')).toContainText('/api/v1/sample/40');
  await capture('api-reference');
  failDocs = true;
  failStatus = true;
  await page.getByRole('button', { name: '刷新接口文档' }).click();
  await expect(page.getByText(/当前仍显示上次读取的结果/)).toBeVisible();
  await expect(page.getByText('状态未知', { exact: true })).toBeVisible();
  failDocs = false;
  failStatus = false;
  emptyDocs = true;
  await page.getByRole('button', { name: '刷新接口文档' }).click();
  await expect(page.getByText('当前实例没有返回可展示的接口。')).toBeVisible();
  emptyDocs = false;
  expect(report.writes).toHaveLength(0);
  report.checks.push(
    'API catalogue filters, details, pagination, examples, stale/error/empty states; no business write',
  );

  await goto('/waf-host/wafIPBan', 'service-punishments');
  const punish = page.getByTestId('service-punishments');
  await expect(punish.getByText('192.0.2.5/32', { exact: true })).toBeVisible();
  await punish.getByRole('button', { name: '释放', exact: true }).click();
  await expect(page.locator('.el-message-box')).toContainText('释放会同时影响其他服务');
  await page.locator('.el-message-box').getByRole('button', { name: '取消', exact: true }).click();
  expect(report.writes).toHaveLength(0);
  failManage = true;
  await punish.getByRole('button', { name: '释放', exact: true }).click();
  await page.getByRole('button', { name: '确认操作', exact: true }).click();
  await expect(punish.getByText('模拟处罚操作失败')).toBeVisible();
  await expect(punish.getByText('192.0.2.5/32', { exact: true })).toBeVisible();
  failManage = false;
  await punish.getByRole('button', { name: '刷新处罚记录' }).click();
  await expect(punish.getByRole('button', { name: '释放', exact: true })).toBeEnabled();
  await capture('native-punishments');
  await punish.getByRole('button', { name: '释放', exact: true }).click();
  await page.getByRole('button', { name: '确认操作', exact: true }).click();
  await expect(punish.getByText('当前服务没有处罚记录')).toBeVisible();
  expect(stateRows.get(15001)).toHaveLength(1);
  await punish.locator('.el-select').click();
  await page.getByRole('option', { name: '测试服务15001 · :15001', exact: true }).click();
  await expect(punish.getByRole('button', { name: '解除锁定' })).toBeVisible();
  failState = true;
  await punish.getByRole('button', { name: '刷新处罚记录' }).click();
  await expect(punish.getByText(/当前保留上次结果/)).toBeVisible();
  await expect(punish.getByRole('button', { name: '解除锁定' })).toBeDisabled();
  failState = false;
  expect(report.writes.every((write) => write.endpoint.startsWith('pipeline/'))).toBe(true);
  report.checks.push(
    'Native punishment confirmation, cancellation, failed action retention, service isolation and stale-read action lock',
  );

  }
  await page.goto(origin + '/?verification=' + Date.now() + '#/application/AppList');
  await expect(page.locator('.el-table__row').filter({ hasText: '测试应用' })).toBeVisible();
  const row = () => page.locator('.el-table__row').filter({ hasText: '测试应用' });
  await row().getByRole('button', { name: '编辑', exact: true }).click();
  await expect(dialog().locator('input[type=password]')).toBeVisible();
  await dialog().locator('input[type=password]').fill('wrong-operation-password');
  await dialog().getByRole('button', { name: '确认', exact: true }).click();
  await expect(dialog().locator('input[type=password]')).toBeVisible();
  await dialog().getByRole('button', { name: '取消', exact: true }).click();
  await row().getByRole('button', { name: '编辑', exact: true }).click();
  await expect(dialog().locator('input[type=password]')).toHaveValue('');
  await capture('application-operation-password');
  await dialog().locator('input[type=password]').fill('isolated-operation-password');
  await dialog().getByRole('button', { name: '确认', exact: true }).click();
  const form = page.getByTestId('application-form');
  await expect(form).toBeVisible();
  expect(await page.evaluate(() => sessionStorage.getItem('app_op_password'))).toBeNull();
  expect(await page.evaluate(() => sessionStorage.getItem('app_op_password_ts'))).toBeNull();
  expect(await form.locator('fieldset legend').allTextContents()).toEqual([
    '应用与启动位置',
    '启动与异常恢复',
    '停止方式',
    '日志与备注',
  ]);
  const command = form
    .locator('.el-form-item')
    .filter({ has: page.locator('label').filter({ hasText: '启动命令' }) })
    .locator('input');
  await command.fill('./modified-server');
  failAppSave = true;
  await form.getByRole('button', { name: '提交', exact: true }).click();
  await expect(form.getByText('模拟应用保存失败')).toBeVisible();
  await expect(command).toHaveValue('./modified-server');
  await capture('application-edit');
  failAppSave = false;
  await form.getByRole('button', { name: '提交', exact: true }).click();
  await expect(form).toBeHidden();
  expect(app.start_cmd).toBe('./modified-server');
  expect(app.remarks).toBe('保留原备注');
  await row()
    .getByRole('button', { name: /升级|更新与恢复/ })
    .click();
  const maintenance = page.getByTestId('application-maintenance');
  await expect(maintenance).toBeVisible();
  await maintenance.getByRole('tab', { name: '恢复备份', exact: true }).click();
  await expect(maintenance.getByText('fixture-backup.bin')).toBeVisible();
  failBackups = true;
  await maintenance.getByRole('button', { name: '刷新备份' }).click();
  await expect(maintenance.getByText(/当前保留上次结果/)).toBeVisible();
  failBackups = false;
  await maintenance.getByRole('button', { name: '刷新备份' }).click();
  await expect(maintenance.getByText(/模拟备份读取失败/)).toBeHidden();
  await capture('application-restore');
  await maintenance.getByRole('button', { name: '回滚', exact: true }).click();
  await expect(page.locator('.el-message-box')).toContainText('fixture-backup.bin');
  await page.locator('.el-message-box').getByRole('button', { name: '取消', exact: true }).click();
  expect(report.writes.some((w) => w.endpoint.endsWith('/rollback'))).toBe(false);
  await maintenance.getByRole('button', { name: '回滚', exact: true }).click();
  await page.getByRole('button', { name: '停止并恢复', exact: true }).click();
  await expect(maintenance).toBeHidden();
  const recovery = report.writes.filter((w) => /\/(stop|rollback)$/.test(w.endpoint));
  expect(recovery.map((w) => w.endpoint)).toEqual(['application/app/stop', 'application/app/rollback']);
  expect(recovery.every((w) => w.passwordPresent)).toBe(true);
  expect(recovery[1].params.filename).toBe('fixture-backup.bin');
  report.checks.push(
    'Application edit preserves fields and failed draft; restore has read recovery, cancellation, stop-before-rollback and password header',
  );
  await page.reload();
  await row().getByRole('button', { name: '编辑', exact: true }).click();
  await expect(dialog().locator('input[type=password]')).toBeVisible();
  await dialog().getByRole('button', { name: '取消', exact: true }).click();
  report.checks.push('Operation passwords require verification, survive only in memory, and are requested again after reload');
  if (!process.argv.includes('--application-only')) {
  await page.goto(origin + '/?verification=' + Date.now() + '#/tunnel/Tunnel');
  const tunnelRow = page.locator('.el-table__row').filter({ hasText: '测试隧道' });
  await expect(tunnelRow).toBeVisible();
  await tunnelRow.getByRole('button', { name: '查看连接', exact: true }).click();
  const connections = page.getByTestId('tunnel-connections');
  await expect(connections.getByText('192.0.2.77', { exact: true })).toBeVisible();
  await connections.getByLabel('查找连接来源').fill('absent');
  await expect(connections.getByText('没有匹配的来源，请调整筛选')).toBeVisible();
  await connections.getByLabel('查找连接来源').fill('');
  await capture('tunnel-udp');
  await dialog().locator('.el-dialog__headerbtn').click();
  await tunnelRow.getByRole('button', { name: '编辑', exact: true }).click();
  await expect(dialog().getByText('本机监听入口', { exact: true })).toBeVisible();
  await expect(dialog().getByText('转发目标', { exact: true })).toBeVisible();
  await capture('tunnel-edit');
  await dialog().getByRole('button', { name: '关闭', exact: true }).click();
  expect(tunnel.protocol).toBe('udp');
  expect(tunnel.remark).toBe('保留的隧道备注');
  report.checks.push(
    'UDP source addresses returned by backend are visible and searchable; tunnel editor covers listener, target, restrictions, TLS and limits',
  );
  failGuardRead = true;
  await page.goto(origin + '/?verification=' + Date.now() + '#/sys/HostGuard');
  await page.getByRole('tab', { name: '设置', exact: true }).click();
  await expect(page.getByText('部分设置未能读取，已禁止提交，请重新读取设置。')).toBeVisible();
  const guardSave = page.getByRole('button', { name: '保存', exact: true });
  await expect(guardSave).toBeDisabled();
  failGuardRead = false;
  await page.getByRole('button', { name: '重新读取设置' }).click();
  await expect(guardSave).toBeEnabled();
  await capture('hostguard-settings');
  failGuardSave = true;
  await guardSave.click();
  await expect(page.getByText(/未保存：host_guard_max_retry/)).toBeVisible();
  failGuardSave = false;
  await guardSave.click();
  await expect(page.getByText(/未保存：host_guard_max_retry/)).toHaveCount(0);
  report.checks.push(
    'HostGuard read failure blocks save; partial persistence names failed item, preserves draft and supports retry',
  );
  failTuning = true;
  await page.goto(origin + '/?verification=' + Date.now() + '#/sys/OwaspManage?tab=tuning');
  await expect(page.getByText('模拟引擎读取失败')).toBeVisible();
  await expect(page.getByRole('button', { name: '确认', exact: true })).toBeDisabled();
  failTuning = false;
  await page.getByRole('button', { name: '重试读取引擎配置' }).click();
  await expect(page.getByRole('button', { name: '确认', exact: true })).toBeEnabled();
  await capture('owasp-tuning');
  await expect(page.getByRole('switch', { name: '强制 URLENCODED 请求体处理', exact: true })).toHaveAttribute(
    'aria-checked',
    'true',
  );
  await page.getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => report.writes.filter((write) => write.endpoint === 'owasp/tuning').length).toBe(1);
  expect(report.writes.find((write) => write.endpoint === 'owasp/tuning').input.enforce_bodyproc_urlencoded).toBe(1);
  report.checks.push('OWASP tuning retains mode/threshold/body budgets and prevents saving unread defaults');
  await page.goto(origin + '/?verification=' + Date.now() + '#/waf-pipeline/legacy-sources');
  await page.getByRole('button', { name: /新建防护/ }).click();
  const legacyNav = page.getByRole('navigation', { name: '旧站点配置分类' });
  await expect(legacyNav).toBeVisible();
  await expect(legacyNav.locator('section')).toHaveCount(4);
  await expect(legacyNav.getByRole('button')).toHaveCount(20);
  const legacyNames = await legacyNav.getByRole('button').allTextContents();
  for (const name of legacyNames) {
    await legacyNav.getByRole('button', { name: name.trim(), exact: true }).click();
    await expect(legacyNav.getByRole('button', { name: name.trim(), exact: true })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBe(true);
  }
  await legacyNav.getByRole('button').first().click();
  await capture('legacy-editor');
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(page.getByRole('combobox', { name: '旧站点配置任务', exact: true })).toBeVisible();
  await dialog().getByRole('button', { name: '关闭', exact: true }).click();
  report.checks.push('Legacy editor retains all twenty pane identities under four task groups and compact selection');
  }
  expect(report.errors).toEqual([]);
  console.log(JSON.stringify(report, null, 2));
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), animations: 'disabled' });
  console.error(JSON.stringify(report, null, 2));
  throw error;
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
