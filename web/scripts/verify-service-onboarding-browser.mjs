// Real first-use Vue flow; all management API requests are intercepted by isolated fixtures.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const source = JSON.parse(
  execFileSync('go', ['run', './scripts/pipeline-fixtures'], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, CGO_ENABLED: '0' },
  }),
);
const origin = process.env.ONBOARDING_TEST_ORIGIN || 'http://127.0.0.1:3081';
const artifact = path.join(root, 'artifacts', 'service-onboarding');
fs.mkdirSync(artifact, { recursive: true });
const clone = (v) => JSON.parse(JSON.stringify(v));
const configs = new Map(),
  users = new Map(),
  certificateBindings = new Map(),
  tokens = new Set();
let templates = clone(source),
  saveCalls = [],
  savePayloads = [],
  failSavePort = 0,
  failConnection = false,
  failCertificates = false,
  emptyCertificates = false,
  certificateCreated = false,
  failCertificateSave = true;
const report = { checks: [], errors: [], writes: [] };
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1600, height: 1100 }, colorScheme: 'light' });
await context.addInitScript((appOrigin) => {
  if (location.origin !== appOrigin) return;
  localStorage.setItem('access_token', 'isolated-pipeline-ui-test');
  localStorage.setItem('lang', 'zh_CN');
}, origin);
await context.routeWebSocket('**/api/**', (ws) => ws.close());
function resultFor(config) {
  const blocked = !config.policy.access.enabled || config.policy.ip_group.unlisted_action === 'deny';
  const gate = config.policy.gate.enabled || config.policy.gate.require_token;
  return {
    req_id: 'ui-preview',
    verdict: {
      action: blocked ? 'deny' : gate ? 'challenge' : 'allow',
      stage: blocked ? 1 : gate ? 3 : 5,
      reason: blocked ? 'access_policy' : gate ? 'auth_required' : 'passed',
    },
    trace: [
      { stage: 1, check: 'access_switch', elapsed_us: 2, verdict: { action: 'continue', reason: 'access_enabled' } },
      { stage: 2, check: 'ip_group', elapsed_us: 2, verdict: { action: 'continue', reason: 'ip:unlisted' } },
      {
        stage: 3,
        check: 'auth_gate',
        elapsed_us: 3,
        verdict: { action: gate ? 'challenge' : 'continue', reason: gate ? 'auth_required' : 'gate_disabled' },
      },
    ],
  };
}
await context.route('**/api/**', async (route) => {
  const req = route.request(),
    url = new URL(req.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  if (endpoint.includes('handshake') || endpoint === 'public/seckey' || endpoint === 'sysinfo/checkversion')
    return route.fulfill({ json: { code: 1, msg: 'Isolated UI fixture' } });
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
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, msg, data: AesEncrypt(JSON.stringify({})) } });
  const p = Number(url.searchParams.get('port') || input.port || input.config?.port);
  if (endpoint === 'iplocation/status')
    return ok({ ipv4_file_size: 11122714, ipv6_file_size: 37283761, ipv4_builtin: true, ipv6_builtin: true });
  if (endpoint === 'wafhost/ipgroup/options') return ok([]);
  if (endpoint === 'wafhost/sslorder/targets') return ok([]);
  if (endpoint === 'certificates/list')
    return ok({
      list: certificateCreated
        ? [
            {
              id: 'new-cert',
              domains: 'nas.example.test, new.example.test',
              validity: 'valid',
              update_method: 'manual',
              bindings: [],
            },
          ]
        : [],
      total: certificateCreated ? 1 : 0,
    });
  if (endpoint === 'sslconfig/add') {
    if (failCertificateSave)
      return route.fulfill({
        json: {
          code: 1,
          msg: '证书与私钥不匹配',
          data: AesEncrypt(JSON.stringify({ field_errors: { key_content: '证书与私钥不匹配' } })),
        },
      });
    certificateCreated = true;
    report.writes.push({ action: 'sslconfig/add' });
    return ok({ id: 'new-cert', export_failed: false });
  }
  if (!endpoint.startsWith('pipeline/')) return ok({ list: [], total: 0 });
  const action = endpoint.slice(9);
  if (req.method() === 'POST') report.writes.push({ action, port: p || undefined });
  if (action === 'overview')
    return ok({
      ports: [...configs.values()].map((config) => ({
        config,
        stats: { today_hits: 3, today_denied: 1 },
        ban_count: 0,
        honeypot_count: 0,
        locked: false,
        listener: { running: config.listen_enabled },
      })),
      issues: {},
      punishment: { queued: 0, capacity: 4096 },
      log_queue: { queued: 0, capacity: 8192 },
    });
  if (action === 'ports') return ok([...configs.values()]);
  if (action === 'migration/legacy') return ok([]);
  if (action === 'templates') return ok(templates);
  if (action === 'regions')
    return ok([
      { country: '中国', province: '重庆' },
      { country: '中国', province: '浙江省' },
      { country: '中国', province: '广东省' },
      { country: 'United States', province: 'California' },
    ]);
  if (action === 'defense/capabilities')
    return ok({ reset_supported: true, drop_supported: false, drop_reason: '当前测试平台不支持静默丢包' });
  if (action === 'honeypot') return ok({ shared: { configured: false } });
  if (action === 'defense') {
    const config = configs.get(p);
    return ok({
      revision: config.revision,
      enabled: !!config.policy.defense?.enabled,
      mode: config.policy.defense?.mode,
      legacy: !config.policy.defense,
      failures: 0,
      success_prefixes: 0,
      history_ready: true,
    });
  }
  if (action === 'ports/check') return configs.has(input.port) ? fail('端口已有独立配置') : ok('检查通过');
  if (action === 'tls/options') {
    if (failCertificates) return fail('读取证书列表失败，请重试');
    const valid = !input.host_names.some((host) => host !== 'nas.example.test');
    const certificate = {
      id: 'nas-cert',
      domains: ['nas.example.test'],
      expires_at: '2030-01-01T00:00:00Z',
      valid,
      reason: valid ? '' : '证书未覆盖域名或 IP：' + input.host_names.join(', '),
    };
    return ok({
      certificates: emptyCertificates
        ? []
        : [
            certificate,
            { ...certificate, id: 'expired-cert', valid: false, reason: '证书已过期，请更新证书' },
            ...(certificateCreated
              ? [{ ...certificate, id: 'new-cert', domains: ['nas.example.test', 'new.example.test'] }]
              : []),
          ],
      current: certificateBindings.has(p) ? certificate : undefined,
    });
  }
  if (action === 'tls/check') {
    if (input.host_names.some((host) => host !== 'nas.example.test'))
      return fail('证书未覆盖域名或 IP：' + input.host_names.join(', '));
    if (input.tls_certificate || input.tls_private_key) return fail('证书或私钥不合法，或两者不匹配');
    if (!['nas-cert', 'new-cert'].includes(input.tls_certificate_id) && !certificateBindings.has(p))
      return fail('HTTPS 需要证书和私钥，请选择已有证书或填写 PEM');
    return ok({ id: 'nas-cert', domains: ['nas.example.test'], valid: true, expires_at: '2030-01-01T00:00:00Z' });
  }
  if (action === 'upstream/test') return failConnection ? fail('无法连接现有服务，请检查地址和端口') : ok('连接成功');
  if (action === 'explain') return ok(resultFor(input.config));
  if (action === 'ports/save') {
    if (p === failSavePort) return fail('模拟并发修改，原配置保留');
    const old = configs.get(p),
      config = clone(input.config);
    if (old && old.revision !== config.revision) return fail('配置已更新，请刷新');
    if (old?.listen_enabled && old.protocol !== config.protocol && !input.confirm_listener_restart)
      return fail('更改监听协议将平滑重启该服务，请确认后保存');
    if (
      config.policy.gate.enabled &&
      !(users.get(p) || []).some((u) => u.enabled) &&
      (!input.initial_username || input.initial_password?.length < 10)
    )
      return fail('登录门启用前必须建立账号');
    if (config.policy.gate.require_token && config.listen_enabled && !tokens.has(p))
      return fail('开始监听前必须配置服务令牌');
    if (!old && (!config.policy.defense?.enabled || !config.policy.defense?.mode))
      return fail('接入时必须配置异常访问处置');
    if (!old && config.listen_enabled) return fail('新建端口不得直接监听');
    saveCalls.push(clone(config));
    savePayloads.push(clone(input));
    config.revision = (old?.revision || 0) + 1;
    configs.set(p, config);
    if (input.tls_certificate_id) certificateBindings.set(p, input.tls_certificate_id);
    if (input.initial_username)
      users.set(p, [{ username: input.initial_username, enabled: true, expires_at: 0, remark: '' }]);
    return ok(config);
  }
  if (action === 'ports/copy') {
    const config = clone(configs.get(input.source));
    config.port = input.target;
    config.revision = 0;
    config.listen_enabled = false;
    config.template = {};
    config.policy.gate.enabled = false;
    if (!input.include_ips) {
      config.policy.ip_group.entries = [];
      config.policy.ip_group.allow_groups = [];
      config.policy.ip_group.deny_groups = [];
    }
    return ok(config);
  }
  if (action === 'gate/users') return ok(users.get(p) || []);
  if (action === 'gate/sessions' || action === 'state') return ok([]);
  if (action === 'logs') return ok([]);
  if (action === 'stats')
    return ok({
      port: { today_hits: 3, today_denied: 1 },
      punishment: { queued: 0, dropped_low: 0 },
      log_queue: { queued: 0 },
    });
  if (action === 'templates/preview') {
    const tpl = templates.find((t) => t.id === input.template_id),
      config = clone(input.config),
      keys = input.sections?.length ? input.sections : Object.keys(tpl.policy),
      diff = [];
    for (const section of keys) {
      if (JSON.stringify(config.policy[section]) !== JSON.stringify(tpl.policy[section]))
        diff.push({ section, current: clone(config.policy[section]), template: clone(tpl.policy[section]) });
      const ips = config.policy.ip_group.entries,
        proxies = config.policy.network.trusted_proxies;
      config.policy[section] = clone(tpl.policy[section]);
      config.policy.ip_group.entries = ips;
      config.policy.network.trusted_proxies = proxies;
    }
    config.template = { id: tpl.id, version: tpl.version };
    return ok({ config, diff });
  }
  return fail('Unexpected isolated test endpoint ' + action);
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
async function acceptConfirm() {
  const box = page.locator('.el-message-box');
  await expect(box).toBeVisible();
  await box.locator('.el-message-box__btns button').last().click();
}
try {
  await page.goto(origin + '/#/waf-pipeline/ports');
  await expect(page.getByRole('heading', { name: '接入你的第一个服务' })).toBeVisible();
  await page.getByRole('button', { name: '接入服务', exact: true }).click();
  const guide = page.getByTestId('guided-setup');
  await expect(guide).toBeVisible();
  await page.screenshot({ animations: 'disabled', path: path.join(artifact, 'guided-scenes.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(guide.getByRole('button', { name: '继续填写地址' })).toBeInViewport();
  await page.screenshot({ animations: 'disabled', path: path.join(artifact, 'guided-scenes-mobile.png') });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  expect(
    await guide
      .locator('.scene-option')
      .first()
      .evaluate((el) => parseFloat(getComputedStyle(el).transitionDuration)),
  ).toBeLessThanOrEqual(0.001);
  await expect(guide.getByRole('button', { name: '继续填写地址' })).toBeVisible();
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await guide.getByRole('button', { name: '继续填写地址' }).click();
  await guide.getByRole('checkbox', { name: '使用独立访问账号' }).locator('xpath=ancestor::label[1]').click();
  await guide.getByLabel('访问账号', { exact: true }).fill('unused-private-user');
  await guide.getByLabel('访问密码', { exact: true }).fill('unused-private-password');
  await guide.getByRole('button', { name: '上一步' }).click();
  await guide.getByRole('radio', { name: /公开网站/ }).check();
  await guide.getByRole('button', { name: '继续填写地址' }).click();
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toBeVisible();
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toHaveValue('');
  await expect(guide.getByLabel('允许的主机名（域名或 IP）', { exact: true })).toBeVisible();
  await guide.getByLabel('现有服务地址', { exact: true }).fill('https://user:secret@nas.test');
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByText('地址中不能包含账号密码，请只填写服务地址。')).toBeVisible();
  await guide.getByLabel('现有服务地址', { exact: true }).fill('127.0.0.1:18080');
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByText('填写 HomeWaf 监听端口，供访客访问。')).toBeVisible();
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toBeFocused();
  await guide.getByLabel('HomeWaf 监听端口', { exact: true }).fill('65536');
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByText('监听端口必须是 1–65535 之间的整数。')).toBeVisible();
  await guide.getByLabel('HomeWaf 监听端口', { exact: true }).fill('18480');
  await guide.getByLabel('允许的主机名（域名或 IP）', { exact: true }).fill('https://nas.example.test');
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByText('请只填写域名或 IP，不含协议、端口和路径：https://nas.example.test')).toBeVisible();
  await guide.getByLabel('允许的主机名（域名或 IP）', { exact: true }).fill('nas.example.test,192.0.2.10,2001:db8::1');
  for (const [width, height] of [
    [1440, 1000],
    [820, 1180],
    [390, 844],
    [320, 780],
  ]) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark']) {
      await page.evaluate((theme) => {
        document.documentElement.setAttribute('theme-mode', theme);
        document.documentElement.setAttribute('data-theme', theme);
        document.documentElement.classList.toggle('dark', theme === 'dark');
      }, theme);
      await guide.getByLabel('HomeWaf 监听端口', { exact: true }).scrollIntoViewIfNeeded();
      await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toBeInViewport();
      await guide.getByLabel('允许的主机名（域名或 IP）', { exact: true }).scrollIntoViewIfNeeded();
      await expect(guide.getByLabel('允许的主机名（域名或 IP）', { exact: true })).toBeInViewport();
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
        .toBeLessThanOrEqual(2);
      await page.screenshot({
        animations: 'disabled',
        path: path.join(artifact, `guided-connection-${width}-${theme}.png`),
      });
    }
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByRole('heading', { name: '设置异常访问处置' })).toBeVisible();
  await expect(guide.getByRole('radio', { checked: true })).toHaveCount(0);
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.getByText('请选择一种异常访问处置方式。', { exact: true })).toBeVisible();
  await expect(guide.getByText('请选择一种异常访问处置方式。', { exact: true })).toBeFocused();
  expect(saveCalls.length).toBe(0);
  await guide.getByRole('radio', { name: '蜜罐', exact: true }).check();
  await expect(guide.getByText('先准备统一蜜罐内容', { exact: true })).toBeVisible();
  await expect(guide.getByRole('spinbutton')).toHaveCount(0);
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.getByRole('heading', { name: '设置异常访问处置' })).toBeVisible();
  await guide.getByRole('radio', { name: '自动关闭访问', exact: true }).check();
  await expect(guide.locator('.rule-summary')).toContainText('最近 30 分钟内，累计 10 次');
  await guide.getByRole('spinbutton', { name: '触发处置的异常次数' }).fill('12');
  await guide.getByRole('spinbutton', { name: '触发处置的异常次数' }).press('Tab');
  await expect(guide.locator('.rule-summary')).toContainText('累计 12 次');
  for (const [width, height] of [
    [1440, 1000],
    [820, 1180],
    [390, 844],
    [320, 780],
  ]) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark']) {
      await page.evaluate((theme) => {
        document.documentElement.setAttribute('theme-mode', theme);
        document.documentElement.setAttribute('data-theme', theme);
        document.documentElement.classList.toggle('dark', theme === 'dark');
      }, theme);
      await guide.getByRole('heading', { name: '设置异常访问处置' }).scrollIntoViewIfNeeded();
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
        .toBeLessThanOrEqual(2);
      await page.screenshot({
        animations: 'disabled',
        path: path.join(artifact, `guided-defense-${width}-${theme}.png`),
      });
      await guide.getByRole('spinbutton', { name: '自动恢复等待时间（小时）' }).scrollIntoViewIfNeeded();
      await page.screenshot({
        animations: 'disabled',
        path: path.join(artifact, `guided-defense-timing-${width}-${theme}.png`),
      });
    }
  }
  await page.evaluate(() => {
    document.documentElement.setAttribute('theme-mode', 'light');
    document.documentElement.setAttribute('data-theme', 'light');
    document.documentElement.classList.remove('dark');
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await guide.getByRole('button', { name: '上一步' }).click();
  await expect(guide.getByLabel('现有服务地址', { exact: true })).toHaveValue('http://127.0.0.1:18080');
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByRole('radio', { name: '自动关闭访问', exact: true })).toBeChecked();
  await expect(guide.getByRole('spinbutton', { name: '触发处置的异常次数' })).toHaveValue('12');
  failConnection = true;
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.getByText('无法连接现有服务，请检查地址和端口')).toBeVisible();
  await expect(guide.getByRole('spinbutton', { name: '触发处置的异常次数' })).toHaveValue('12');
  failConnection = false;
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
    [320, 780],
  ]) {
    await page.setViewportSize({ width, height });
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
      .toBeLessThanOrEqual(2);
    await page.screenshot({ animations: 'disabled', path: path.join(artifact, `guided-address-${width}.png`) });
  }
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.getByRole('heading', { name: '确认服务配置' })).toBeVisible();
  await expect(guide.locator('.setup-review')).toContainText('累计 12 次');
  const beforeNewSave = saveCalls.length;
  failSavePort = 18480;
  await guide.getByRole('button', { name: '保存服务配置' }).click();
  await expect(guide.getByText('模拟并发修改，原配置保留')).toBeVisible();
  expect(saveCalls.length).toBe(beforeNewSave);
  failSavePort = 0;
  await guide.getByRole('button', { name: '保存服务配置' }).evaluate((button) => {
    button.click();
    button.click();
  });
  await expect(guide).toHaveCount(0);
  expect(saveCalls.length).toBe(beforeNewSave + 1);
  expect(configs.get(18480).listen_enabled).toBe(false);
  expect(configs.get(18480).policy.gate.enabled).toBe(false);
  expect(configs.get(18480).name).toBe('公开网站 · 127.0.0.1');
  expect(configs.get(18480).policy.defense).toMatchObject({ enabled: true, mode: 'close', count: 12 });
  expect(configs.get(18480).host_names).toEqual(['nas.example.test', '192.0.2.10', '2001:db8::1']);
  expect(savePayloads.at(-1)).not.toHaveProperty('confirm_defense_migration');
  expect(savePayloads.at(-1).initial_username).toBe('');
  expect(savePayloads.at(-1).initial_password).toBe('');
  await expect(page.getByRole('button', { name: '继续启用服务' })).toBeVisible();
  report.checks.push(
    'first-use service list, explicit mandatory defense, readable rule, mode dependencies and back navigation, inline errors, retained input and exactly-once save retry',
  );

  await page.setViewportSize({ width: 1440, height: 1000 });
  const serviceCard = page
    .getByTestId('service-cards')
    .locator('article')
    .filter({ hasText: '公开网站 · 127.0.0.1' })
    .filter({ hasText: '18480' });
  await serviceCard.getByRole('button', { name: '异常访问处置', exact: true }).click();
  const editor = page.getByTestId('port-editor');
  await expect(editor.getByRole('radio', { name: '自动关闭访问', exact: true })).toBeChecked();
  await expect(editor.getByRole('spinbutton', { name: '触发处置的异常次数' })).toHaveValue('12');
  await editor.getByRole('spinbutton', { name: '触发处置的异常次数' }).fill('15');
  await editor.getByRole('spinbutton', { name: '触发处置的异常次数' }).press('Tab');
  await editor.getByRole('button', { name: '保存端口配置', exact: true }).click();
  await expect(editor).toBeHidden();
  expect(configs.get(18480).policy.defense.count).toBe(15);
  report.checks.push('new service reopens with persisted defense and supports subsequent edits');

  await serviceCard.getByRole('button', { name: '更多服务操作' }).click();
  await page.getByRole('menuitem', { name: '复制服务配置' }).click();
  await expect(page.getByLabel('复制目标端口', { exact: true })).toHaveValue('');
  await page.getByLabel('复制目标端口', { exact: true }).fill('19480');
  await page.getByRole('button', { name: '生成复制草稿' }).click();
  await guide.getByRole('button', { name: '继续填写地址' }).click();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByRole('radio', { checked: true })).toHaveCount(0);
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.getByText('请选择一种异常访问处置方式。', { exact: true })).toBeVisible();
  await guide.getByRole('radio', { name: '无提示拒绝', exact: true }).check();
  await expect(guide.getByRole('spinbutton', { name: '触发处置的异常次数' })).toHaveValue('15');
  await guide.getByRole('button', { name: '上一步' }).click();
  await guide.getByRole('button', { name: '上一步' }).click();
  await guide.getByRole('button', { name: '继续填写地址' }).click();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByRole('radio', { name: '无提示拒绝', exact: true })).toBeChecked();
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await guide.getByRole('button', { name: '保存服务配置' }).click();
  await expect(guide).toBeHidden();
  const copied = savePayloads.at(-1).config;
  expect(copied.port).toBe(19480);
  expect(copied.policy.defense).toMatchObject({ enabled: true, mode: 'quiet', count: 15 });
  expect(configs.get(18480).policy.defense.mode).toBe('close');
  report.checks.push('copied service requires a mode, retains parameters through back navigation, saves independently');

  await page.goto(origin + '/#/waf-pipeline/ports?action=create');
  await guide.getByRole('button', { name: '继续填写地址' }).click();
  await guide.getByRole('checkbox', { name: '使用独立访问账号' }).locator('xpath=ancestor::label[1]').click();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByText('填写供访客使用的访问账号。')).toBeVisible();
  await guide.getByLabel('现有服务地址', { exact: true }).fill('127.0.0.1:18080');
  await guide.getByLabel('访问账号', { exact: true }).fill('visitor');
  await guide.getByLabel('访问密码', { exact: true }).fill('isolated-visitor-password');
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toHaveValue('');
  await guide.getByLabel('HomeWaf 监听端口', { exact: true }).fill('18481');
  await guide.getByRole('combobox', { name: '监听协议', exact: true }).focus();
  await guide.getByRole('combobox', { name: '监听协议', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: 'HTTPS（需要证书）', exact: true }).click();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await expect(guide.getByText('请选择证书；没有可用证书时，请先前往证书管理。')).toBeVisible();
  await guide.getByRole('combobox', { name: '监听协议', exact: true }).focus();
  await guide.getByRole('combobox', { name: '监听协议', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: 'HTTP', exact: true }).click();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await guide.getByRole('radio', { name: '收紧 IP 访问', exact: true }).check();
  await expect(guide.getByText(/刚接入时通常还没有成功访问记录/)).toBeVisible();
  await guide.getByRole('button', { name: '上一步' }).click();
  await expect(guide.getByLabel('访问账号', { exact: true })).toHaveValue('visitor');
  await expect(guide.getByLabel('访问密码', { exact: true })).toHaveValue('isolated-visitor-password');
  const beforeCancel = saveCalls.length;
  await page.getByRole('dialog', { name: '接入服务', exact: true }).locator('.el-dialog__headerbtn').click();
  await acceptConfirm();
  await expect(guide).toBeHidden();
  expect(saveCalls.length).toBe(beforeCancel);
  const storage = await page.evaluate(() =>
    JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }),
  );
  expect(storage).not.toContain('isolated-visitor-password');
  report.checks.push(
    'account and HTTPS prerequisites explain missing input, credentials survive safe back navigation and never enter browser storage, cancellation does not save',
  );

  configs.get(18480).listen_enabled = true;
  await page.reload();
  await serviceCard.getByRole('button', { name: '配置服务', exact: true }).click();
  await editor.getByRole('tab', { name: /监听入口/ }).click();
  await expect(editor.getByLabel('允许的主机名（域名或 IP）', { exact: true })).toHaveValue(
    'nas.example.test\n192.0.2.10\n2001:db8::1',
  );
  await editor.getByLabel('允许的主机名（域名或 IP）', { exact: true }).fill('nas.example.test');
  failCertificates = true;
  await editor.getByRole('combobox', { name: '监听协议', exact: true }).focus();
  await editor.getByRole('combobox', { name: '监听协议', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: 'HTTPS', exact: true }).click();
  await expect(editor.getByText('读取证书列表失败，请重试')).toBeVisible();
  failCertificates = false;
  emptyCertificates = true;
  await editor.getByRole('button', { name: '刷新证书列表', exact: true }).click();
  await expect(editor.getByText(/还没有已保存的证书/)).toBeVisible();
  emptyCertificates = false;
  await editor.getByRole('button', { name: '刷新证书列表', exact: true }).click();
  await expect(editor.locator('.service-tls .el-select')).toContainText('nas.example.test');
  await expect(editor.getByText(/已匹配：nas.example.test/)).toBeVisible();
  for (const [width, height] of [
    [1440, 1000],
    [820, 1180],
    [390, 844],
    [320, 780],
  ]) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark']) {
      await page.evaluate((theme) => {
        document.documentElement.setAttribute('theme-mode', theme);
        document.documentElement.setAttribute('data-theme', theme);
        document.documentElement.classList.toggle('dark', theme === 'dark');
      }, theme);
      await editor.getByRole('heading', { name: 'HTTPS 证书', exact: true }).scrollIntoViewIfNeeded();
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
        .toBeLessThanOrEqual(2);
      await page.screenshot({ animations: 'disabled', path: path.join(artifact, `service-tls-${width}-${theme}.png`) });
      await editor.getByRole('combobox', { name: '域名证书', exact: true }).scrollIntoViewIfNeeded();
      await expect(editor.getByRole('combobox', { name: '域名证书', exact: true })).toBeInViewport();
      await page.screenshot({
        animations: 'disabled',
        path: path.join(artifact, `service-tls-controls-${width}-${theme}.png`),
      });
    }
  }
  await editor.getByRole('button', { name: '保存端口配置', exact: true }).click();
  const restart = page.getByRole('dialog', { name: '确认平滑重启服务', exact: true });
  await expect(restart).toContainText('18480');
  await expect(restart).toContainText('HTTP 切换为 HTTPS');
  const beforeRestart = saveCalls.length;
  await restart.getByRole('button', { name: '返回修改', exact: true }).click();
  expect(saveCalls.length).toBe(beforeRestart);
  expect(configs.get(18480).protocol).toBe('http');
  await editor.getByRole('button', { name: '保存端口配置', exact: true }).click();
  await restart.getByRole('button', { name: '保存并平滑重启', exact: true }).click();
  await expect(editor).toBeHidden();
  expect(configs.get(18480).protocol).toBe('https');
  expect(certificateBindings.get(18480)).toBe('nas-cert');
  expect(savePayloads.at(-1).tls_certificate_id).toBe('nas-cert');
  expect(savePayloads.at(-1)).not.toHaveProperty('tls_private_key');
  expect(savePayloads.at(-1).confirm_listener_restart).toBe(true);
  expect(configs.get(18480).listen_enabled).toBe(true);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await serviceCard.getByRole('button', { name: '配置服务', exact: true }).click();
  await editor.getByRole('tab', { name: /监听入口/ }).click();
  await expect(editor.getByText(/当前证书检查通过/)).toBeVisible();
  await editor.getByLabel('允许的主机名（域名或 IP）', { exact: true }).fill('wrong.example.test');
  await expect(editor.getByText('证书未覆盖域名或 IP：wrong.example.test')).toBeVisible();
  const beforeMismatch = saveCalls.length;
  await editor.getByRole('button', { name: '保存端口配置', exact: true }).click();
  await expect(editor).toBeVisible();
  expect(saveCalls.length).toBe(beforeMismatch);
  expect(configs.get(18480).host_names).toEqual(['nas.example.test']);
  await editor.getByLabel('允许的主机名（域名或 IP）', { exact: true }).fill('nas.example.test');
  await editor.getByRole('button', { name: '保存端口配置', exact: true }).click();
  await expect(editor).toBeHidden();
  report.checks.push(
    'HTTP to HTTPS selects a matching stored certificate, distinguishes read failure and empty catalog, persists only its identity, and blocks hostname mismatch on reopen',
  );

  await page.goto(origin + '/#/waf-pipeline/ports?action=create');
  await guide.getByRole('button', { name: '继续填写地址' }).click();
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toHaveValue('');
  await guide.getByLabel('现有服务地址', { exact: true }).fill('127.0.0.1:18080');
  await guide.getByLabel('HomeWaf 监听端口', { exact: true }).fill('18480');
  await guide.getByLabel('允许的主机名（域名或 IP）', { exact: true }).fill('nas.example.test');
  await guide.getByRole('combobox', { name: '监听协议', exact: true }).focus();
  await guide.getByRole('combobox', { name: '监听协议', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: 'HTTPS（需要证书）', exact: true }).click();
  await expect(guide.getByText(/已匹配：nas.example.test/)).toBeVisible();
  await expect(guide.getByLabel('服务名称', { exact: true })).toBeVisible();
  const managementLink = guide.getByRole('link', { name: '前往证书管理', exact: true });
  await expect(managementLink).toHaveAttribute('href', /#\/waf-host\/wafsslconfiglist$/);
  await expect(managementLink).toHaveAttribute('target', '_blank');
  await expect(managementLink).toHaveAttribute('rel', /noopener/);
  const managementPagePromise = context.waitForEvent('page');
  await managementLink.click();
  const managementPage = await managementPagePromise;
  managementPage.on('pageerror', (error) => report.errors.push(error.message));
  await managementPage.waitForURL(/#\/waf-host\/wafsslconfiglist$/);
  expect(await managementPage.evaluate(() => window.opener === null)).toBe(true);
  await expect(page.getByTestId('certificate-editor')).toHaveCount(0);
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toHaveValue('18480');
  await managementPage.getByRole('button', { name: '导入证书', exact: true }).click();
  const certificateEditor = managementPage.getByTestId('certificate-editor');
  await expect(certificateEditor).toBeVisible();
  await certificateEditor.getByRole('button', { name: '取消', exact: true }).click();
  await expect(certificateEditor).toBeHidden();
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toHaveValue('18480');
  await managementPage.getByRole('button', { name: '导入证书', exact: true }).click();
  const certificatePEM = '-----BEGIN CERTIFICATE-----\nZmFrZS1jZXJ0\n-----END CERTIFICATE-----';
  const privatePEM = '-----BEGIN PRIVATE KEY-----\nprivate-key-fixture\n-----END PRIVATE KEY-----';
  await certificateEditor.locator('#certificate-cert_content').fill(certificatePEM);
  await certificateEditor.locator('#certificate-key_content').fill(privatePEM);
  for (const [width, height] of [
    [1440, 1000],
    [390, 844],
  ]) {
    await managementPage.setViewportSize({ width, height });
    await certificateEditor.locator('#certificate-key_content').scrollIntoViewIfNeeded();
    await expect(certificateEditor.locator('#certificate-key_content')).toBeInViewport();
    await expect
      .poll(() => managementPage.evaluate(() => document.documentElement.scrollWidth - innerWidth))
      .toBeLessThanOrEqual(2);
    await managementPage.screenshot({
      animations: 'disabled',
      path: path.join(artifact, `managed-certificate-${width}.png`),
    });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await certificateEditor.locator('button[type="submit"]').click();
  await expect(certificateEditor.getByText('证书与私钥不匹配').first()).toBeVisible();
  await expect(certificateEditor.locator('#certificate-key_content')).toHaveValue(privatePEM);
  failCertificateSave = false;
  await certificateEditor.locator('button[type="submit"]').click();
  await expect(certificateEditor).toBeHidden();
  await expect(managementPage.getByText('nas.example.test, new.example.test', { exact: true }).first()).toBeVisible();
  await managementPage.close();
  await page.bringToFront();
  await expect(guide.getByLabel('HomeWaf 监听端口', { exact: true })).toHaveValue('18480');
  await expect(guide.getByLabel('允许的主机名（域名或 IP）', { exact: true })).toHaveValue('nas.example.test');
  await guide.getByRole('button', { name: '刷新证书列表', exact: true }).click();
  await expect(guide.locator('.service-tls')).toHaveAttribute('aria-busy', 'false');
  await guide.getByRole('combobox', { name: '域名证书', exact: true }).focus();
  await guide.getByRole('combobox', { name: '域名证书', exact: true }).press('ArrowDown');
  await page.getByRole('option', { name: /new.example.test/ }).click();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await guide.getByRole('radio', { name: '无提示拒绝', exact: true }).check();
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.getByText('端口已有独立配置')).toBeVisible();
  await guide.getByRole('button', { name: '上一步' }).click();
  await expect(guide.getByLabel('允许的主机名（域名或 IP）', { exact: true })).toHaveValue('nas.example.test');
  await guide.getByLabel('HomeWaf 监听端口', { exact: true }).fill('28443');
  await expect(guide.getByText(/已匹配：nas.example.test/)).toBeVisible();
  await guide.getByRole('button', { name: '继续设置异常处置' }).click();
  await guide.getByRole('button', { name: '检查配置并继续' }).click();
  await expect(guide.locator('.setup-review')).toContainText('28443');
  await expect(guide.locator('.setup-review')).toContainText('nas.example.test');
  await guide.getByRole('button', { name: '保存服务配置' }).click();
  await expect(guide).toBeHidden();
  expect(configs.get(28443).protocol).toBe('https');
  expect(configs.get(28443).listen_enabled).toBe(false);
  expect(certificateBindings.get(28443)).toBe('new-cert');
  expect(certificateCreated).toBe(true);
  expect(savePayloads.at(-1)).not.toHaveProperty('tls_certificate');
  expect(savePayloads.at(-1)).not.toHaveProperty('tls_private_key');
  expect(await page.evaluate(() => JSON.stringify({ ...localStorage, ...sessionStorage }))).not.toContain(
    'private-key-fixture',
  );
  report.checks.push(
    'certificate management opens in an isolated new tab; creation, cancellation and retry stay on that page; returning preserves the service draft and refresh selects the new certificate',
  );
  expect(report.errors).toEqual([]);
  fs.writeFileSync(path.join(artifact, 'onboarding-report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  await page.screenshot({ animations: 'disabled', path: path.join(artifact, 'onboarding-failure.png') });
  console.error(JSON.stringify(report, null, 2));
  throw error;
} finally {
  await browser.close();
}
