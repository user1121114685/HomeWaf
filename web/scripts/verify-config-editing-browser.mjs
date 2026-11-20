// Exercise repeated edits against the real Vue app with isolated, in-memory
// management responses. No request from this browser reaches a live API.
import { chromium, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { handleTransportHandshake, routeSession } from './transport-fixture.mjs';

const root = fileURLToPath(new URL('../..', import.meta.url));
const origin = process.env.CONFIG_EDIT_TEST_ORIGIN || 'http://127.0.0.1:3001';
const output = path.join(root, 'artifacts', 'config-editability');
fs.mkdirSync(output, { recursive: true });
const clone = (value) => JSON.parse(JSON.stringify(value));
let templates = JSON.parse(
  execFileSync('go', ['run', './scripts/pipeline-fixtures'], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, CGO_ENABLED: '0' },
  }),
);
templates.push({ ...clone(templates[0]), id: 'user-edit', name: '可修改模板', builtin: false, version: 1 });
const configs = new Map(),
  users = new Map();
for (const port of [5000, 5001]) {
  const template = templates.find((template) => template.id === (port === 5000 ? 'family-nas' : 'public-site'));
  configs.set(port, {
    schema_version: 1,
    port,
    name: `测试服务${port}`,
    protocol: 'http',
    listen_enabled: port === 5000,
    upstream: 'http://127.0.0.1:18080',
    host_names: [],
    policy: clone(template.policy),
    revision: 1,
    template: { id: template.id, version: 1 },
  });
  users.set(port, [{ username: 'owner', enabled: true, expires_at: 0, remark: '初始备注' }]);
}
const otherPort = JSON.stringify(configs.get(5001));
let sharedHoneypot = {
  revision: 0,
  configured: false,
  config: { max_concurrent: 32, max_delay_ms: 250, archive_max_bytes: 16384, lures: [], trap_paths: [] },
};
let failNextSave = false;
const report = { checks: [], errors: [], writes: [], confirmations: [], origin };
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1050 }, colorScheme: 'light' });
await context.addInitScript(() => {
  if (window !== window.top) return; // Keep the authentication fixture out of sandboxed previews.
  localStorage.setItem('access_token', 'isolated-config-edit-test');
  localStorage.setItem('lang', 'zh_CN');
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
await context.route('**/api/**', async (route) => {
  if (await handleTransportHandshake(route)) return;
  const request = route.request(),
    url = new URL(request.url()),
    endpoint = url.pathname.split('/api/v1/')[1] || '';
  const ok = (data) => route.fulfill({ json: { code: 0, data: routeSession(route).encrypt(JSON.stringify(data)), msg: '' } });
  const fail = (msg) => route.fulfill({ json: { code: 1, msg, data: routeSession(route).encrypt('{}') } });
  if (endpoint === 'sysinfo/checkversion')
    return fail('Isolated config-edit fixture');
  let input = {};
  if (request.postData()) {
    input = JSON.parse(routeSession(route).decrypt(request.postData()));
  }
  const port = Number(url.searchParams.get('port') || input.port || input.config?.port);
  if (endpoint === 'iplocation/status')
    return ok({ ipv4_file_size: 11122714, ipv6_file_size: 37283761, ipv4_builtin: true, ipv6_builtin: true });
  if (endpoint === 'wafhost/ipgroup/options') return ok([]);
  if (!endpoint.startsWith('pipeline/')) return ok({ list: [], total: 0 });
  const action = endpoint.slice(9);
  if (request.method() === 'POST') report.writes.push({ action, port: port || undefined });
  if (action === 'overview')
    return ok({
      ports: [...configs.values()].map((config) => ({
        config: clone(config),
        stats: {},
        listener: { running: config.listen_enabled },
      })),
      issues: {},
    });
  if (action === 'templates') return ok(clone(templates));
  if (action === 'migration/legacy') return ok([]);
  if (action === 'regions') return ok([{ country: '中国', province: '浙江省' }]);
  if (action === 'gate/users') return ok(clone(users.get(port) || []));
  if (['gate/sessions', 'state', 'logs'].includes(action)) return ok([]);
  if (action === 'stats') return ok({ port: {}, punishment: {}, log_queue: {} });
  if (action === 'gate/users/save') {
    const list = clone(users.get(port)),
      existing = list.find((user) => user.username === input.username);
    if (!existing) return fail('This fixture edits existing accounts only');
    const { password, totp_secret, ...values } = input;
    Object.assign(existing, values);
    users.set(port, list);
    return ok({});
  }
  if (action === 'defense')
    return ok({
      revision: configs.get(port).revision,
      legacy: true,
      enabled: false,
      history_ready: true,
      failures: 0,
      success_prefixes: 0,
    });
  if (action === 'defense/capabilities')
    return ok({ reset_supported: true, drop_supported: false, drop_reason: 'Isolated fixture' });
  if (action === 'honeypot/logs') return ok([]);
  if (action === 'honeypot')
    return ok({
      shared: sharedHoneypot,
      legacy: [],
      services: [],
      sources: [...configs.values()].map((c) => ({ port: c.port, name: c.name })),
    });
  if (action === 'honeypot/save') {
    if (failNextSave) {
      failNextSave = false;
      return fail('模拟保存失败，草稿应保留');
    }
    if (input.revision !== sharedHoneypot.revision) return fail('蜜罐配置版本冲突');
    sharedHoneypot = { ...clone(input), revision: sharedHoneypot.revision + 1, configured: true };
    return ok(sharedHoneypot);
  }
  if (action === 'ports/save') {
    if (failNextSave) {
      failNextSave = false;
      return fail('模拟保存失败，草稿应保留');
    }
    const config = clone(input.config),
      old = configs.get(config.port);
    if (!old || old.revision !== config.revision) return fail('配置版本冲突');
    config.revision++;
    configs.set(config.port, config);
    return ok(clone(config));
  }
  if (action === 'templates/import-preview') return ok(input);
  if (action === 'templates/save') {
    const old = templates.find((template) => template.id === input.id);
    const updated = { ...clone(input), version: (old?.version || 0) + 1 };
    templates = [...templates.filter((template) => template.id !== input.id), updated];
    return ok(updated);
  }
  return fail('Unexpected isolated endpoint: ' + action);
});
const page = await context.newPage();
page.on('pageerror', (error) => report.errors.push(error.message));
const cards = () => page.getByTestId('service-cards');
const card = () =>
  cards()
    .locator('article')
    .filter({ hasText: /测试服务5000|持续修改的服务/ });
const editor = () => page.getByTestId('port-editor');
async function openCommon() {
  if (!page.url().includes('/waf-pipeline/ports')) await page.goto(origin + '/#/waf-pipeline/ports');
  await card().getByRole('button', { name: '配置服务', exact: true }).click();
  await expect(editor()).toBeVisible();
}
async function savePort() {
  await editor().getByRole('button', { name: '保存端口配置', exact: true }).click();
  await expect(editor()).toHaveCount(0);
}
async function cancelPort() {
  await editor().getByRole('button', { name: '关闭', exact: true }).click();
  const box = page.locator('.el-message-box');
  await expect(box).toBeVisible();
  await box.locator('.el-message-box__btns button').last().click();
  await expect(editor()).toHaveCount(0);
}
try {
  await page.goto(origin + '/#/waf-pipeline/ports');
  await expect(card()).toBeVisible({ timeout: 30000 });
  await page.goto(origin + '/#/waf-pipeline/honeypot');
  const sharedPage = page.getByTestId('shared-honeypot');
  const honey = sharedPage.getByTestId('honeypot-settings');
  const saveHoney = async () => {
    await sharedPage.getByRole('button', { name: '保存统一蜜罐设置', exact: true }).click();
    const box = page.locator('.el-message-box:visible');
    await box.locator('.el-message-box__btns button').last().click();
    await expect(page.locator('.el-message-box:visible')).toHaveCount(0);
  };
  const setSwitch = async (name) => {
    const control = honey.getByRole('switch', { name, exact: true });
    if ((await control.getAttribute('aria-checked')) !== 'true') await control.locator('xpath=..').click();
  };
  await honey.getByRole('button', { name: '添加蜜罐页面', exact: true }).click();
  await honey.getByLabel('蜜罐提示1标题', { exact: true }).fill('持续可修改的提示');
  await honey.getByLabel('蜜罐提示1内容', { exact: true }).fill('第一版内容');
  await honey.locator('.weight-details summary').click();
  await honey.getByRole('spinbutton', { name: '蜜罐提示1权重', exact: true }).fill('70');
  await setSwitch('保存脱敏后的提交内容');
  await saveHoney();
  await expect(sharedPage.getByText('配置已保存', { exact: true })).toBeVisible();
  const lureID = sharedHoneypot.config.lures[0].id;
  expect(configs.get(5000).listen_enabled).toBe(true);
  expect(sharedHoneypot.config.lures[0].weight).toBe(70);
  report.checks.push('independent shared honeypot settings save without recreating or stopping services');
  await page.reload();
  await expect(honey.getByLabel('蜜罐提示1标题', { exact: true })).toHaveValue('持续可修改的提示');
  await honey.getByLabel('蜜罐提示1内容', { exact: true }).fill('第二版内容');
  await honey.getByRole('button', { name: '添加蜜罐页面', exact: true }).click();
  await honey.getByLabel('蜜罐提示2标题', { exact: true }).fill('可删除的提示');
  await honey.getByLabel('蜜罐提示1标题', { exact: true }).fill('');
  await expect(sharedPage.getByRole('button', { name: '保存统一蜜罐设置' })).toBeDisabled();
  await honey.getByLabel('蜜罐提示1标题', { exact: true }).fill('持续可修改的提示');
  failNextSave = true;
  await saveHoney();
  await expect(sharedPage.getByText('模拟保存失败，草稿应保留', { exact: true })).toBeVisible();
  await expect(honey.getByLabel('蜜罐提示1内容', { exact: true })).toHaveValue('第二版内容');
  expect(sharedHoneypot.config.lures[0].message).toBe('第一版内容');
  await saveHoney();
  await expect(sharedPage.getByText('配置已保存', { exact: true })).toBeVisible();
  expect(sharedHoneypot.config.lures[0].id).toBe(lureID);
  await honey.getByRole('button', { name: '移除蜜罐提示2', exact: true }).click();
  await saveHoney();
  await expect(sharedPage.getByText('配置已保存', { exact: true })).toBeVisible();
  expect(sharedHoneypot.config.lures).toHaveLength(1);
  report.checks.push('shared honeypot reopens, edits/adds/removes prompts, validates and retains failed-save drafts');
  await honey.getByLabel('蜜罐提示1内容', { exact: true }).fill('取消的内容');
  const writesBeforeCancel = report.writes.length;
  for (const [width, height] of [
    [1920, 911],
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
      await page.getByRole('button', { name: '返回我的服务', exact: true }).click();
      const box = page.locator('.el-message-box:visible');
      await expect(box).toContainText('未保存的修改');
      const bounds = await box.boundingBox();
      expect(bounds.width).toBeLessThanOrEqual(480);
      expect(bounds.x).toBeGreaterThanOrEqual(16);
      expect(bounds.x + bounds.width).toBeLessThanOrEqual(width - 16);
      expect(await box.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);
      const screenshot = `unsaved-confirmation-${width}-${theme}.png`;
      await box.screenshot({ path: path.join(output, screenshot), animations: 'disabled' });
      report.confirmations.push({ viewport: width, theme, width: bounds.width, height: bounds.height, screenshot });
      if (theme === 'dark') await page.keyboard.press('Escape');
      else await box.getByRole('button', { name: '取消', exact: true }).click();
      await expect(box).toHaveCount(0);
      await expect(honey.getByLabel('蜜罐提示1内容', { exact: true })).toHaveValue('取消的内容');
      expect(report.writes).toHaveLength(writesBeforeCancel);
    }
  }
  report.checks.push(
    'compact unsaved confirmation at desktop/tablet/phone in two themes; cancel and Escape preserve draft without writes',
  );
  await page.setViewportSize({ width: 1440, height: 1050 });
  await page.evaluate(() => {
    document.documentElement.setAttribute('theme-mode', 'light');
    document.documentElement.setAttribute('data-theme', 'light');
    document.documentElement.classList.remove('dark');
  });
  await page.getByRole('button', { name: '返回我的服务', exact: true }).click();
  await page.locator('.el-message-box:visible .el-message-box__btns button').last().click();
  await expect(card()).toBeVisible();
  expect(sharedHoneypot.config.lures[0].message).toBe('第二版内容');
  report.checks.push('discarding a shared honeypot draft preserves saved content');

  await openCommon();
  await editor().getByLabel('服务名称', { exact: true }).fill('持续修改的服务');
  await editor().getByLabel('现有服务地址', { exact: true }).fill('http://127.0.0.1:18081');
  await savePort();
  await openCommon();
  await expect(editor().getByLabel('现有服务地址', { exact: true })).toHaveValue('http://127.0.0.1:18081');
  await editor().getByRole('tab', { name: '代理与证书' }).click();
  await editor().locator('details').filter({ hasText: '连接限额、超时与响应高级参数' }).locator('summary').click();
  const routes = editor().getByLabel('同端口业务路由JSON', { exact: true });
  await routes.fill('[unfinished');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await editor().getByRole('tab', { name: '连接与访问' }).click();
  await editor().getByRole('button', { name: '定位未完成的配置', exact: true }).click();
  await expect(routes).toHaveValue('[unfinished');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await routes.fill('[]');
  await editor().getByRole('spinbutton', { name: '代理最大跳数', exact: true }).fill('12');
  await savePort();
  expect(configs.get(5000).policy.network.max_proxy_hops).toBe(12);
  expect(sharedHoneypot.config.lures[0].message).toBe('第二版内容');
  report.checks.push(
    'connection settings reopen and invalid proxy JSON survives task switches, can be located, corrected and saved',
  );

  await openCommon();
  await editor().getByRole('tab', { name: '认证与账号' }).click();
  await editor().getByLabel('登录页标题', { exact: true }).fill('已修改的登录门');
  await editor().getByRole('button', { name: '修改 / 重设口令', exact: true }).click();
  const account = page.getByRole('dialog', { name: '端口账号', exact: true });
  await account.locator('label').filter({ hasText: '备注' }).locator('input').fill('账号可持续修改');
  await account.getByRole('button', { name: '立即保存账号', exact: true }).click();
  await expect(account).toBeHidden();
  await savePort();
  expect(users.get(5000)[0].remark).toBe('账号可持续修改');
  expect(configs.get(5000).policy.gate.title).toBe('已修改的登录门');
  report.checks.push('existing login account and gate settings update independently and remain saved');

  await openCommon();
  await editor().getByRole('tab', { name: '检测规则' }).click();
  const ssrfMode = page.getByRole('combobox', { name: 'SSRF URL 检测模式' });
  const csrfMode = page.getByRole('combobox', { name: 'CSRF 同源校验模式', exact: true });
  const csrfRow = editor().getByRole('row').filter({ has: csrfMode });
  const dsm = editor()
    .getByRole('row')
    .filter({ has: page.getByRole('combobox', { name: '群晖 DSM 登录保护模式', exact: true }) });
  await expect(dsm).toContainText('拦截');
  await dsm.getByRole('button', { name: '阈值与范围' }).click();
  await expect(page.getByRole('spinbutton', { name: '暂停新登录时长（秒）' })).toHaveValue('10800');
  await page.getByRole('button', { name: '返回配置', exact: true }).click();
  await expect(editor().locator('.el-select').filter({ has: ssrfMode })).toContainText('允许（关闭检测）');
  await expect(csrfRow.locator('.rule-controls .el-select')).toHaveCount(1);
  await expect(csrfRow.getByRole('combobox', { name: '缺少 Origin / Referer' })).toHaveCount(0);
  await csrfRow.getByRole('button', { name: '配置范围' }).click();
  const csrfScope = page.getByRole('dialog').last();
  await expect(csrfScope.getByRole('combobox', { name: '缺少 Origin / Referer' })).toBeVisible();
  await expect(csrfScope.locator('.el-select')).toContainText('允许');
  const missingOrigin = csrfScope.getByRole('combobox', { name: '缺少 Origin / Referer' });
  await missingOrigin.press('ArrowDown');
  await page.getByRole('option', { name: '仅观察', exact: true }).click();
  await page.getByRole('button', { name: '返回配置', exact: true }).click();
  await editor().getByRole('spinbutton', { name: '每 IP 每秒请求', exact: true }).fill('45');
  const scripts = editor().getByLabel('自定义GRL规则JSON', { exact: true });
  await scripts.fill('{}');
  await expect(editor().getByRole('button', { name: '保存端口配置' })).toBeDisabled();
  await editor().getByRole('tab', { name: '连接与访问' }).click();
  await editor().getByRole('button', { name: '定位未完成的配置', exact: true }).click();
  await expect(scripts).toHaveValue('{}');
  await scripts.fill('[]');
  await editor().getByRole('tab', { name: '访问名单' }).click();
  await editor().getByRole('button', { name: '添加条目', exact: true }).click();
  await editor().getByLabel('名单IP', { exact: true }).fill('192.0.2.20');
  await savePort();
  expect(configs.get(5000).policy.ruleset.qps.per_ip).toBe(45);
  expect(configs.get(5000).policy.ruleset.csrf_missing_origin).toBe('observe');
  expect(configs.get(5000).policy.ruleset.rules.find((r) => r.category === 'ssrf').mode).toBe('off');
  expect(configs.get(5000).policy.ip_group.entries.at(-1).value).toBe('192.0.2.20');
  report.checks.push('rules and IP entries support repeat edits and retain unrelated policy sections');

  await page.getByRole('button', { name: '表格与批量管理', exact: true }).click();
  const row = page.getByTestId('ports-table').locator('.el-table__body-wrapper tbody tr').filter({ hasText: '5000' });
  await row.getByRole('button', { name: '异常访问处置', exact: true }).click();
  await expect(editor().getByRole('tab', { name: '异常访问处置' })).toHaveAttribute('aria-selected', 'true');
  await cancelPort();
  expect(JSON.stringify(configs.get(5001))).toBe(otherPort);
  report.checks.push('table and card entries both open per-port defense without changing another service');

  await page.locator('.task-navigation').getByRole('link', { name: '策略模板', exact: true }).click();
  const templateCard = page.locator('.el-card').filter({ hasText: '可修改模板' });
  await templateCard.getByRole('button', { name: '编辑 / 重命名', exact: true }).click();
  const templateDialog = page.getByRole('dialog', { name: '用户模板 / 导入预览', exact: true });
  await templateDialog.getByLabel('模板名称', { exact: true }).fill('持续修改的模板');
  await templateDialog.getByRole('button', { name: '校验并预览', exact: true }).click();
  await templateDialog.getByRole('button', { name: '确认保存模板', exact: true }).click();
  await expect(templateDialog).toBeHidden();
  await page.reload();
  await page
    .locator('.el-card')
    .filter({ hasText: '持续修改的模板' })
    .getByRole('button', { name: '编辑 / 重命名', exact: true })
    .click();
  await expect(templateDialog.getByLabel('模板名称', { exact: true })).toHaveValue('持续修改的模板');
  report.checks.push('user templates can be renamed, validated, saved and reopened after refresh');
  await templateDialog.getByRole('button', { name: '关闭', exact: true }).click();

  await page.locator('.task-navigation').getByRole('link', { name: '我的服务', exact: true }).click();
  const cardViewButton = page.getByRole('button', { name: '卡片视图', exact: true });
  if (await cardViewButton.count()) await cardViewButton.click();
  await page.setViewportSize({ width: 390, height: 844 });
  await card().getByRole('button', { name: '异常访问处置', exact: true }).click();
  await expect(editor().getByTestId('defense-settings')).toBeVisible();
  await expect(editor().locator('.editor-task-select')).toContainText('异常访问处置');
  await editor().locator('.editor-task-select .el-select').click();
  await page.getByRole('option', { name: '监听入口', exact: true }).click();
  await expect(editor().getByText('关闭业务访问时', { exact: true })).toBeVisible();
  await editor().locator('.editor-task-select .el-select').click();
  await page.getByRole('option', { name: '异常访问处置', exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
    .toBeLessThanOrEqual(2);
  await page.screenshot({ path: path.join(output, 'defense-mobile.png'), animations: 'disabled' });
  await page.setViewportSize({ width: 1440, height: 1050 });
  await page.screenshot({ path: path.join(output, 'defense-desktop.png'), animations: 'disabled' });
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await expect
      .poll(async () => {
        const box = await editor().locator('.editor-actions').boundingBox();
        return !!box && box.y >= 0 && box.y + box.height <= 900;
      })
      .toBe(true);
    await expect
      .poll(() =>
        editor().evaluate((element) => {
          const body = element.closest('.el-dialog__body');
          return body.scrollHeight <= body.clientHeight + 2;
        }),
      )
      .toBe(true);
  }
  report.checks.push('service editor keeps save/close visible and has no outer double scroll at three widths');
  report.checks.push('mobile and desktop retain accessible honeypot edits without document overflow');
  expect(report.errors).toEqual([]);
  fs.writeFileSync(path.join(output, 'browser-report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), animations: 'disabled' });
  console.error(JSON.stringify(report, null, 2));
  throw error;
} finally {
  await browser.close();
}
