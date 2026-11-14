// Real service policy controls with isolated in-memory configuration snapshots.
import { chromium, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';
import { servicePolicyEntries } from '../src/pages/waf/pipeline/policyEntries.ts';
const root = fileURLToPath(new URL('../..', import.meta.url));
const output = path.join(root, 'artifacts', 'service-policy-review');
fs.mkdirSync(output, { recursive: true });
const origin = process.env.CONSOLE_TEST_ORIGIN || 'http://127.0.0.1:3118';
const templates = JSON.parse(
  execFileSync('go', ['run', './scripts/pipeline-fixtures'], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, CGO_ENABLED: '0' },
  }),
);
const configs = [15000, 15001].map((port, i) => ({
  schema_version: 1,
  revision: 1,
  port,
  host_code: 'pipeline-' + port,
  name: i ? '家庭相册' : '群晖',
  protocol: 'http',
  listen_enabled: false,
  host_names: [],
  upstream: 'http://127.0.0.1:5000',
  policy: structuredClone(templates.find((t) => t.id === 'family-nas').policy),
  template: { id: 'family-nas', version: 1 },
}));
const before = structuredClone(configs),
  writes = [],
  errors = [],
  checks = [];
let failNextSave = false;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-service-policy');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const request = route.request(),
    url = new URL(request.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg } });
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
  if (endpoint === 'pipeline/overview')
    return ok({ ports: configs.map((config) => ({ config, listener: { running: false }, stats: {} })) });
  if (endpoint === 'pipeline/templates') return ok(templates);
  if (endpoint === 'pipeline/ports/save') {
    writes.push(input);
    if (failNextSave) {
      failNextSave = false;
      return fail('配置版本冲突，修改已保留');
    }
    const index = configs.findIndex((c) => c.port === input.config.port);
    if (index < 0 || configs[index].revision !== input.config.revision) return fail('revision conflict');
    configs[index] = { ...input.config, revision: input.config.revision + 1 };
    return ok(configs[index]);
  }
  if (
    ['pipeline/gate/users', 'pipeline/gate/sessions', 'pipeline/regions', 'wafhost/ipgroup/options'].includes(endpoint)
  )
    return ok([]);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (e) => errors.push(e.message));
try {
  await page.goto(origin + '/?appearance=light#/waf-host/wafanticclist');
  await expect(page.getByTestId('service-policy-page')).toBeVisible();
  await expect(page.getByRole('heading', { name: '群晖 · CC 防御' })).toBeVisible();
  const threshold = page.getByRole('spinbutton', { name: 'CC 阈值', exact: true });
  const initial = before[0].policy.ruleset.cc.threshold;
  await threshold.fill(String(initial + 10));
  await threshold.press('Tab');
  await page.getByRole('button', { name: '保存 CC 防护', exact: true }).click();
  await expect.poll(() => configs[0].policy.ruleset.cc.threshold).toBe(initial + 10);
  const untouched = structuredClone(configs[0]);
  untouched.revision = before[0].revision;
  untouched.policy.ruleset.cc = before[0].policy.ruleset.cc;
  expect(untouched).toEqual(before[0]);
  expect(configs[1]).toEqual(before[1]);
  await page.reload();
  await expect(threshold).toHaveValue(String(initial + 10));
  checks.push('CC saves to the selected listener configuration, preserves other settings/services and survives reload');
  await threshold.fill(String(initial + 11));
  await threshold.press('Tab');
  failNextSave = true;
  await page.getByRole('button', { name: '保存 CC 防护', exact: true }).click();
  await expect(page.getByText('配置版本冲突，修改已保留', { exact: true })).toBeVisible();
  await expect(threshold).toHaveValue(String(initial + 11));
  expect(configs[0].policy.ruleset.cc.threshold).toBe(initial + 10);
  await page.getByRole('button', { name: '撤销修改', exact: true }).click();
  checks.push('revision/save failures preserve the draft and never report success');
  const selector = page.getByRole('combobox', { name: '服务', exact: true });
  await selector.press('ArrowDown');
  await page.getByRole('option', { name: '家庭相册 · :15001', exact: true }).click();
  await expect(threshold).toHaveValue(String(before[1].policy.ruleset.cc.threshold));
  checks.push('service switching reads each service independently');
  for (const name of Object.keys(servicePolicyEntries)) {
    await page.evaluate((name) => window.vm.$router.push({ name }), name);
    await expect(page.getByTestId('service-policy-page')).toBeVisible();
    if (name !== 'WafAntiCCList') {
      await page.getByRole('button', { name: '配置' + servicePolicyEntries[name].title, exact: true }).click();
      await expect(page.getByTestId('port-editor')).toBeVisible();
      await page.getByTestId('port-editor').getByRole('button', { name: '关闭', exact: true }).click();
      await expect(page.getByTestId('port-editor')).toHaveCount(0);
    }
  }
  checks.push('all 14 protection entry points open the authoritative service configuration');
  await page.evaluate(() => window.vm.$router.push({ name: 'WafAntiCCList' }));
  for (const width of [1440, 768, 390, 320]) {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2)).toBe(true);
    await page.screenshot({ path: path.join(output, 'cc-' + width + '.png'), fullPage: true });
  }
  expect(errors).toEqual([]);
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  throw error;
} finally {
  fs.writeFileSync(
    path.join(output, 'report.json'),
    JSON.stringify({ checks, errors, writes: writes.length }, null, 2),
  );
  console.log(JSON.stringify({ checks, errors, writes: writes.length }));
  await browser.close();
}
