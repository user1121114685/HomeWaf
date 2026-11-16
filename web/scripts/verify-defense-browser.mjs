// Real Vue interactions with isolated encrypted API fixtures. This never sends
// management writes or firewall operations to a running HomeWaf instance.
import { chromium, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const origin = process.env.DEFENSE_TEST_ORIGIN || 'http://127.0.0.1:3081';
const out = path.join(root, 'artifacts', 'port-defense');
fs.mkdirSync(out, { recursive: true });
const clone = (value) => JSON.parse(JSON.stringify(value));
const templates = JSON.parse(
  execFileSync('go', ['run', './scripts/pipeline-fixtures'], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, CGO_ENABLED: '0' },
  }),
);
const configs = new Map(
  [5000, 5001].map((port) => [
    port,
    {
      schema_version: 1,
      port,
      name: `处置测试${port}`,
      protocol: 'http',
      listen_enabled: true,
      upstream: 'http://127.0.0.1:18080',
      host_names: [],
      policy: clone(templates[0].policy),
      revision: 1,
      template: { id: templates[0].id, version: 1 },
    },
  ]),
);
configs.get(5001).policy.honeypot.enabled = true;
for (const c of configs.values()) {
  c.policy.ip_group.unlisted_action = 'continue';
  c.policy.gate.enabled = false;
}
const otherPort = JSON.stringify(configs.get(5001));
let shared = { configured: false, revision: 0, config: clone(templates[0].policy.honeypot) };
let failSave = false,
  failSharedSave = false,
  failSharedRead = false;
const report = { checks: [], screenshots: [], errors: [], writes: [] };
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-defense-test');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  const req = route.request(),
    url = new URL(req.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, data: AesEncrypt('{}'), msg } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint))
    return fail('isolated test');
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
  if (endpoint === 'wafhost/ipgroup/options') return ok([]);
  if (endpoint === 'iplocation/status')
    return ok({ ipv4_file_size: 1, ipv6_file_size: 1, ipv4_builtin: true, ipv6_builtin: true });
  if (!endpoint.startsWith('pipeline/')) return ok({ list: [], total: 0 });
  const action = endpoint.slice(9),
    port = Number(url.searchParams.get('port') || input.port || input.config?.port);
  if (req.method() === 'POST')
    report.writes.push({ action, port, input: action === 'ports/access' ? input : undefined });
  if (action === 'overview')
    return ok({
      ports: [...configs.values()].map((config) => ({
        config: clone(config),
        stats: {},
        listener: { running: true },
        locked: !!config.defense_state,
      })),
      issues: {},
    });
  if (action === 'templates') return ok(templates);
  if (['migration/legacy', 'regions', 'gate/users', 'gate/sessions', 'state', 'logs', 'honeypot/logs'].includes(action))
    return ok([]);
  if (action === 'defense') {
    const c = configs.get(port);
    return ok({
      revision: c.revision,
      access_enabled: c.policy.access.enabled,
      mode: c.policy.defense?.mode || '',
      enabled: !!c.policy.defense?.enabled,
      legacy: !c.policy.defense,
      honeypot_ready: shared.configured,
      failures: 4,
      success_prefixes: 12,
      history_ready: true,
      state: c.defense_state,
    });
  }
  if (action === 'defense/capabilities')
    return ok({
      reset_supported: true,
      drop_supported: false,
      drop_reason: '隔离测试环境不提供内核丢包权限',
      runtime_error: '',
      direct_only: true,
    });
  if (action === 'ports/save') {
    if (failSave) {
      failSave = false;
      return fail('模拟服务保存失败，草稿已保留');
    }
    const old = configs.get(port);
    if (old.revision !== input.config.revision) return fail('配置版本冲突');
    if (!old.policy.defense && input.config.policy.defense && !input.confirm_defense_migration)
      return fail('未确认迁移');
    const value = clone(input.config);
    value.revision++;
    if (value.policy.defense) {
      value.policy.honeypot.enabled = false;
      value.policy.hit_policy.lockdown.enabled = false;
      value.policy.hit_policy.ban_scope = 'port';
    }
    configs.set(port, value);
    return ok(value);
  }
  if (action === 'ports/access') {
    const c = configs.get(port);
    if (c.revision !== input.revision) return fail('状态版本冲突');
    if (input.action === 'release' && c.defense_state?.id !== input.state_id) return fail('处置已变化');
    if (input.action === 'close') c.policy.access.enabled = false;
    if (input.action === 'open' || (input.action === 'release' && c.defense_state?.mode === 'close'))
      c.policy.access.enabled = true;
    delete c.defense_state;
    c.revision++;
    return ok(c);
  }
  if (action === 'honeypot') {
    if (failSharedRead) {
      failSharedRead = false;
      return fail('模拟蜜罐读取失败');
    }
    return ok({
      shared,
      legacy: [...configs.values()]
        .filter((c) => !c.policy.defense && c.policy.honeypot.enabled)
        .map((c) => ({ port: c.port, name: c.name, config: c.policy.honeypot })),
      services: [...configs.values()]
        .filter((c) => c.policy.defense?.mode === 'honeypot' && c.policy.defense.enabled)
        .map((c) => ({ port: c.port, name: c.name })),
    });
  }
  if (action === 'honeypot/save') {
    if (failSharedSave) {
      failSharedSave = false;
      return fail('模拟统一蜜罐保存失败');
    }
    if (input.revision !== shared.revision) return fail('蜜罐配置版本冲突');
    shared = { ...clone(input), revision: shared.revision + 1, configured: true };
    return ok(shared);
  }
  return fail('Unexpected isolated endpoint: ' + action);
});
const page = await context.newPage();
page.on('pageerror', (e) => report.errors.push(e.message));
const card = () => page.getByTestId('service-cards').locator('article').filter({ hasText: '处置测试5000' });
const editor = () => page.getByTestId('port-editor');
const panel = () => page.getByTestId('defense-settings');
const dialog = () => page.locator('.el-message-box:visible');
async function confirm() {
  await dialog()
    .getByRole('button', { name: /确定|确认/ })
    .click();
  await expect(page.locator('.el-message-box:visible')).toHaveCount(0);
}
async function chooseMode(name) {
  const input = panel().getByRole('combobox', { name: '异常访问处置模式' });
  await input.focus();
  await input.press('ArrowDown');
  await page.getByRole('option', { name, exact: true }).click();
}
async function openDefense() {
  await card().getByRole('button', { name: '异常访问处置', exact: true }).click();
  await expect(panel()).toBeVisible();
}
async function closeEditor() {
  await editor().getByRole('button', { name: '关闭', exact: true }).click();
  await confirm();
  await expect(editor()).toBeHidden();
}
async function savePort() {
  await editor().getByRole('button', { name: '保存端口配置', exact: true }).click();
}

try {
  await page.goto(origin + '/#/waf-pipeline/ports');
  await openDefense();
  await panel().getByRole('button', { name: '配置新处置模式' }).click();
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await panel().getByText('我已核对差异，确认用新模式替代旧端口处置', { exact: true }).click();
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeEnabled();
  await expect(panel().getByRole('spinbutton', { name: '统计窗口（分钟）' })).toHaveValue('30');
  await expect(panel().getByRole('spinbutton', { name: '触发次数' })).toHaveValue('10');
  failSave = true;
  await savePort();
  await expect(editor().getByText('模拟服务保存失败，草稿已保留', { exact: true })).toBeVisible();
  await expect(panel().getByRole('spinbutton', { name: '统计窗口（分钟）' })).toHaveValue('30');
  await savePort();
  await expect(editor()).toBeHidden();
  await openDefense();
  await expect(panel().getByRole('button', { name: '配置新处置模式' })).toHaveCount(0);
  if (configs.get(5000).policy.defense.mode !== 'quiet' || JSON.stringify(configs.get(5001)) !== otherPort)
    throw new Error('wrong persistence scope');
  report.checks.push('migration confirmation, defaults, save failure retains draft, save/reopen, port isolation');
  await chooseMode('收紧 IP 访问');
  await expect(panel().getByText(/没有有效历史时会关闭整个服务访问/)).toBeVisible();
  await closeEditor();
  if (configs.get(5000).policy.defense.mode !== 'quiet') throw new Error('cancel saved a draft');
  for (const action of ['release', 'close']) {
    const c = configs.get(5000);
    c.policy.access.enabled = false;
    c.defense_state = {
      id: 'incident-' + action,
      mode: 'close',
      reason: 'failure_threshold',
      started_at: Math.floor(Date.now() / 1000),
      expires_at: Math.floor(Date.now() / 1000) + 10800,
      access_was_enabled: true,
    };
    c.revision++;
    await page.getByRole('button', { name: '刷新状态', exact: true }).first().click();
    await openDefense();
    const name = action === 'release' ? '提前恢复' : '保持关闭';
    await panel().getByRole('button', { name, exact: true }).click();
    if (action === 'release') {
      await dialog().getByRole('button', { name: '取消', exact: true }).click();
      if (!configs.get(5000).defense_state) throw new Error('cancel committed recovery');
      await panel().getByRole('button', { name, exact: true }).click();
    }
    await confirm();
    await expect(editor()).toBeHidden();
    if (configs.get(5000).defense_state || configs.get(5000).policy.access.enabled !== (action === 'release'))
      throw new Error('incorrect access action');
  }
  report.checks.push('active state, recovery cancellation, versioned early recovery, keep closed');
  failSharedRead = true;
  await page.goto(origin + '/#/waf-pipeline/honeypot');
  const honey = () => page.getByTestId('shared-honeypot');
  await expect(honey().getByText('模拟蜜罐读取失败', { exact: true })).toBeVisible();
  await honey().getByRole('button', { name: '重试', exact: true }).click();
  await expect(honey().getByRole('button', { name: '保存统一蜜罐设置' })).toBeVisible();
  await honey().getByRole('button', { name: '添加蜜罐提示' }).click();
  await honey().getByRole('textbox', { name: '蜜罐提示1标题' }).fill('统一仿真入口');
  await honey().getByRole('textbox', { name: '蜜罐提示1内容' }).fill('独立记录安全拒绝，不连接真实业务。');
  failSharedSave = true;
  await honey().getByRole('button', { name: '保存统一蜜罐设置' }).click();
  await confirm();
  await expect(honey().getByText('模拟统一蜜罐保存失败', { exact: true })).toBeVisible();
  await expect(honey().getByRole('textbox', { name: '蜜罐提示1标题' })).toHaveValue('统一仿真入口');
  await honey().getByRole('button', { name: '保存统一蜜罐设置' }).click();
  await confirm();
  await expect(honey().getByText(/配置已读取/)).toBeVisible();
  report.checks.push('standalone honeypot read failure/retry, shared write failure preserves draft, shared save');
  await expect(page.locator('.el-message:visible')).toHaveCount(0, { timeout: 6000 });
  for (const [width, height] of [
    [1440, 1000],
    [820, 1180],
    [390, 844],
  ]) {
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await page.evaluate((theme) => {
        document.documentElement.setAttribute('theme-mode', theme);
        document.documentElement.setAttribute('data-theme', theme);
        document.documentElement.classList.toggle('dark', theme === 'dark');
      }, theme);
      await page.evaluate(() => {
        document.activeElement?.blur?.();
        window.scrollTo(0, 0);
      });
      await page.waitForTimeout(250);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2);
      if (overflow) throw new Error(`horizontal overflow ${width}/${theme}`);
      const file = `honeypot-${width}-${theme}.png`;
      await page.screenshot({ path: path.join(out, file), fullPage: true });
      report.screenshots.push(file);
    }
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(origin + '/#/waf-pipeline/ports');
  await openDefense();
  await chooseMode('蜜罐');
  await savePort();
  await expect(editor()).toBeHidden();
  await openDefense();
  await expect(panel().getByRole('link', { name: '在新标签页配置统一蜜罐' })).toBeVisible();
  const popupPromise = context.waitForEvent('page');
  await panel().getByRole('link', { name: '在新标签页配置统一蜜罐' }).click();
  const popup = await popupPromise;
  await expect(popup.getByTestId('shared-honeypot')).toBeVisible();
  await popup.close();
  await expect(panel()).toBeVisible();
  if (configs.get(5000).policy.defense.mode !== 'honeypot' || shared.config.lures[0].title !== '统一仿真入口')
    throw new Error('honeypot binding failed');
  await chooseMode('自动关闭访问');
  await expect(page.locator('.el-message:visible')).toHaveCount(0, { timeout: 6000 });
  for (const [width, height] of [
    [1440, 1000],
    [820, 1180],
    [390, 844],
  ]) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark']) {
      await page.evaluate((theme) => {
        document.documentElement.setAttribute('theme-mode', theme);
        document.documentElement.setAttribute('data-theme', theme);
        document.documentElement.classList.toggle('dark', theme === 'dark');
      }, theme);
      const count = panel().getByRole('spinbutton', { name: '触发次数' });
      await count.fill('12');
      await count.press('Tab');
      await expect(count).toHaveValue('12');
      await count.fill('10');
      await count.press('Tab');
      await panel().getByRole('spinbutton', { name: '持续时间（小时）' }).scrollIntoViewIfNeeded();
      await page.waitForTimeout(250);
      const file = `defense-${width}-${theme}.png`;
      await page.screenshot({ path: path.join(out, file) });
      report.screenshots.push(file);
    }
  }
  report.checks.push(
    'shared honeypot binding, prerequisite new-tab return, desktop/tablet/phone form interaction, two themes, reduced motion',
  );
  if (report.errors.length) throw new Error(report.errors.join('\n'));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  await page.screenshot({ path: path.join(out, 'failure.png'), fullPage: true });
  throw error;
} finally {
  fs.writeFileSync(path.join(out, 'browser-report.json'), JSON.stringify(report, null, 2));
  await browser.close();
}
