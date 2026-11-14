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
  credentials = [];
let failCatalogue = false,
  failStatus = false,
  failSave = false;
const renewal = {
  id: 'existing-order',
  host_code: 'nas',
  apply_platform: 'letsencrypt',
  apply_method: 'dns01',
  apply_dns: 'hurricane',
  private_group_name: 'household',
  apply_email: 'owner@example.com',
  apply_domain: 'example.com,*.example.com',
  apply_status: 'success',
  skip_dns_verify: 0,
};
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-dns-fixture');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
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
  if (endpoint === 'wafhost/host/allhost') return ok([{ value: 'nas', label: '家庭 NAS :8443' }]);
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
          ca_server_name: 'letsencrypt',
          ca_server_address: 'https://acme-v02.api.letsencrypt.org/directory',
          remarks: "Let's Encrypt",
        },
      ],
      total: 1,
    });
  if (endpoint === 'wafhost/sslorder/list') return ok({ list: [renewal], total: 1 });
  if (endpoint === 'wafhost/sslorder/add' || endpoint === 'wafhost/sslorder/edit') {
    writes.push({ endpoint, input });
    return ok({});
  }
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.setDefaultTimeout(10000);
page.on('pageerror', (e) => errors.push(e.message));
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
  await page.getByRole('button', { name: '新建', exact: true }).click();
  let dialog = activeDialog();
  await expect(dialog.getByTestId('dns-config')).toBeVisible();
  await expect(dialog.getByText(/无需开放或转发入站 80 \/ 443/)).toBeVisible();
  await expect(dialog).toContainText('DNS 验证（家庭网络推荐，无需入站端口）');
  await select(dialog.getByLabel(/主机/), '家庭 NAS :8443');
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
  await dialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => writes.filter((w) => w.endpoint.endsWith('/sslorder/add')).length).toBe(1);
  expect(writes.find((w) => w.endpoint.endsWith('/sslorder/add')).input).toMatchObject({
    apply_method: 'dns01',
    apply_dns: 'hurricane',
    apply_domain: 'example.com,*.example.com',
    private_group_name: 'default',
  });
  checks.push('HE multiple DDNS keys save correctly; failed save preserves values; order uses DNS and wildcard');
  await page.getByRole('button', { name: '续期', exact: true }).first().click();
  dialog = activeDialog();
  await expect(
    dialog.locator('.el-select').filter({ has: page.getByRole('combobox', { name: '密钥分组', exact: true }) }),
  ).toContainText('household');
  await dialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => writes.filter((w) => w.endpoint.endsWith('/sslorder/edit')).length).toBe(1);
  expect(writes.find((w) => w.endpoint.endsWith('/sslorder/edit')).input.private_group_name).toBe('household');
  checks.push('Renewal preserves the stored group instead of selecting the first group');
  failCatalogue = true;
  await page.getByRole('button', { name: '新建', exact: true }).click();
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
  expect(errors).toEqual([]);
  fs.writeFileSync(
    path.join(output, 'results.json'),
    JSON.stringify({ checks, errors, providerCount: catalogue.providers.length }, null, 2),
  );
  console.log(JSON.stringify({ checks, errors, providerCount: catalogue.providers.length }, null, 2));
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  fs.writeFileSync(path.join(output, 'failure.txt'), String(error) + '\n' + (await page.locator('body').innerText()));
  throw error;
} finally {
  await context.close();
  await browser.close();
}
