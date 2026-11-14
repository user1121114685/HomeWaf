// Isolated API fixtures: no real credentials, DNS updates, or CA orders.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';
import { serializeHurricaneCredentials } from '../src/pages/waf/sslorder/hurricane.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const output = path.join(root, 'artifacts', 'dns-orders-review');
fs.mkdirSync(output, { recursive: true });
const catalogue = JSON.parse(fs.readFileSync(path.join(root, 'internal/dnsproviders/catalogue.json'), 'utf8'));
const origin = process.env.CONSOLE_TEST_ORIGIN || 'http://127.0.0.1:3001';
const errors = [],
  checks = [],
  writes = [],
  listQueries = [],
  credentials = [];
let failHosts = false,
  emptyHosts = false;
let failCatalogue = false,
  failStatus = false,
  failSave = false;
let failZeroSSLStatus = false,
  failZeroSSLSave = false;
let zeroSSLAccount = { configured: false, has_access_key: false, has_eab_kid: false, has_eab_hmac_key: false };
let holdOrder = false, releaseOrder, failOrderOnce = false;
let activeOrder = null;
let retryOrder = null;
let failLogs = false, holdRetry = false, releaseRetry, failRetryOnce = false;
let failList = false;
const retryKeys = new Set();
const renewal = {
  id: 'existing-order',
  auto_renew: false,
  result_valid_to: new Date(Date.now() + 60 * 86400000).toISOString(),
  host_code: 'pipeline-15000',
  apply_platform: 'letsencrypt',
  apply_method: 'dns01',
  apply_dns: 'hurricane',
  private_group_name: 'household',
  apply_email: 'owner@example.com',
  apply_domain: 'example.com,*.example.com',
  apply_status: 'success',
  skip_dns_verify: 0,
};
const interruptedOrder = { ...renewal, id: 'interrupted-order', apply_status: 'fail', progress: '申请任务已中断', result_error: '证书任务超过 10 分钟未更新运行状态，请确认服务稳定运行后重试。' };
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-dns-fixture');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
// Verify the external destination without logging into a real ZeroSSL account.
await context.route('https://app.zerossl.com/developer', (route) =>
  route.fulfill({ contentType: 'text/html', body: '<title>ZeroSSL developer fixture</title>' }),
);
await context.route('**/api/**', async (route) => {
  const request = route.request(),
    url = new URL(request.url());
  const endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '成功' } });
  const fail = (msg) => route.fulfill({ json: { code: -1, data: AesEncrypt('{}'), msg } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint))
    return fail('Isolated fixture');
  let input = {};
  if (request.postData()) {
    try {
      input = JSON.parse(AesDecrypt(request.postData()));
    } catch {
      try {
        input = JSON.parse(request.postData());
      } catch {}
    }
  }
  if (endpoint === 'wafhost/sslorder/zerossl/account') {
    if (request.method() === 'GET') return failZeroSSLStatus ? fail('认证状态读取失败') : ok(zeroSSLAccount);
    if (failZeroSSLSave) {
      failZeroSSLSave = false;
      return fail('认证保存失败，请重试');
    }
    writes.push({ endpoint, input });
    zeroSSLAccount =
      input.mode === 'access_key'
        ? { configured: true, has_access_key: true, has_eab_kid: false, has_eab_hmac_key: false }
        : { ...zeroSSLAccount, configured: true, has_eab_kid: true, has_eab_hmac_key: true };
    return ok(zeroSSLAccount);
  }
  if (endpoint === 'wafhost/sslorder/targets')
    return failHosts
      ? fail('服务读取失败')
      : ok(
          emptyHosts
            ? []
            : [
                {
                  value: 'pipeline-15000',
                  label: '家庭 NAS · HTTP :15000',
                  source: 'pipeline',
                  protocol: 'http',
                  port: 15000,
                  domains: ['example.com', '*.example.com'],
                  http01_ready: false,
                },
              ],
        );
  if (endpoint === 'wafhost/sslorder/dns/providers') return failCatalogue ? fail('目录暂不可用') : ok(catalogue);
  if (endpoint === 'wafhost/sslorder/dns/credentials')
    return failStatus
      ? fail('读取失败')
      : ok(
          credentials
            .filter(
              (c) =>
                c.private_group_belong_cloud === url.searchParams.get('provider') &&
                c.private_group_name === url.searchParams.get('group'),
            )
            .map(({ private_value, ...rest }) => rest),
        );
  if (endpoint === 'wafhost/privategroup/listbybelongcloud')
    return ok({ list: [{ private_group_name: 'default' }, { private_group_name: 'household' }], total: 2 });
  if (endpoint === 'wafhost/privateinfo/add' || endpoint === 'wafhost/privateinfo/edit') {
    if (failSave) {
      failSave = false;
      return fail('保存失败，请稍后重试');
    }
    const index = credentials.findIndex(
      (c) =>
        c.private_key === input.private_key &&
        c.private_group_name === input.private_group_name &&
        c.private_group_belong_cloud === input.private_group_belong_cloud,
    );
    if (index < 0) credentials.push({ id: 'credential-' + credentials.length, ...input });
    else
      credentials[index] = {
        ...credentials[index],
        ...input,
        private_value: input.private_value === '****' ? credentials[index].private_value : input.private_value,
      };
    writes.push({ endpoint, input });
    return ok({});
  }
  if (endpoint === 'wafhost/host/allhost') return ok([{ value: 'global', label: '全局网站:0', global_host: 1 }]);
  if (endpoint === 'wafhost/host/alldomainbyhostcode')
    return ok([{ label: 'example.com' }, { label: '*.example.com' }]);
  if (endpoint === 'wafhost/host/detail')
    return ok({
      id: 'nas',
      code: 'nas',
      host: 'example.com',
      port: 8443,
      ssl: 1,
      port_listens_json: '[{"port":8443,"protocol":"https"}]',
    });
  if (endpoint === 'wafhost/caserverinfo/list' || endpoint === 'wafhost/ca_server_info/list')
    return ok({
      list: [
        {
          id: 'le-ca',
          ca_server_name: 'letsencrypt',
          ca_server_address: 'https://acme-v02.api.letsencrypt.org/directory',
          remarks: "Let's Encrypt",
        },
        {
          id: 'zero-ca',
          ca_server_name: 'zerossl',
          ca_server_address: 'https://acme.zerossl.com/v2/DV90',
          remarks: 'ZeroSSL',
        },
      ],
      total: 2,
    });
  if (endpoint === 'wafhost/sslorder/list') {
    if (failList) return fail('申请列表暂不可用');
    listQueries.push({ ...input });
    const history = Array.from({ length: 14 }, (_, index) => ({ ...renewal, id: `history-${index}`, apply_domain: 'older.example.net', apply_status: 'fail' }));
    const rows = [renewal, ...(activeOrder ? [activeOrder] : []), interruptedOrder, ...(retryOrder ? [retryOrder] : []), ...history]
      .filter((row) => !input.host_code || row.host_code === input.host_code)
      .filter((row) => !input.apply_domain || row.apply_domain.includes(input.apply_domain))
      .filter((row) => !input.apply_status || row.apply_status === input.apply_status);
    const start = ((input.pageIndex || 1) - 1) * (input.pageSize || 10);
    return ok({ list: rows.slice(start, start + (input.pageSize || 10)), total: rows.length });
  }
  if (endpoint === 'wafhost/sslorder/logs') {
    if (failLogs) return fail('日志暂不可用，请重试');
    const current = url.searchParams.get('id') === retryOrder?.id ? retryOrder : interruptedOrder;
    return ok({ ...current, update_time: '2026-11-15 00:05:00', heartbeat_stale: !!current.heartbeat_stale, legacy: current === interruptedOrder, limit: 500,
      entries: current === interruptedOrder
        ? Array.from({ length: 160 }, (_, i) => ({ id: i + 1, time: '2026-11-15 00:05:00', level: 'info', message: `DNS 传播检查 ${i + 1}：token=[REDACTED]；日志文本 <img src=x onerror=alert(1)>；记录 _acme-challenge.${'long-domain-'.repeat(12)}example.com，等待服务商权威服务器同步。` }))
        : [{ id: 1, time: '2026-11-15 00:05:00', level: 'info', message: '开始重试，正在检查 CA 与 DNS 配置' }] });
  }
  if (endpoint === 'wafhost/sslorder/retry') {
    writes.push({ endpoint, input });
    if (!retryKeys.has(input.request_id)) {
      retryKeys.add(input.request_id);
      retryOrder = { ...renewal, id: 'retried-order', apply_status: 'submitted', progress: '等待证书任务处理', result_error: '' };
    }
    if (holdRetry) await new Promise((resolve) => { releaseRetry = resolve; });
    if (failRetryOnce) { failRetryOnce = false; return route.abort('failed'); }
    return ok({ id: retryOrder.id, apply_status: retryOrder.apply_status });
  }
  if (endpoint === 'wafhost/sslorder/add' || endpoint === 'wafhost/sslorder/edit') {
    writes.push({ endpoint, input });
    if (holdOrder) await new Promise((resolve) => { releaseOrder = resolve; });
    if (failOrderOnce) { failOrderOnce = false; return route.abort('failed'); }
    if (endpoint.endsWith('/add')) activeOrder = { ...input, id: 'new-order', apply_status: 'submitted', progress: '等待证书任务处理' };
    return ok({ id: 'new-order', apply_status: 'submitted' });
  }
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.setDefaultTimeout(10000);
page.on('pageerror', (e) => errors.push(e.stack || e.message));
const activeDialog = () => page.locator('.el-dialog:visible').last();
async function select(control, name) {
  await control.press('ArrowDown');
  await page.getByRole('option', { name, exact: true }).click();
}
try {
  expect(serializeHurricaneCredentials([{ domain: 'EXAMPLE.COM.', token: 'fixture-token' }])).toBe(
    'example.com:fixture-token',
  );
  expect(() => serializeHurricaneCredentials([{ domain: '*.example.com', token: 'token' }])).toThrow();
  await page.goto(origin + '/?appearance=light#/waf-host/wafsslOrderlist');
  await page.locator('.search-input input').fill('older.example.net');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  await page.locator('.el-pagination .btn-next').click();
  await expect.poll(() => listQueries.at(-1)?.pageIndex).toBe(2);
  await page.getByRole('button', { name: '申请证书', exact: true }).click();
  let dialog = activeDialog();
  await expect(dialog.getByRole('checkbox', { name: '启用自动续期', exact: true })).toBeChecked();
  await expect(dialog.getByTestId('dns-config')).toBeVisible();
  await expect(dialog.getByText(/无需开放或转发入站 80 \/ 443/)).toBeVisible();
  await expect(dialog).toContainText('DNS 验证（家庭网络推荐，无需入站端口）');
  await dialog
    .locator('.el-select')
    .filter({ has: page.getByRole('combobox', { name: '关联服务', exact: true }) })
    .click();
  await page.getByRole('option', { name: '家庭 NAS · HTTP :15000', exact: true }).click();
  await expect(dialog.getByText(/当前 HTTP 服务不会自动切换为 HTTPS/)).toBeVisible();
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    await dialog.getByRole('combobox', { name: '关联服务', exact: true }).scrollIntoViewIfNeeded();
    expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
    await page.screenshot({ path: path.join(output, `service-selector-${width}.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  const firstProvider = dialog.getByRole('combobox', { name: 'DNS 服务商', exact: true });
  await firstProvider.fill('dns.he.net');
  await page.getByRole('option', { name: 'dns.he.net（Hurricane Electric DNS） · hurricane', exact: true }).click();
  await expect(dialog.getByTestId('hurricane-guide')).toContainText('Enable entry for dynamic dns');
  await expect(dialog.getByRole('button', { name: '配置 HURRICANE_TOKENS', exact: true })).toBeEnabled();
  checks.push('DNS is the default and no public 80/443 listener is requested');
  for (const appearance of ['light', 'dark']) {
    await page.evaluate((mode) => {
      document.documentElement.classList.toggle('dark', mode === 'dark');
      document.documentElement.setAttribute('data-theme', mode);
    }, appearance);
    for (const width of [1440, 768, 390, 320]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      await dialog.getByTestId('hurricane-guide').scrollIntoViewIfNeeded();
      expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
      await page.screenshot({ path: path.join(output, `hurricane-${appearance}-${width}.png`), fullPage: true });
      await dialog.getByRole('button', { name: '配置 HURRICANE_TOKENS', exact: true }).scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(output, `fields-${appearance}-${width}.png`), fullPage: true });
    }
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await dialog.getByRole('button', { name: '配置 HURRICANE_TOKENS', exact: true }).click();
  let credentialDialog = activeDialog();
  await credentialDialog.getByRole('textbox', { name: '验证域名 1', exact: true }).fill('example.com');
  await credentialDialog.getByLabel('DDNS Key 1', { exact: true }).fill('fixture-root-token');
  await credentialDialog.getByRole('button', { name: '添加验证域名', exact: true }).click();
  await credentialDialog.getByRole('textbox', { name: '验证域名 2', exact: true }).fill('nas.example.com');
  await credentialDialog.getByLabel('DDNS Key 2', { exact: true }).fill('fixture-nas-token');
  for (const width of [1440, 768, 390, 320]) {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    await credentialDialog.getByLabel('DDNS Key 2', { exact: true }).scrollIntoViewIfNeeded();
    expect(await credentialDialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
    await page.screenshot({ path: path.join(output, `ddns-key-${width}.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  failSave = true;
  await credentialDialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect(page.getByText('保存失败，请稍后重试', { exact: true })).toBeVisible();
  await expect(credentialDialog.getByLabel('DDNS Key 1', { exact: true })).toHaveValue('fixture-root-token');
  await credentialDialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => credentials.length).toBe(1);
  expect(credentials[0].private_value).toBe('example.com:fixture-root-token,nas.example.com:fixture-nas-token');
  dialog = activeDialog();
  await expect(dialog.getByRole('button', { name: '配置 HURRICANE_TOKENS', exact: true })).toHaveText('修改');
  await dialog.getByLabel(/申请邮箱/).fill('owner@example.com');
  holdOrder = true;
  failOrderOnce = true;
  await dialog.getByRole('button', { name: '确认', exact: true }).dblclick();
  await expect.poll(() => writes.filter((w) => w.endpoint.endsWith('/sslorder/add')).length).toBe(1);
  await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeDisabled();
  await dialog.locator('form').evaluate((form) => { form.requestSubmit(); form.requestSubmit(); });
  expect(writes.filter((w) => w.endpoint.endsWith('/sslorder/add')).length).toBe(1);
  const firstRequestID = writes.find((w) => w.endpoint.endsWith('/sslorder/add')).input.request_id;
  expect(firstRequestID).toMatch(/^[0-9a-f-]{36}$/);
  holdOrder = false;
  releaseOrder();
  await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeEnabled();
  await dialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => writes.filter((w) => w.endpoint.endsWith('/sslorder/add')).length).toBe(2);
  expect(writes.filter((w) => w.endpoint.endsWith('/sslorder/add'))[1].input.request_id).toBe(firstRequestID);
  await expect(page.getByText('等待证书任务处理', { exact: true })).toBeVisible();
  await expect(page.locator('.search-input input')).toHaveValue('');
  expect(listQueries.at(-1)).toMatchObject({ pageIndex: 1, apply_domain: '' });
  checks.push('Accepted order appears immediately after submitting from page 2 with a different domain filter');
  activeOrder = { ...activeOrder, apply_status: 'applying', progress: '正在请求 CA 并验证域名，DNS 传播可能需要数分钟' };
  await expect(page.getByText(activeOrder.progress, { exact: true })).toBeVisible({ timeout: 7000 });
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    await page.screenshot({ path: path.join(output, `order-progress-${width}.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  activeOrder = { ...activeOrder, apply_status: 'success', progress: '证书签发并保存成功' };
  await expect(page.getByText(activeOrder.progress, { exact: true })).toBeVisible({ timeout: 7000 });
  checks.push('Double click and Enter emit one in-flight request; retry reuses request_id; progress refreshes through success');
  while (await page.locator('.el-notification__closeBtn').count()) {
    const before = await page.locator('.el-notification__closeBtn').count();
    await page.locator('.el-notification__closeBtn').first().click();
    await expect(page.locator('.el-notification__closeBtn')).toHaveCount(before - 1);
  }
  for (const appearance of ['light', 'dark']) {
    await page.evaluate((mode) => { document.documentElement.classList.toggle('dark', mode === 'dark'); document.documentElement.setAttribute('data-theme', mode); }, appearance);
    for (const width of [1440, 1024, 768, 390, 320]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      await page.getByRole('button', { name: '申请证书', exact: true }).scrollIntoViewIfNeeded();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBe(true);
      const first = page.locator('.ssl-order-row[data-order-id="existing-order"]');
      await expect(first.getByText('已关闭', { exact: true })).toBeVisible();
      await expect(first.getByRole('button', { name: '手动续期', exact: true })).toBeVisible();
      expect(await first.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
      await page.screenshot({ path: path.join(output, `order-list-${appearance}-${width}.png`) });
    }
  }
  checks.push('Certificate list puts domain, status, expiry and renewal first, with usable responsive rows and no horizontal page overflow');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await select(page.getByRole('combobox', { name: '筛选申请状态', exact: true }), '已失败');
  await expect.poll(() => listQueries.at(-1)?.apply_status).toBe('fail');
  await expect(page.locator('.ssl-order-row[data-order-id="existing-order"]')).toHaveCount(0);
  await page.getByRole('button', { name: '重置', exact: true }).click();
  await expect(page.locator('.ssl-order-row[data-order-id="existing-order"]')).toBeVisible();
  await page.getByRole('textbox', { name: '筛选申请域名', exact: true }).fill('no-matching-order.invalid');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  await expect(page.getByText('没有符合条件的申请', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '重置', exact: true }).click();
  await expect(page.locator('.ssl-order-row[data-order-id="existing-order"]')).toBeVisible();
  failList = true;
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: '申请列表暂不可用' })).toBeVisible();
  failList = false;
  await page.getByRole('button', { name: '刷新状态', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: '申请列表暂不可用' })).toHaveCount(0);
  const existingRow = page.locator('.ssl-order-row[data-order-id="existing-order"]');
  await existingRow.getByRole('button', { name: '更多操作', exact: true }).click();
  await page.getByRole('menuitem', { name: '删除记录', exact: true }).click();
  await expect(activeDialog()).toBeVisible();
  await activeDialog().getByRole('button', { name: '取消', exact: true }).click();
  await expect(existingRow).toBeVisible();
  checks.push('Status filters reset pagination; empty/error states recover; deleting a record still requires confirmation');

  expect(writes.find((w) => w.endpoint.endsWith('/sslorder/add')).input).toMatchObject({
    host_code: 'pipeline-15000',
    apply_method: 'dns01',
    apply_dns: 'hurricane',
    apply_domain: 'example.com,*.example.com',
    private_group_name: 'default',
    auto_renew: true,
  });
  checks.push('HE multiple DDNS keys save correctly; failed save preserves values; order uses DNS and wildcard');
  await page.getByRole('button', { name: '手动续期', exact: true }).first().click();
  dialog = activeDialog();
  await expect(dialog.getByRole('checkbox', { name: '启用自动续期', exact: true })).not.toBeChecked();
  await expect(
    dialog.locator('.el-select').filter({ has: page.getByRole('combobox', { name: '密钥分组', exact: true }) }),
  ).toContainText('household');
  await dialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => writes.filter((w) => w.endpoint.endsWith('/sslorder/edit')).length).toBe(1);
  expect(writes.find((w) => w.endpoint.endsWith('/sslorder/edit')).input.private_group_name).toBe('household');
  expect(writes.find((w) => w.endpoint.endsWith('/sslorder/edit')).input.auto_renew).toBe(false);
  checks.push('New applications default to automatic renewal; manual renewal preserves the saved opt-out and credential group');
  failCatalogue = true;
  await page.getByRole('button', { name: '申请证书', exact: true }).click();
  dialog = activeDialog();
  await expect(dialog.getByText('DNS 服务商目录加载失败，请重试后再申请。')).toBeVisible();
  failCatalogue = false;
  await dialog.getByRole('button', { name: '重试加载目录', exact: true }).click();
  const selector = dialog.getByRole('combobox', { name: 'DNS 服务商', exact: true });
  await selector.fill('duckdns');
  await page.getByRole('option', { name: 'Duck DNS · duckdns', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '配置 DUCKDNS_TOKEN', exact: true })).toBeEnabled();
  await select(selector, 'Cloudflare · cloudflare');
  await expect(dialog.getByRole('button', { name: '配置 CF_DNS_API_TOKEN', exact: true })).toBeEnabled();
  failStatus = true;
  await select(dialog.getByRole('combobox', { name: '密钥分组', exact: true }), 'household');
  await expect(dialog.getByText(/配置状态加载失败/)).toBeVisible();
  await expect(dialog.getByRole('button', { name: '配置 CF_DNS_API_TOKEN', exact: true })).toBeDisabled();
  failStatus = false;
  await dialog.getByRole('button', { name: '重试读取配置', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '配置 CF_DNS_API_TOKEN', exact: true })).toBeEnabled();
  checks.push(
    'All provider metadata loads, searchable selection works, and catalogue/status errors recover explicitly',
  );
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  emptyHosts = true;
  await page.getByRole('button', { name: '申请证书', exact: true }).click();
  dialog = activeDialog();
  await expect(dialog.getByText(/暂无可关联服务/)).toBeVisible();
  await expect(dialog.getByRole('link', { name: '去添加服务', exact: true })).toHaveAttribute(
    'href',
    '#/waf-pipeline/ports',
  );
  await expect(dialog.getByRole('combobox', { name: '关联服务', exact: true })).toBeDisabled();
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  emptyHosts = false;
  failHosts = true;
  await page.getByRole('button', { name: '申请证书', exact: true }).click();
  dialog = activeDialog();
  await expect(dialog.getByText('服务列表加载失败，请重试。')).toBeVisible();
  failHosts = false;
  await dialog.getByRole('button', { name: '重试加载服务', exact: true }).click();
  await expect(dialog.getByRole('combobox', { name: '关联服务', exact: true })).toBeEnabled();
  checks.push(
    'Native-only service is selectable by mouse; selection fills domains; empty and failed service lists show guidance and retry',
  );
  await select(dialog.getByRole('combobox', { name: '申请平台', exact: true }), 'ZeroSSL');
  await expect(
    dialog.getByTestId('zerossl-account').getByText('ZeroSSL 尚未配置账户认证，暂时不能申请。'),
  ).toBeVisible();
  await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeDisabled();
  await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 15000 });
  for (const appearance of ['light', 'dark']) {
    await page.evaluate((mode) => {
      document.documentElement.classList.toggle('dark', mode === 'dark');
      document.documentElement.setAttribute('data-theme', mode);
    }, appearance);
    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      await dialog.getByTestId('zerossl-account').scrollIntoViewIfNeeded();
      expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
      await page.screenshot({ path: path.join(output, `zerossl-warning-${appearance}-${width}.png`), fullPage: true });
    }
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await dialog.getByRole('button', { name: '配置 ZeroSSL 认证', exact: true }).click();
  let zeroEditor = activeDialog();
  const developerLink = zeroEditor.getByRole('link', {
    name: '前往 ZeroSSL 获取 API Access Key（新标签页）',
    exact: true,
  });
  const developerTabOpened = context.waitForEvent('page');
  await developerLink.click();
  const developerTab = await developerTabOpened;
  await developerTab.waitForURL('https://app.zerossl.com/developer');
  expect(await developerTab.evaluate(() => window.opener === null)).toBe(true);
  await developerTab.close();
  await expect(zeroEditor).toBeVisible();
  checks.push(
    'ZeroSSL developer link opens the requested page in an isolated new tab and keeps the credential editor open',
  );
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    expect(await zeroEditor.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
    await page.screenshot({ path: path.join(output, `zerossl-editor-${width}.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await zeroEditor.getByLabel('ZeroSSL API Access Key', { exact: true }).fill('fixture-zero-access-key');
  await expect(zeroEditor.getByLabel('ZeroSSL API Access Key', { exact: true })).toHaveAttribute('type', 'password');
  failZeroSSLSave = true;
  await zeroEditor.getByRole('button', { name: '保存 ZeroSSL 认证', exact: true }).click();
  await expect(zeroEditor.getByText('认证保存失败，请重试', { exact: true })).toBeVisible();
  await expect(zeroEditor.getByLabel('ZeroSSL API Access Key', { exact: true })).toHaveValue('fixture-zero-access-key');
  await zeroEditor.getByRole('button', { name: '保存 ZeroSSL 认证', exact: true }).click();
  dialog = activeDialog();
  await expect(dialog.getByText('ZeroSSL 账户认证已配置', { exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeEnabled();
  await dialog.getByRole('button', { name: '维护平台', exact: true }).click();
  await activeDialog()
    .locator('.el-table__row')
    .filter({ hasText: 'zerossl' })
    .getByRole('button', { name: '编辑', exact: true })
    .click();
  let caEditor = activeDialog();
  await expect(caEditor.getByTestId('zerossl-account')).toBeVisible();
  await caEditor.getByRole('button', { name: '修改 ZeroSSL 认证', exact: true }).click();
  zeroEditor = activeDialog();
  await expect(
    zeroEditor.getByRole('link', { name: '前往 ZeroSSL 获取 API Access Key（新标签页）', exact: true }),
  ).toHaveAttribute('href', 'https://app.zerossl.com/developer');
  await select(zeroEditor.getByRole('combobox', { name: 'ZeroSSL 配置方式', exact: true }), '直接配置 EAB 凭据');
  await zeroEditor.getByLabel('EAB Key ID', { exact: true }).fill('fixture-eab-kid');
  const countBeforePartial = writes.filter((w) => w.endpoint.endsWith('/zerossl/account')).length;
  await zeroEditor.getByRole('button', { name: '保存 ZeroSSL 认证', exact: true }).click();
  await expect(zeroEditor.getByText('请同时填写 EAB Key ID 和 HMAC Key。')).toBeVisible();
  expect(writes.filter((w) => w.endpoint.endsWith('/zerossl/account')).length).toBe(countBeforePartial);
  await zeroEditor.getByLabel('EAB HMAC Key', { exact: true }).fill('fixture-eab-hmac');
  await zeroEditor.getByRole('button', { name: '保存 ZeroSSL 认证', exact: true }).click();
  caEditor = activeDialog();
  await expect(caEditor.getByText(/优先使用已保存的 EAB 凭据/)).toBeVisible();
  await caEditor.getByRole('button', { name: '关闭', exact: true }).click();
  await page
    .locator('.el-dialog:visible')
    .filter({ has: page.getByRole('heading', { name: 'CA服务器', exact: true }) })
    .getByRole('button', { name: '关闭', exact: true })
    .click();
  dialog = activeDialog();
  await select(dialog.getByRole('combobox', { name: '申请平台', exact: true }), "Let's Encrypt");
  failZeroSSLStatus = true;
  await select(dialog.getByRole('combobox', { name: '申请平台', exact: true }), 'ZeroSSL');
  await expect(dialog.getByText('ZeroSSL 认证状态读取失败，请重试后再申请。')).toBeVisible();
  await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeDisabled();
  failZeroSSLStatus = false;
  await dialog.getByRole('button', { name: '重试认证检查', exact: true }).click();
  await expect(dialog.getByText('ZeroSSL 账户认证已配置', { exact: true })).toBeVisible();
  checks.push(
    'Switching to ZeroSSL immediately checks credentials and blocks unconfigured submission; order and CA editor share secure API-key/EAB setup with retry',
  );
  await dialog.getByRole('button', { name: '关闭', exact: true }).click();
  failLogs = true;
  await page.locator('.ssl-order-row[data-order-id="interrupted-order"]').getByRole('button', { name: '日志', exact: true }).click();
  let logDialog = activeDialog();
  await expect(logDialog.getByText('日志暂不可用，请重试', { exact: true })).toBeVisible();
  failLogs = false;
  await logDialog.getByRole('button', { name: '刷新日志', exact: true }).click();
  await expect(logDialog.getByText(interruptedOrder.result_error, { exact: true })).toBeVisible();
  expect(await logDialog.locator('.log-entries img').count()).toBe(0);
  await expect(logDialog.locator('.log-entries')).toContainText('[REDACTED]');
  while (await page.locator('.el-notification__closeBtn').count()) {
    const before = await page.locator('.el-notification__closeBtn').count();
    await page.locator('.el-notification__closeBtn').first().click();
    await expect(page.locator('.el-notification__closeBtn')).toHaveCount(before - 1);
  }
  for (const appearance of ['light', 'dark']) {
    await page.evaluate((mode) => { document.documentElement.classList.toggle('dark', mode === 'dark'); document.documentElement.setAttribute('data-theme', mode); }, appearance);
    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
      expect(await logDialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 2)).toBe(true);
      const geometry = await logDialog.evaluate((el) => {
        const body = el.querySelector('.el-dialog__body');
        const entries = el.querySelector('.log-entries');
        const overlay = el.closest('.el-overlay-dialog');
        const footer = el.querySelector('.el-dialog__footer').getBoundingClientRect();
        const scrolls = (node) => ['auto', 'scroll'].includes(getComputedStyle(node).overflowY) && node.scrollHeight > node.clientHeight + 2;
        return { bodyScrolls: scrolls(body), entriesScroll: scrolls(entries), overlayScrolls: scrolls(overlay), horizontalOverflow: body.scrollWidth > body.clientWidth + 2, footerVisible: footer.top >= 0 && footer.bottom <= innerHeight };
      });
      expect(geometry).toEqual({ bodyScrolls: true, entriesScroll: false, overlayScrolls: false, horizontalOverflow: false, footerVisible: true });
      await page.screenshot({ path: path.join(output, `order-logs-${appearance}-${width}.png`), fullPage: true });
    }
  }
  checks.push('160 long log entries use one scroll area, wrap without horizontal overflow, and retain visible footer controls at desktop/tablet/phone widths in both themes');
  await page.setViewportSize({ width: 1440, height: 1000 });
  holdRetry = true;
  failRetryOnce = true;
  await logDialog.getByRole('button', { name: '重试申请', exact: true }).dblclick();
  await expect.poll(() => writes.filter((w) => w.endpoint.endsWith('/sslorder/retry')).length).toBe(1);
  await expect(logDialog.getByRole('button', { name: '重试申请', exact: true })).toBeDisabled();
  const retryID = writes.find((w) => w.endpoint.endsWith('/sslorder/retry')).input.request_id;
  expect(retryID).toMatch(/^[0-9a-f-]{36}$/);
  holdRetry = false;
  releaseRetry();
  await expect(logDialog.getByRole('button', { name: '重试申请', exact: true })).toBeEnabled();
  await logDialog.getByRole('button', { name: '重试申请', exact: true }).click();
  await expect(logDialog.getByText('订单 ID：retried-order', { exact: true })).toBeVisible();
  expect(writes.filter((w) => w.endpoint.endsWith('/sslorder/retry'))[1].input.request_id).toBe(retryID);
  expect(retryKeys.size).toBe(1);
  await expect(logDialog.locator('.log-entries')).toContainText('开始重试');
  retryOrder = { ...retryOrder, apply_status: 'applying', progress: '正在写入 DNS 验证记录' };
  await expect(logDialog.getByText(retryOrder.progress, { exact: true })).toBeVisible({ timeout: 7000 });
  await expect(logDialog.getByRole('button', { name: '重试申请', exact: true })).toHaveCount(0);
  retryOrder = { ...retryOrder, heartbeat_stale: true };
  await expect(logDialog.getByText('心跳中断（等待确认）', { exact: true })).toBeVisible({ timeout: 7000 });
  await logDialog.getByRole('button', { name: '关闭', exact: true }).click();
  await expect(page.locator('.ssl-order-row[data-order-id="interrupted-order"]')).toBeVisible();
  checks.push('Order logs show failure history and escaped text, recover from read errors, and retry once with stable idempotency while preserving the old order');
  expect(errors).toEqual([]);
  fs.writeFileSync(
    path.join(output, 'results.json'),
    JSON.stringify({ checks, errors, providerCount: catalogue.providers.length }, null, 2),
  );
  console.log(JSON.stringify({ checks, errors, providerCount: catalogue.providers.length }, null, 2));
} catch (error) {
  console.error('Browser errors:', JSON.stringify(errors));
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  fs.writeFileSync(path.join(output, 'failure.txt'), String(error) + '\n' + (await page.locator('body').innerText()));
  throw error;
} finally {
  await context.close();
  await browser.close();
}
