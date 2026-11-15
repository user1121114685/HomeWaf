// Actual built Vue screens; all backend traffic is intercepted and all state is isolated.
import { chromium, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';
const root = fileURLToPath(new URL('../..', import.meta.url)),
  dist = path.join(root, 'web/dist'),
  output = path.join(root, 'artifacts/api-ui-integration');
fs.mkdirSync(output, { recursive: true });
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
  name: i ? '家庭相册' : '群晖',
  protocol: 'http',
  listen_enabled: false,
  host_code: 'pipeline-' + port,
  host_names: [],
  upstream: 'http://127.0.0.1:5000',
  policy: structuredClone(templates.find((t) => t.id === 'family-nas').policy),
  template: { id: 'family-nas', version: 1 },
}));
const untouched = structuredClone(configs[1]);
const configRows = [
  { id: 'builtin', item: 'record_resp', value: '0', remarks: '内置配置', is_system: '1', item_class: 'system' },
  {
    id: 'custom',
    item: 'extension.example',
    value: 'sample',
    remarks: '测试读取方',
    is_system: '0',
    item_class: 'extension',
  },
];
const base = {
  blocking_paranoia_level: 1,
  detection_paranoia_level: 2,
  inbound_anomaly_score_threshold: 7,
  outbound_anomaly_score_threshold: 4,
  early_blocking: 0,
  enforce_bodyproc_urlencoded: 0,
};
const tuning = {
  ...base,
  rule_engine: 'DetectionOnly',
  request_body_limit: 0,
  request_body_in_memory_limit: 0,
  body_inspect_limit: 0,
};
let ipItems = [],
  failBase = false,
  failFile = false,
  emptyFiles = false,
  failRevoke = false,
  failRecommend = false,
  failAI = false;
let sessions = [15000, 15001].flatMap((port) =>
  ['owner', 'guest'].map((username, i) => ({
    id: port + '-' + username,
    port_id: 'port-' + port,
    username,
    client_ip: '192.0.2.' + (i + 1),
    created_at: 1780000000,
    last_seen: 1780000001,
    expires_at: 1990000000,
    port,
  })),
);
const subscription = {
  id: 'sub-1',
  channel_id: 'channel-1',
  message_type: 'rule_trigger',
  status: 1,
  filter_json: '',
  throttle_json: '',
  template_json: '',
  remarks: 'fixture',
};
const notifyLog = {
  id: 'log-1',
  subscription_id: 'sub-1',
  channel_id: 'channel-1',
  channel_name: '家庭通知',
  channel_type: 'bark',
  message_type: 'rule_trigger',
  message_title: '测试通知',
  message_content: 'fixture',
  status: 2,
  suppress_reason: 'cooldown',
  send_time: '2026-11-16 10:00:00',
};
const report = { checks: [], screens: [], errors: [], writes: [] };
const server = http.createServer((req, res) => {
  const name = decodeURIComponent(new URL(req.url, 'http://localhost').pathname),
    file = path.resolve(dist, '.' + (name === '/' ? '/index.html' : name));
  if (!file.startsWith(dist + path.sep) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
    res.writeHead(404);
    res.end();
    return;
  }
  res.setHeader(
    'Content-Type',
    {
      '.html': 'text/html',
      '.js': 'application/javascript',
      '.css': 'text/css',
      '.svg': 'image/svg+xml',
      '.json': 'application/json',
    }[path.extname(file)] || 'application/octet-stream',
  );
  fs.createReadStream(file).pipe(res);
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`,
  browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'isolated-ui-test');
  localStorage.setItem('lang', 'zh_CN');
  Object.defineProperty(crypto, 'randomUUID', { configurable: true, value: undefined });
});
await context.routeWebSocket('**/api/**', (ws) => ws.close());
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
  const write = () => report.writes.push({ endpoint, input });
  if (['account/info', 'account/current', 'account/detail'].includes(endpoint))
    return ok({ user_name: 'fixture', role: 'super_admin' });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint))
    return fail('Isolated fixture');
  if (endpoint === 'owasp/rules')
    return ok({ list: [{ id: 1001, file: 'rules/test.conf', message: 'fixture', raw: 'SecAction test' }], total: 1 });
  if (endpoint === 'owasp/files')
    return ok(
      emptyFiles
        ? []
        : [
            { file: 'rules/test.conf', rule_count: 1, mtime: 1780000000 },
            { file: 'rules/other.conf', rule_count: 2, mtime: 1780000001 },
          ],
    );
  if (endpoint === 'owasp/file_content')
    return failFile
      ? fail('模拟文件读取失败')
      : ok({ path: url.searchParams.get('path'), content: 'SecAction "id:1001,pass"\n# <img src=x onerror=alert(1)>' });
  if (endpoint === 'owasp/tuning') return ok(tuning);
  if (endpoint === 'owasp/crs_vars') return ok({ vars: {} });
  if (endpoint === 'owasp/base_config') {
    if (req.method() === 'POST') {
      write();
      if (failBase) return fail('模拟基线保存失败');
      Object.assign(base, input);
    }
    return ok(base);
  }
  if (endpoint === 'systemconfig/list') return ok({ list: configRows, total: configRows.length });
  if (endpoint === 'systemconfig/add') {
    write();
    if (configRows.some((row) => row.item === input.item)) return fail('配置键已存在');
    configRows.push({ ...input, id: 'new-custom', is_system: '0' });
    return ok({});
  }
  if (endpoint === 'systemconfig/del') {
    write();
    const index = configRows.findIndex((row) => row.id === url.searchParams.get('id'));
    if (index < 0 || configRows[index].is_system === '1') return fail('内置项不能删除');
    configRows.splice(index, 1);
    return ok({});
  }
  if (endpoint === 'wafhost/ipgroup/list')
    return ok({
      list: [{ id: 'g1', group_code: 'group-one', group_name: '家庭网段', item_count: ipItems.length }],
      total: 1,
    });
  if (endpoint === 'wafhost/ipgroup/refs') return ok({ ports: [] });
  if (endpoint === 'wafhost/ipgroup/validate') return ok({ valid: input.ip === '192.0.2.8', message: '地址格式无效' });
  if (endpoint === 'wafhost/ipgroupitem/list') return ok({ list: ipItems, total: ipItems.length });
  if (endpoint === 'wafhost/ipgroupitem/add') {
    write();
    ipItems.push({ ...input, id: 'ip-1' });
    return ok({});
  }
  if (endpoint === 'wafhost/ipgroupitem/edit') {
    write();
    const row = ipItems.find((item) => item.id === input.id);
    if (!row) return fail('IP 条目不存在');
    Object.assign(row, input);
    return ok({});
  }
  if (endpoint === 'notify/log/list') return ok({ list: [notifyLog], total: 1 });
  if (endpoint === 'notify/log/detail') return ok(notifyLog);
  if (endpoint === 'notify/channel/types') return ok([{ type: 'bark', name: 'Bark' }]);
  if (endpoint === 'notify/channel/list')
    return ok({ list: [{ id: 'channel-1', name: '家庭通知', type: 'bark', status: 1 }], total: 1 });
  if (endpoint === 'notify/subscription/list') return ok({ list: [subscription], total: 1 });
  if (endpoint === 'notify/subscription/template_vars') return ok([]);
  if (endpoint === 'notify/subscription/config') {
    write();
    return ok({});
  }
  if (endpoint === 'pipeline/overview')
    return ok({ ports: configs.map((config) => ({ config, listener: { running: false }, stats: {} })), issues: {} });
  if (endpoint === 'pipeline/templates') return ok(templates);
  if (endpoint === 'pipeline/cc/hits')
    return ok([
      { req_id: 'request-cc', time_ms: 1780000000000, client_ip: '192.0.2.1', verdict: 'deny', observe: false },
    ]);
  if (endpoint === 'pipeline/cc/recommend')
    return failRecommend
      ? fail('模拟流量读取失败')
      : ok({
          supported: true,
          requests: 1200,
          clients: 24,
          window_seconds: input.window_seconds,
          p99: 12,
          suggested: 36,
          from: 1780000000000,
          to: 1780001000000,
          scope: '当前日志分区，归档未纳入',
        });
  if (endpoint === 'pipeline/gate/sessions')
    return ok(sessions.filter((s) => s.port === Number(url.searchParams.get('port'))));
  if (endpoint === 'pipeline/gate/users') return ok([]);
  if (endpoint === 'pipeline/gate/sessions/revoke') {
    write();
    if (failRevoke) return fail('模拟持久化失败，请重试');
    const before = sessions.length;
    sessions = sessions.filter(
      (s) => s.port !== input.port || (input.scope === 'account' && s.username !== input.username),
    );
    return ok({ revoked: before - sessions.length });
  }
  if (endpoint === 'pipeline/ports/save') {
    write();
    const idx = configs.findIndex((c) => c.port === input.config.port);
    if (idx < 0 || configs[idx].revision !== input.config.revision) return fail('配置版本冲突');
    configs[idx] = { ...input.config, revision: input.config.revision + 1 };
    return ok(configs[idx]);
  }
  if (endpoint === 'wafhost/rule/format')
    return ok({
      rule_content: `rule R${input.rule_code.replaceAll('-', '')} "" salience 10 { when MF.URL == "/private" then RF.Deny(); }`,
    });
  if (endpoint === 'gpt/config') return ok({ has_token: true, gpt_url: 'https://ai.example', gpt_model: 'fixture' });
  if (endpoint === 'wafhost/rule/aigen')
    return failAI
      ? fail('模拟 AI 服务失败')
      : ok({ rule_content: 'rule RAI "" salience 1 { when MF.URL == "/private" then RF.Deny(); }', valid: true });
  if (endpoint === 'wafhost/rule/aiprompt') return ok('生成安全规则，不自动保存。');
  if (endpoint === 'pipeline/explain') {
    write();
    return ok({ req_id: 'dry-run', verdict: { action: 'allow', reason: 'preview' }, trace: [], hits: [] });
  }
  if (req.method() === 'POST' && !endpoint.endsWith('/list')) return fail('Unhandled isolated write: ' + endpoint);
  return ok({ list: [], total: 0 });
});
const page = await context.newPage();
page.on('pageerror', (err) => report.errors.push(err.message));
const dialog = () => page.locator('.el-dialog:visible').last();
async function goto(route) {
  await page.goto(origin + '/?appearance=light&verification=' + Date.now() + '#' + route);
  await expect(page.locator('body')).not.toContainText('页面不存在');
}
async function capture(name) {
  for (const width of [1440, 768, 390])
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width, height: 1000 });
      await page.evaluate((theme) => {
        document.documentElement.dataset.theme = theme;
        document.documentElement.classList.toggle('dark', theme === 'dark');
        document.activeElement?.blur();
        window.scrollTo(0, 0);
      }, theme);
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth))
        .toBeLessThanOrEqual(2);
      await page.screenshot({
        path: path.join(output, `${name}-${width}-${theme}.png`),
        fullPage: true,
        animations: 'disabled',
      });
      report.screens.push({ name, width, theme });
    }
  await page.setViewportSize({ width: 1440, height: 1000 });
}
try {
  await goto('/sys/OwaspManage');
  await page.getByRole('button', { name: '查看规则文件', exact: true }).click();
  await page.getByRole('combobox', { name: '规则文件', exact: true }).click();
  await page.getByRole('option', { name: 'rules/test.conf', exact: true }).click();
  await expect(page.getByLabel('规则文件原文')).toContainText('<img');
  expect(await page.getByLabel('规则文件原文').locator('img').count()).toBe(0);
  await capture('owasp-file');
  failFile = true;
  await page.getByRole('button', { name: '刷新文件列表' }).click();
  await expect(dialog()).toContainText('模拟文件读取失败');
  failFile = false;
  await page.getByRole('button', { name: '重试读取文件' }).click();
  await expect(page.getByLabel('规则文件原文')).toBeVisible();
  emptyFiles = true;
  await page.getByRole('button', { name: '刷新文件列表' }).click();
  await expect(dialog()).toContainText('尚无可查看的规则文件');
  await expect(page.getByLabel('规则文件原文')).toHaveCount(0);
  emptyFiles = false;
  await page.getByRole('button', { name: '返回规则列表' }).click();
  report.checks.push('OWASP text preview, failure/retry and empty state clear stale content; no HTML execution');
  await goto('/sys/OwaspManage?tab=tuning');
  await page.getByRole('button', { name: '查看高级基线' }).click();
  await expect(page.getByRole('spinbutton', { name: '基线请求异常分阈值' })).toHaveValue('7');
  await page.getByRole('spinbutton', { name: '基线请求异常分阈值' }).fill('9');
  await capture('owasp-base');
  failBase = true;
  await page.getByRole('button', { name: '保存高级基线', exact: true }).click();
  await page.locator('.el-message-box').getByRole('button').last().click();
  await expect(dialog()).toContainText('模拟基线保存失败');
  await expect(page.getByRole('spinbutton', { name: '基线请求异常分阈值' })).toHaveValue('9');
  failBase = false;
  await page.getByRole('button', { name: '保存高级基线', exact: true }).click();
  await page.locator('.el-message-box').getByRole('button').last().click();
  await expect.poll(() => base.inbound_anomaly_score_threshold).toBe(9);
  expect(tuning.inbound_anomaly_score_threshold).toBe(7);
  report.checks.push('Independent baseline failure retains input, save reads back and preserves tuning');
  await goto('/sys/SystemConfig');
  await page.getByRole('button', { name: '高级配置项维护' }).click();
  await expect(dialog()).not.toContainText('record_resp');
  await page.getByRole('textbox', { name: '新增配置键' }).fill('extension.extra');
  await page.getByRole('textbox', { name: '新增配置值' }).fill('value');
  await page.getByRole('textbox', { name: '配置用途' }).fill('指定扩展读取方');
  await page.getByRole('button', { name: '新增配置项', exact: true }).click();
  await expect(dialog()).toContainText('extension.extra');
  await capture('advanced-config');
  await dialog()
    .getByRole('row')
    .filter({ hasText: 'extension.extra' })
    .getByRole('button', { name: '删除', exact: true })
    .click();
  await page.locator('.el-message-box').getByRole('button', { name: '取消' }).click();
  expect(configRows.some((r) => r.item === 'extension.extra')).toBe(true);
  await dialog()
    .getByRole('row')
    .filter({ hasText: 'extension.extra' })
    .getByRole('button', { name: '删除', exact: true })
    .click();
  await page.locator('.el-message-box').getByRole('button', { name: '删除此项' }).click();
  await expect.poll(() => configRows.some((r) => r.item === 'extension.extra')).toBe(false);
  expect(configRows.some((r) => r.id === 'builtin')).toBe(true);
  report.checks.push('Advanced create/reopen/delete cancellation; built-ins have no delete action');
  await goto('/waf-host/wafipgroup');
  await page.getByRole('button', { name: '管理IP', exact: true }).click();
  await page.getByRole('button', { name: '添加IP', exact: true }).click();
  await dialog().getByRole('textbox').first().fill('bad-ip');
  await page.getByRole('button', { name: '检查地址格式' }).click();
  await expect(dialog()).toContainText('地址格式无效');
  expect(ipItems).toHaveLength(0);
  await dialog().getByRole('textbox').first().fill('192.0.2.8');
  await page.getByRole('button', { name: '检查地址格式' }).click();
  await expect(dialog()).toContainText('地址格式有效');
  await capture('ip-validation');
  await dialog().getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => ipItems.length).toBe(1);
  expect(ipItems[0].group_code).toBe('group-one');
  await goto('/waf-host/wafipgroup');
  await page.getByRole('button', { name: '管理IP', exact: true }).click();
  await page.locator('.el-drawer:visible').getByRole('button', { name: '编辑', exact: true }).click();
  await expect(dialog().getByRole('textbox').first()).toHaveValue('192.0.2.8');
  await dialog().getByRole('textbox').last().fill('重开后修改');
  await dialog().getByRole('button', { name: '确认', exact: true }).click();
  await expect.poll(() => ipItems[0].remarks).toBe('重开后修改');
  expect(ipItems).toHaveLength(1);
  report.checks.push('IP invalid input blocked; create, leave, reopen and edit the same selected-group item');
  await goto('/notify/log');
  await page.getByRole('button', { name: '查看详情', exact: true }).click();
  await page.getByRole('button', { name: '查看对应订阅' }).click();
  await expect(page.locator('.el-drawer:visible')).toContainText('家庭通知');
  await capture('linked-subscription');
  await page.locator('.el-drawer:visible').getByRole('button', { name: '关闭', exact: true }).click();
  await page.getByRole('button', { name: '返回通知日志' }).click();
  await expect(dialog()).toContainText('测试通知');
  await goto('/notify/subscription?subscription_id=missing&return_to=%2Fnotify%2Flog');
  await expect(page.getByText('这条日志对应的订阅或渠道已删除，或当前账号无权查看。现有订阅未被修改。')).toBeVisible();
  report.checks.push(
    'Notification log opens exact subscription, returns to original log detail, missing subscription is explicit',
  );
  await goto('/waf-access/wafaccesssession?host_code=pipeline-15000');
  await expect(page.getByRole('button', { name: '撤销此服务全部会话' })).toBeEnabled();
  await capture('sessions');
  await page.getByRole('combobox', { name: '会话账号范围' }).press('ArrowDown');
  await page.getByRole('option', { name: 'owner', exact: true }).click();
  await page.getByRole('button', { name: '撤销该账号全部会话' }).click();
  await page.locator('.el-message-box').getByRole('button', { name: '取消' }).click();
  expect(sessions).toHaveLength(4);
  failRevoke = true;
  await page.getByRole('button', { name: '撤销该账号全部会话' }).click();
  await page.locator('.el-message-box').getByRole('button', { name: '立即撤销' }).click();
  await expect(page.getByRole('alert')).toContainText('模拟持久化失败');
  failRevoke = false;
  await page.getByRole('button', { name: '刷新会话', exact: true }).click();
  await page.getByRole('button', { name: '撤销该账号全部会话' }).click();
  await page.locator('.el-message-box').getByRole('button', { name: '立即撤销' }).click();
  await expect.poll(() => sessions.length).toBe(3);
  expect(sessions.filter((s) => s.port === 15001)).toHaveLength(2);
  await page.reload();
  await page.getByRole('button', { name: '撤销此服务全部会话' }).click();
  await page.locator('.el-message-box').getByRole('button', { name: '立即撤销' }).click();
  await expect.poll(() => sessions.filter((s) => s.port === 15000).length).toBe(0);
  expect(sessions.filter((s) => s.port === 15001)).toHaveLength(2);
  report.checks.push('Account/service session revocation, cancel, failure/retry and reload; other service untouched');
  await goto('/waf-host/wafanticclist?host_code=pipeline-15000');
  await page.getByRole('button', { name: '按历史流量估算阈值' }).click();
  failRecommend = true;
  await page.getByRole('button', { name: '读取流量并估算' }).click();
  await expect(dialog()).toContainText('模拟流量读取失败');
  failRecommend = false;
  await page.getByRole('button', { name: '读取流量并估算' }).click();
  await expect(dialog()).toContainText('36 次');
  await page.getByRole('button', { name: '填入草稿阈值' }).click();
  await expect(page.getByRole('spinbutton', { name: 'CC 阈值', exact: true })).toHaveValue('36');
  expect(configs[0].policy.ruleset.cc.threshold).not.toBe(36);
  await page.getByRole('combobox', { name: '临时挑战模式' }).press('ArrowDown');
  await page.getByRole('option', { name: '要求挑战', exact: true }).click();
  await page.getByRole('button', { name: '30 分钟后关闭', exact: true }).click();
  await capture('cc-workbench');
  await page.getByRole('button', { name: '保存 CC 防护', exact: true }).click();
  await expect.poll(() => configs[0].policy.ruleset.cc.threshold).toBe(36);
  expect(configs[1]).toEqual(untouched);
  await page.reload();
  await expect(page.getByRole('spinbutton', { name: 'CC 阈值', exact: true })).toHaveValue('36');
  await page.getByRole('button', { name: '查看最近 CC 命中' }).click();
  await expect(dialog()).toContainText('192.0.2.1');
  report.checks.push(
    'CC recommendation applies to draft only; challenge and threshold save/reopen on one service; real recent-hit entry',
  );
  await goto('/waf-host/wafrule?host_code=pipeline-15000');
  await page.getByRole('button', { name: '管理自定义规则' }).click();
  await page.getByRole('button', { name: '构建 / AI 生成规则' }).click();
  await page.getByRole('textbox', { name: '匹配内容 1' }).fill('/private');
  await page.getByRole('button', { name: '生成规则预览' }).click();
  await expect(page.getByRole('textbox', { name: '生成规则源码' })).toHaveValue(/RF.Deny/);
  await page.getByRole('button', { name: '验证并解释草稿' }).click();
  await expect(dialog()).toContainText('已通过当前服务');
  await capture('rule-assistant');
  await page.getByRole('button', { name: '加入当前服务草稿' }).click();
  await page.getByRole('button', { name: '保存配置', exact: true }).click();
  await expect.poll(() => configs[0].policy.ruleset.scripts?.length).toBe(1);
  await page.reload();
  await expect(page.locator('.rule-collection')).toContainText(configs[0].policy.ruleset.scripts[0].id);
  await page.getByRole('button', { name: '构建 / AI 生成规则' }).click();
  await page.getByRole('radio', { name: 'AI 生成', exact: true }).press('Space');
  await page.getByRole('textbox', { name: '防护需求' }).fill('观察私有路径');
  failAI = true;
  await page.getByRole('button', { name: '生成规则', exact: true }).click();
  await expect(dialog()).toContainText('模拟 AI 服务失败');
  await expect(page.getByRole('textbox', { name: '防护需求' })).toHaveValue('观察私有路径');
  failAI = false;
  await page.getByRole('button', { name: '生成规则', exact: true }).click();
  await page.getByRole('button', { name: '查看生成提示词' }).click();
  await expect(page.getByRole('textbox', { name: '规则生成提示词' })).toHaveValue('生成安全规则，不自动保存。');
  report.checks.push(
    'Discoverable rule builder, compiled preview, draft/save/reopen; AI failure keeps intent; no automatic save',
  );
  expect(report.errors).toEqual([]);
  expect(configs[1]).toEqual(untouched);
  fs.writeFileSync(path.join(output, 'browser-results.json'), JSON.stringify(report, null, 2));
  console.log(
    JSON.stringify({
      checks: report.checks.length,
      screens: report.screens.length,
      errors: report.errors,
      writes: report.writes.length,
    }),
  );
} catch (error) {
  await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
  fs.writeFileSync(path.join(output, 'failure-state.txt'), await page.locator('body').innerText());
  fs.writeFileSync(path.join(output, 'browser-results.json'), JSON.stringify(report, null, 2));
  throw error;
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
