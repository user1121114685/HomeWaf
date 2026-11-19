// Isolated lifecycle UI fixtures. No CA, DNS or notification provider is contacted.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { handleTransportHandshake, routeSession } from './transport-fixture.mjs';
const root = fileURLToPath(new URL('../..', import.meta.url));
const dist = path.join(root, 'web/dist'),
  output = path.join(root, 'artifacts/certificate-lifecycle');
fs.mkdirSync(output, { recursive: true });
const catalogue = JSON.parse(fs.readFileSync(path.join(root, 'internal/dnsproviders/catalogue.json'), 'utf8'));
const policy = {
  id: 'cert-one',
  version: 1,
  enabled: true,
  status: 'retry_wait',
  host_code: 'pipeline-80',
  apply_domain: 'nas.example.test',
  apply_platform: 'letsencrypt',
  apply_method: 'dns01',
  apply_dns: 'hurricane',
  private_group_name: 'household',
  apply_email: 'owner@example.test',
  skip_dns_verify: 0,
  last_error: 'DNS 查询超时',
  failure_count: 1,
  next_attempt_at: '2026-11-18 14:30:00',
  last_order_id: 'failed-one',
};
const record = {
  id: 'cert-one',
  domains: 'nas.example.test',
  issuer: 'Example Test CA',
  valid_from: '2026-09-01 00:00:00',
  valid_to: '2026-12-01 00:00:00',
  validity: 'expiring',
  update_method: 'managed',
  renewal: policy,
  auto_load_path: 0,
  cert_path: '/etc/homewaf/nas.crt',
  key_path: '/etc/homewaf/nas.key',
  export_status: '导出成功',
  bindings: [
    {
      kind: 'pipeline',
      id: 'service-one',
      code: 'pipeline-80',
      port: 80,
      protocol: 'http',
      name: '家庭 NAS',
      listening: false,
      source: 'sslconfig:cert-one',
    },
  ],
  cert_content:
    '-----BEGIN CERTIFICATE-----\n' + Buffer.from('fixture').toString('base64') + '\n-----END CERTIFICATE-----',
  key_content:
    '-----BEGIN PRIVATE KEY-----\n' + Buffer.from('fixture key').toString('base64') + '\n-----END PRIVATE KEY-----',
};
const job = {
  id: 'failed-one',
  certificate_id: 'cert-one',
  apply_domain: record.domains,
  host_code: policy.host_code,
  apply_status: 'fail',
  trigger: 'automatic',
  operation: 'renewal',
  stage: 'issue',
  result_error: policy.last_error,
  progress: '申请失败',
  create_time: '2026-11-17 12:00:00',
  auto_renew: true,
  apply_platform: 'letsencrypt',
  apply_method: 'dns01',
  renewal: policy,
};
let failCredential = true,
  credentialWrites = 0,
  failList = false,
  failPolicy = false,
  failMonitor = false,
  policyWrites = 0,
  renewWrites = 0,
  requestWrites = 0,
  targetState = 'normal',
  monitorWrites = 0;
let monitors = [
  {
    id: 'monitor-one',
    domain: 'nas.example.test',
    port: 8443,
    status: 'expired',
    valid_to: '2026-11-01 00:00:00',
    last_detect: '2026-11-17 12:30:00',
    last_success: '2026-11-17 12:30:00',
    expiration_info: '已过期 16 天',
    expiration_day: -16,
    visit_log: '',
  },
];
const report = { checks: [], views: [], errors: [] };
const requestPayloads = [];
const server = http.createServer((req, res) => {
  const file = path.resolve(
    dist,
    '.' +
      (new URL(req.url, 'http://localhost').pathname === '/'
        ? '/index.html'
        : decodeURIComponent(new URL(req.url, 'http://localhost').pathname)),
  );
  if (!file.startsWith(dist + path.sep) || !fs.existsSync(file)) {
    res.writeHead(404);
    res.end();
    return;
  }
  res.setHeader(
    'Content-Type',
    {
      '.html': 'text/html',
      '.js': 'text/javascript',
      '.css': 'text/css',
      '.svg': 'image/svg+xml',
      '.woff2': 'font/woff2',
    }[path.extname(file)] || 'application/octet-stream',
  );
  fs.createReadStream(file).pipe(res);
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const origin = 'http://127.0.0.1:' + server.address().port;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
  reducedMotion: 'reduce',
  serviceWorkers: 'block',
});
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-certificate-lifecycle');
  localStorage.setItem('lang', 'zh_CN');
});
await context.route('**/*', (route) =>
  new URL(route.request().url()).origin === origin ? route.continue() : route.abort(),
);
await context.routeWebSocket('**/api/**', (socket) => socket.close());
await context.route('**/api/**', async (route) => {
  if (await handleTransportHandshake(route)) return;
  const req = route.request(),
    url = new URL(req.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  const respond = (code, data, msg = '') =>
    route.fulfill({ json: { code, data: routeSession(route).encrypt(JSON.stringify(data)), msg } });
  const ok = (data, msg = '成功') => respond(0, data, msg),
    fail = (msg) => respond(-1, {}, msg);
  let data = {};
  if (req.postData()) {
    data = JSON.parse(routeSession(route).decrypt(req.postData()));
  }
  if (endpoint === 'sysinfo/checkversion')
    return fail('Isolated fixture');
  if (endpoint === 'certificates/list') {
    const list = data.domains && data.domains !== 'nas.example.test' ? [] : [record];
    return failList ? fail('证书列表读取失败') : ok({ list, total: list.length });
  }
  if (endpoint === 'certificates/detail') return ok(record);
  if (endpoint === 'certificates/policy') {
    policyWrites++;
    if (failPolicy) return fail('保存失败，请重试');
    expect(data.id).toBe(record.id);
    expect(data.version).toBe(policy.version);
    Object.assign(policy, data, { version: policy.version + 1, status: 'idle', last_error: '' });
    return ok(policy, '续期设置已保存，未发起签发任务');
  }
  if (endpoint === 'certificates/renew') {
    renewWrites++;
    expect(Object.keys(data).sort()).toEqual(['id', 'request_id']);
    expect(data.id).toBe(record.id);
    expect(data.request_id).toBeTruthy();
    policy.status = 'queued';
    policy.last_order_id = 'renew-one';
    policy.next_attempt_at = '';
    return ok({ id: 'renew-one', apply_status: 'submitted' }, '任务已受理');
  }
  if (endpoint === 'certificates/settings')
    return ok({
      timezone: 'Asia/Shanghai',
      renew_days: 30,
      ip_renew_days: 3,
      file_sync_enabled: false,
      tasks: [{ id: 'task-one', task_name: '自动续期检查', task_unit: 'day', task_value: 1, task_at: '02:13' }],
    });
  if (endpoint === 'certificates/notifications')
    return ok({
      operation_subscriptions: 0,
      expiry_subscriptions: 1,
      events: [
        {
          id: 'event-one',
          kind: 'failure',
          message: 'DNS 查询超时；将在下一次计划重试',
          create_time: '2026-11-17 12:10:00',
          expanded: true,
          coverage: 'no_subscription',
        },
      ],
      deliveries: [],
    });
  if (endpoint === 'wafhost/privateinfo/edit') {
    credentialWrites++;
    if (failCredential) return fail('凭据保存失败，请重试');
    expect(data.private_value).toBe('example.test:fixture-only-token');
    return ok({}, '凭据已保存');
  }
  if (endpoint === 'certificates/bind-targets')
    return ok([
      {
        kind: 'pipeline',
        id: 'service-one',
        code: 'pipeline-80',
        name: '家庭 NAS',
        port: 80,
        protocol: 'http',
        fingerprint: 'original',
      },
    ]);
  if (endpoint === 'certificates/bind') {
    expect(data).toEqual({
      id: 'cert-one',
      host_code: 'pipeline-80',
      target_id: 'service-one',
      fingerprint: 'original',
    });
    return ok({}, '关联已保存');
  }
  if (endpoint === 'sslconfig/detail') return ok({ ...record, bind_hosts: ['家庭 NAS :80'] });
  if (endpoint === 'sslconfig/list') return ok({ list: [record], total: 1 });
  if (endpoint === 'wafhost/sslorder/list') return ok({ list: [job], total: 1 });
  if (endpoint === 'wafhost/sslorder/logs')
    return ok({
      ...job,
      id: url.searchParams.get('id'),
      renewal: policy,
      entries: [{ id: 1, time: '2026-11-17 12:00:00', level: 'error', message: 'DNS 查询超时' }],
      limit: 500,
    });
  if (endpoint === 'wafhost/sslorder/targets' && targetState === 'error') return fail('服务列表读取失败');
  if (endpoint === 'wafhost/sslorder/targets' && targetState === 'empty') return ok([]);
  if (endpoint === 'wafhost/sslorder/targets')
    return ok([
      {
        value: 'pipeline-80',
        label: '家庭 NAS · HTTP :80',
        source: 'pipeline',
        port: 80,
        protocol: 'http',
        domains: ['nas.example.test'],
        http01_ready: true,
      },
      {
        value: 'pipeline-8443',
        label: '相册 · HTTPS :8443',
        source: 'pipeline',
        port: 8443,
        protocol: 'https',
        domains: ['photos.example.test'],
        http01_ready: false,
      },
    ]);
  if (endpoint === 'wafhost/sslorder/dns/providers') return ok(catalogue);
  if (endpoint === 'wafhost/sslorder/dns/credentials') return ok([{ private_key: 'HURRICANE_TOKENS' }]);
  if (endpoint === 'wafhost/privategroup/listbybelongcloud')
    return ok({ list: [{ private_group_name: 'household' }], total: 1 });
  if (endpoint === 'wafhost/caserverinfo/list')
    return ok({
      list: [
        {
          id: 'ca-one',
          ca_server_name: 'letsencrypt',
          ca_server_address: 'https://example.test/directory',
          remarks: '默认 CA',
        },
        { id: 'ca-two', ca_server_name: 'zerossl', ca_server_address: 'https://example.test/zero', remarks: 'ZeroSSL' },
      ],
      total: 2,
    });
  if (endpoint === 'wafhost/sslorder/zerossl/account')
    return ok({ configured: false, has_access_key: false, has_eab_kid: false, has_eab_hmac_key: false });
  if (endpoint === 'wafhost/sslorder/add') {
    requestWrites++;
    requestPayloads.push(structuredClone(data));
    return ok({ id: 'new-' + requestWrites, apply_status: 'submitted' }, '申请已受理');
  }
  if (endpoint === 'wafhost/sslexpire/list')
    return failMonitor ? fail('监测列表读取失败') : ok({ list: monitors, total: monitors.length });
  if (endpoint === 'wafhost/sslexpire/nowcheck') {
    if (url.searchParams.get('id')) expect(url.searchParams.get('id')).toBe('monitor-one');
    return ok({}, '此目标正在检测');
  }
  if (endpoint === 'wafhost/sslexpire/add') {
    monitorWrites++;
    if (!data.domain) return fail('请填写域名');
    monitors.push({ ...data, id: 'monitor-' + monitorWrites, status: 'unverified' });
    return ok({}, '添加成功');
  }
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.setDefaultTimeout(10000);
page.on('pageerror', (e) => report.errors.push(e.message));
const check = (name) => {
  report.checks.push(name);
  console.log('PASS ' + name);
};
async function go(route) {
  await page.goto(origin + '/#' + route);
  await page.locator('.certificate-page').first().waitFor();
}
try {
  await go('/waf-host/wafsslconfiglist?domains=not-found');
  await expect(page.getByText('没有符合条件的证书', { exact: true })).toBeVisible();
  await page.screenshot({ path: path.join(output, 'list-no-matches.png'), fullPage: true });
  failList = true;
  await go('/waf-host/wafsslconfiglist');
  await expect(page.getByRole('alert').filter({ hasText: '证书列表读取失败' })).toBeVisible();
  await page.screenshot({ path: path.join(output, 'list-read-failure.png'), fullPage: true });
  failList = false;
  await page.getByRole('button', { name: '重试', exact: true }).click();
  await expect(page.locator('[data-certificate-id="cert-one"]')).toContainText('失败，等待重试');
  await page.locator('[data-certificate-id="cert-one"]').getByRole('link', { name: '续期设置', exact: true }).click();
  await expect(page.getByRole('button', { name: '保存续期设置', exact: true })).toBeVisible();
  await page.locator('.el-checkbox').filter({ hasText: '启用自动续期' }).click();
  failPolicy = true;
  await page.getByRole('button', { name: '保存续期设置', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: '输入已保留' })).toBeVisible();
  await expect(page.getByRole('checkbox', { name: '启用自动续期', exact: true })).not.toBeChecked();
  expect(renewWrites).toBe(0);
  failPolicy = false;
  await page.getByRole('button', { name: '保存续期设置', exact: true }).click();
  await expect(page.getByRole('button', { name: '编辑续期设置', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByText('自动续期已关闭', { exact: true })).toBeVisible();
  expect(renewWrites).toBe(0);
  check('policy saves independently; failed save retains draft; reopen preserves opt-out');
  await page.getByRole('button', { name: '编辑续期设置', exact: true }).click();
  await page.locator('.el-checkbox').filter({ hasText: '启用自动续期' }).click();
  await page.getByRole('button', { name: '取消', exact: true }).click();
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: '启用自动续期', exact: true })).toBeChecked();
  await page.getByRole('button', { name: '取消', exact: true }).click();
  await page.getByRole('button', { name: '放弃修改', exact: true }).click();
  await page.getByRole('button', { name: '立即续期', exact: true }).click();
  await page.getByRole('button', { name: '开始处理', exact: true }).click();
  await expect(page.getByRole('dialog', { name: '申请与续期任务详情' })).toBeVisible();
  expect(renewWrites).toBe(1);
  expect(policy.enabled).toBe(false);
  await page.getByRole('button', { name: '关闭', exact: true }).click();
  check('cancel protects the draft; manual renewal uses certificate identity and does not enable auto-renewal');
  await expect(page.getByText('未配置启用的操作通知渠道', { exact: false })).toBeVisible();
  await expect(page.getByText('未找到对应订阅，未向外部渠道投递。')).toBeVisible();
  check('missing failure-notification coverage is visible separately from expiry subscription');
  await go('/waf-host/wafsslOrderlist?certificate_id=cert-one');
  await expect(page.getByRole('button', { name: '手动续期', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: '日志', exact: true }).click();
  await expect(page.getByRole('dialog', { name: '申请与续期任务详情' })).toContainText('当前恢复安排');
  await page.getByRole('button', { name: '关闭', exact: true }).click();
  check('history shows execution and recovery; renewal configuration has one owner');
  await go('/waf-host/wafsslconfiglist/request');
  await expect(page.getByRole('button', { name: '提交申请', exact: true })).toBeVisible();
  await expect(
    page.locator('#console-main').getByText('DNS 验证（家庭网络推荐，无需入站端口）', { exact: true }),
  ).toBeVisible();
  await page.getByRole('combobox', { name: '关联服务', exact: true }).click();
  await page.getByRole('option', { name: '家庭 NAS · HTTP :80', exact: true }).click();
  const email = page
    .locator('.certificate-request-form .el-form-item')
    .filter({ hasText: '邮箱' })
    .locator('input')
    .first();
  await email.fill('owner@example.test');
  const domainField = page
    .locator('.certificate-request-form .el-form-item')
    .filter({ hasText: '申请域名' })
    .locator('textarea');
  await domainField.fill('independent.example.test');
  const serviceSelect = page
    .getByRole('combobox', { name: '关联服务', exact: true })
    .locator('xpath=ancestor::div[contains(@class,"el-select__wrapper")][1]');
  await serviceSelect.hover();
  await serviceSelect.locator('.el-select__clear').click();
  await expect(domainField).toHaveValue('independent.example.test');
  await page.getByRole('button', { name: '取消申请', exact: true }).click();
  await page.getByRole('button', { name: '继续填写', exact: true }).click();
  await expect(email).toHaveValue('owner@example.test');
  await page.getByRole('button', { name: '取消申请', exact: true }).click();
  await page.getByRole('button', { name: '放弃申请', exact: true }).click();
  await expect(page).toHaveURL(/#\/waf-host\/wafsslconfiglist$/);
  await expect(page.locator('.certificate-request-form')).toHaveCount(0);
  check('application is a separate task page with DNS default and draft recovery');
  await go('/waf-host/wafsslconfiglist/request?host_code=pipeline-80');
  await page
    .locator('.certificate-request-form .el-form-item')
    .filter({ hasText: '邮箱' })
    .locator('input')
    .first()
    .fill('owner@example.test');
  await page
    .locator('.certificate-request-form .el-form-item')
    .filter({ hasText: '申请方式' })
    .locator('.el-select__wrapper')
    .click();
  await page.getByRole('option', { name: 'HTTP 文件验证（需要公网 80 端口）', exact: true }).click();
  await page.getByRole('button', { name: '提交申请', exact: true }).click();
  await expect(page.getByRole('dialog', { name: '申请与续期任务详情' })).toBeVisible();
  expect(requestWrites).toBe(1);
  expect(requestPayloads.at(-1).host_code).toBe('pipeline-80');
  await page.getByRole('button', { name: '关闭', exact: true }).click();
  check('new application submits once and lands on its returned task');
  for (const state of ['empty', 'error']) {
    targetState = state;
    await go('/waf-host/wafsslconfiglist/request');
    await expect(
      page.getByText(state === 'empty' ? /暂无服务也可以申请/ : /不关联服务仍可使用 DNS 验证申请/),
    ).toBeVisible();
    await domainField.fill('independent.example.test');
    await email.fill('owner@example.test');
    const provider = page.getByRole('combobox', { name: 'DNS 服务商', exact: true });
    await provider.focus();
    await provider.press('ArrowDown');
    await page.getByRole('option', { name: /Hurricane Electric DNS/ }).click();
    const group = page.getByRole('combobox', { name: '密钥分组', exact: true });
    await group.focus();
    await group.press('ArrowDown');
    await page.getByRole('option', { name: 'household', exact: true }).click();
    await page.getByRole('button', { name: '提交申请', exact: true }).click();
    await expect(page.getByRole('dialog', { name: '申请与续期任务详情' })).toBeVisible();
    expect(requestPayloads.at(-1).host_code).toBe('');
    expect(requestPayloads.at(-1).apply_domain).toBe('independent.example.test');
    expect(requestPayloads.at(-1).apply_method).toBe('dns01');
    await page.getByRole('button', { name: '关闭', exact: true }).click();
  }
  targetState = 'normal';
  expect(requestWrites).toBe(3);
  check(
    'clearing optional service preserves domains; empty or failed service lists do not block independent DNS issuance',
  );
  await go('/waf-host/wafsslconfiglist/settings?provider=hurricane&group=household');
  await page.getByRole('button', { name: '配置 HURRICANE_TOKENS', exact: true }).click();
  const credentialDialog = page.getByRole('dialog', { name: '编辑', exact: true });
  await credentialDialog.getByRole('checkbox', { name: /更新 DDNS/ }).check();
  await credentialDialog.getByRole('textbox', { name: '验证域名 1', exact: true }).fill('example.test');
  await credentialDialog.getByLabel('DDNS Key 1', { exact: true }).fill('fixture-only-token');
  await credentialDialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect(credentialDialog.getByRole('alert')).toContainText('凭据保存失败');
  await expect(credentialDialog.getByLabel('DDNS Key 1', { exact: true })).toHaveValue('fixture-only-token');
  failCredential = false;
  await credentialDialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect(credentialDialog).toBeHidden();
  expect(credentialWrites).toBe(2);
  check('standalone credential maintenance preserves failed input and saves through its existing API');
  await go('/waf-host/wafsslconfiglist/cert-one');
  await page.getByRole('button', { name: '关联服务', exact: true }).click();
  const bindingDialog = page.getByRole('dialog', { name: '关联服务', exact: true });
  await bindingDialog.getByRole('combobox').click();
  await page.getByRole('option', { name: '家庭 NAS :80', exact: true }).click();
  await bindingDialog.getByRole('button', { name: '确认关联', exact: true }).click();
  await expect(bindingDialog).toBeHidden();
  check('certificate binding carries the explicit service identity and prior fingerprint');

  await go('/waf-host/wafsslExpirelist');
  await expect(page.getByText('已过期', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '重新检查', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: '此目标正在检测' })).toBeVisible();
  await page.getByRole('button', { name: '添加监测', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '添加监测目标' });
  await dialog.getByRole('textbox').fill('camera.example.test');
  await dialog.getByRole('button', { name: '保存目标', exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(monitorWrites).toBe(1);
  failMonitor = true;
  await page.getByRole('button', { name: '刷新结果', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: '读取监测目标失败' })).toBeVisible();
  failMonitor = false;
  await page.getByRole('button', { name: '重试', exact: true }).click();
  check('monitoring distinguishes expiry and read failures; a scoped probe and target save remain separate');
  for (const [width, height] of [
    [1440, 1000],
    [768, 1024],
    [390, 844],
  ]) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark']) {
      await page.evaluate((theme) => {
        document.documentElement.setAttribute('theme-mode', theme);
        document.documentElement.classList.toggle('dark', theme === 'dark');
        localStorage.setItem('theme-mode', theme);
      }, theme);
      for (const [label, route] of [
        ['list', '/waf-host/wafsslconfiglist'],
        ['detail', '/waf-host/wafsslconfiglist/cert-one'],
        ['history', '/waf-host/wafsslOrderlist'],
        ['monitor', '/waf-host/wafsslExpirelist'],
        ['settings', '/waf-host/wafsslconfiglist/settings'],
        ['application', '/waf-host/wafsslconfiglist/request'],
      ]) {
        await go(route);
        await page.evaluate((theme) => {
          document.documentElement.setAttribute('theme-mode', theme);
          document.documentElement.classList.toggle('dark', theme === 'dark');
        }, theme);
        await page.waitForTimeout(200);
        const file = label + '-' + width + '-' + theme + '.png';
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 2);
        report.views.push({ label, width, theme, file, overflow });
        expect(overflow, label + ' overflow at ' + width).toBe(false);
      }
    }
  }
  expect(report.errors).toEqual([]);
  const stored = await page.evaluate(() => JSON.stringify({ ...localStorage, ...sessionStorage }));
  expect(stored).not.toContain('PRIVATE KEY');
  check(
    'six task surfaces render at desktop/tablet/phone in both themes without page overflow or private-key persistence',
  );
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  fs.writeFileSync(path.join(output, 'failure.txt'), await page.locator('body').innerText());
  throw error;
} finally {
  fs.writeFileSync(path.join(output, 'browser-results.json'), JSON.stringify(report, null, 2));
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
