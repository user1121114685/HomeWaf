// Real built Vue UI, isolated encrypted API fixtures; no live certificate writes.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const dist = path.join(root, 'web/dist');
const output = path.join(root, 'artifacts/certificate-editor');
fs.mkdirSync(output, { recursive: true });
const cert =
  '-----BEGIN CERTIFICATE-----\n' +
  'U1lOVEhFVElDLUNFUlRJRklDQVRFLUZJWFRVUkU=\n'.repeat(24) +
  '-----END CERTIFICATE-----';
const key = '-----BEGIN PRIVATE KEY-----\n' + 'U1lOVEhFVElDLUtFWS1GSVhUVVJF\n'.repeat(12) + '-----END PRIVATE KEY-----';
const original = {
  id: 'cert-one',
  domains: 'nas.example.test',
  issuer: 'Example Test CA',
  valid_to: '2027-12-01 00:00:00',
  expiration_info: '还有 380 天到期',
  bind_hosts: ['家庭存储 · HTTPS :15000'],
  cert_content: cert,
  key_content: key,
  auto_load_path: 1,
  cert_path: '/etc/ssl/nas.crt',
  key_path: '/etc/ssl/nas.key',
  export_cert_path: '',
  export_key_path: '',
  export_status: '尚未配置导出',
};
let records = [
  structuredClone(original),
  { ...structuredClone(original), id: 'cert-two', domains: 'photos.example.test' },
];
let failure = '',
  listFailure = false,
  releaseSave;
let writes = 0,
  additions = 0;
const report = { checks: [], views: [], errors: [] };
const server = http.createServer((req, res) => {
  const requested = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
  const file = path.resolve(dist, '.' + (requested === '/' ? '/index.html' : requested));
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
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  reducedMotion: 'reduce',
  serviceWorkers: 'block',
});
await context.addInitScript(() => {
  localStorage.setItem('access_token', 'certificate-editor-fixture');
  localStorage.setItem('lang', 'zh_CN');
});
await context.route('**/*', (route) =>
  new URL(route.request().url()).origin === origin ? route.continue() : route.abort(),
);
await context.routeWebSocket('**/api/**', (socket) => socket.close());
await context.route('**/api/**', async (route) => {
  const request = route.request();
  const endpoint = new URL(request.url()).pathname.split('/api/v1/')[1] || '';
  const respond = (code, data, msg = '') =>
    route.fulfill({ json: { code, data: AesEncrypt(JSON.stringify(data)), msg } });
  if (endpoint.includes('handshake') || ['public/seckey', 'sysinfo/checkversion'].includes(endpoint))
    return respond(-1, {}, 'Isolated fixture');
  if (endpoint === 'sslconfig/list')
    return listFailure
      ? respond(-1, {}, '读取失败')
      : respond(0, { list: structuredClone(records), total: records.length });
  if (endpoint === 'sslconfig/edit' || endpoint === 'sslconfig/add') {
    writes++;
    let input;
    try {
      input = JSON.parse(AesDecrypt(request.postData()));
    } catch {
      input = JSON.parse(request.postData());
    }
    if (failure === 'pending')
      await new Promise((resolve) => {
        releaseSave = resolve;
      });
    if (failure === 'network') return route.abort('failed');
    if (failure === 'forbidden') return respond(-403, {}, '无权限：仅系统管理员可以修改证书');
    if (failure === 'key')
      return respond(
        -1,
        { field_errors: { key_content: '私钥格式非法或与证书不匹配，请检查配对' } },
        '私钥格式非法或与证书不匹配，请检查配对',
      );
    if (endpoint === 'sslconfig/add') {
      additions++;
      records.push({ ...structuredClone(original), ...input, id: 'cert-added', domains: 'added.example.test' });
    } else {
      expect(input.id).toBe('cert-one');
      Object.assign(records[0], input);
    }
    if (failure === 'export') {
      records[0].export_status = '导出失败：目标目录未允许';
      return respond(0, { export_failed: true }, '编辑成功，但证书导出失败：目标目录未允许');
    }
    return respond(0, { export_failed: false }, '证书已保存');
  }
  return respond(0, { list: [], total: 0 });
});
const page = await context.newPage();
page.setDefaultTimeout(10000);
page.on('pageerror', (error) => report.errors.push(error.message));
const editor = page.getByTestId('certificate-editor');
const certField = editor.getByRole('textbox', { name: /证书内容（PEM）/ });
const keyField = editor.getByRole('textbox', { name: /私钥内容（PEM）/ });
const save = editor.getByRole('button', { name: '保存证书', exact: true });
async function open() {
  await page
    .locator('.el-table__row')
    .filter({ hasText: 'nas.example.test' })
    .getByRole('button', { name: '编辑', exact: true })
    .click();
  await expect(editor).toBeVisible();
}
async function cancel() {
  await editor.getByRole('button', { name: '取消', exact: true }).click();
  await expect(editor).toBeHidden();
}
const check = (name) => {
  report.checks.push(name);
  console.log('PASS ' + name);
};
try {
  await page.goto(origin + '/#/waf-host/wafsslconfiglist');
  await open();
  await expect(certField).toHaveValue(cert);
  await expect(keyField).toHaveValue(key);
  await certField.fill('broken certificate');
  await save.click();
  await expect(editor.locator('.certificate-error')).toContainText('请检查证书');
  await expect(editor.locator('.certificate-error')).toBeFocused();
  expect(writes).toBe(0);
  await editor.locator('.certificate-error a').first().click();
  await expect(certField).toBeFocused();
  await editor.getByRole('button', { name: '取消', exact: true }).click();
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
  await expect(page.locator('.el-message-box')).toBeHidden();
  await expect(certField).toHaveValue('broken certificate');
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: '放弃修改', exact: true }).click();
  await expect(editor).toBeHidden();
  await open();
  await expect(certField).toHaveValue(cert);
  check('invalid input is blocked; error focus, cancel, Escape and discard retain the saved certificate');

  await editor
    .locator('input[type=file]')
    .first()
    .setInputFiles({ name: 'oversized.crt', mimeType: 'text/plain', buffer: Buffer.alloc(2 * 1024 * 1024 + 1) });
  await expect(editor.locator('.certificate-error')).toContainText('2 MiB');
  await expect(certField).toHaveValue(cert);
  check('rejected file import preserves the existing certificate content');

  await editor
    .locator('input[type=file]')
    .first()
    .setInputFiles({ name: 'replacement.pem', mimeType: 'text/plain', buffer: Buffer.from(cert + '\n') });
  await expect(certField).toHaveValue(cert + '\n');
  await expect(editor.getByRole('status').filter({ hasText: 'replacement.pem' })).toContainText('尚未保存');
  await editor
    .locator('input[type=file]')
    .nth(1)
    .setInputFiles({ name: 'matching.key', mimeType: 'text/plain', buffer: Buffer.from(key + '\n') });
  await expect(keyField).toHaveValue(key + '\n');
  await expect(editor.getByRole('status').filter({ hasText: '当前保存的信息' })).toBeVisible();
  failure = 'key';
  await save.click();
  await expect(editor.locator('.el-form-item__error')).toContainText('私钥格式非法或与证书不匹配');
  await expect(keyField).toHaveValue(key + '\n');
  check('file import only fills the draft; encrypted backend field errors remain actionable');
  failure = 'forbidden';
  await save.click();
  await expect(editor.locator('.certificate-error')).toContainText('仅系统管理员');
  await expect(certField).toHaveValue(cert + '\n');
  failure = 'network';
  await save.click();
  await expect(editor.locator('.certificate-error')).toContainText('无法确认保存结果');
  await expect(keyField).toHaveValue(key + '\n');
  check('permission and network failures preserve the draft for retry');
  failure = 'pending';
  const before = writes;
  await editor.locator('form').evaluate((form) => {
    form.requestSubmit();
    form.requestSubmit();
  });
  await expect.poll(() => writes).toBe(before + 1);
  await expect(save).toBeDisabled();
  await page.keyboard.press('Escape');
  await expect(editor).toBeVisible();
  releaseSave();
  await expect(editor).toBeHidden();
  failure = '';
  expect(records[0].cert_content).toBe(cert + '\n');
  expect(records[1].cert_content).toBe(cert);
  await open();
  await expect(certField).toHaveValue(cert + '\n');
  check('duplicate submit is blocked; save and reopen preserve the selected record only');

  await editor.locator('summary').click();
  await editor.getByRole('checkbox', { name: '路径自动加载', exact: true }).focus();
  await page.keyboard.press('Space');
  await expect(editor.getByRole('checkbox', { name: '路径自动加载', exact: true })).not.toBeChecked();
  await editor.getByRole('textbox', { name: '导出crt文件路径', exact: true }).fill('/exports/nas.crt');
  failure = 'export';
  await save.click();
  await expect(editor).toBeHidden();
  await expect(page.locator('.el-alert--warning')).toContainText('编辑成功，但证书导出失败');
  expect(records[0].auto_load_path).toBe(0);
  expect(records[0].export_cert_path).toBe('/exports/nas.crt');
  failure = '';
  await open();
  await expect(editor.locator('summary')).toContainText('自动加载已关闭');
  await expect(editor.locator('summary')).toContainText('已配置导出');
  await expect(editor.locator('.certificate-export-result')).toContainText('导出失败');
  check('optional settings persist; partial export failure and enabled states stay visible');
  await expect(page.locator('.el-message')).toHaveCount(0, { timeout: 12000 });

  for (const width of [1440, 768, 390])
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width, height: width === 768 ? 1024 : width === 390 ? 844 : 900 });
      await page.evaluate((mode) => {
        document.documentElement.classList.toggle('dark', mode === 'dark');
      }, theme);
      await editor.locator('.el-dialog__body').evaluate((body) => {
        body.scrollTop = 0;
      });
      const metrics = await editor.evaluate((dialog) => {
        const body = dialog.querySelector('.el-dialog__body'),
          footer = dialog.querySelector('.el-dialog__footer');
        const r = footer.getBoundingClientRect();
        const textarea = dialog.querySelector('textarea');
        const lum = (rgb) =>
          rgb
            .slice(0, 3)
            .map((v) => v / 255)
            .map((v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
            .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
        const text = getComputedStyle(dialog.querySelector('.certificate-hint'))
          .color.match(/[\d.]+/g)
          .map(Number);
        const bg = getComputedStyle(dialog)
          .backgroundColor.match(/[\d.]+/g)
          .map(Number);
        const a = lum(text),
          b = lum(bg);
        return {
          viewport: innerWidth,
          overflow: body.scrollWidth > body.clientWidth + 1,
          footerTop: r.top,
          footerBottom: r.bottom,
          height: innerHeight,
          textarea: textarea.clientWidth,
          field: textarea.closest('.certificate-field').clientWidth,
          font: getComputedStyle(textarea).fontFamily,
          contrast: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
        };
      });
      expect(metrics.viewport).toBe(width);
      expect(metrics.overflow).toBe(false);
      expect(metrics.footerBottom).toBeLessThanOrEqual(metrics.height);
      expect(metrics.footerTop).toBeGreaterThan(0);
      expect(metrics.field - metrics.textarea).toBeLessThan(4);
      expect(metrics.font).toContain('Geist Mono');
      expect(metrics.contrast).toBeGreaterThanOrEqual(4.5);
      await page.screenshot({ path: path.join(output, `${width}-${theme}.png`) });
      await editor.locator('summary').click();
      await editor.locator('.el-dialog__body').evaluate((body) => {
        body.scrollTop = body.scrollHeight;
      });
      await page.screenshot({ path: path.join(output, `${width}-${theme}-settings.png`) });
      await editor.locator('summary').evaluate((summary) => summary.click());
      report.views.push({ width, theme, ...metrics });
    }
  check('desktop, tablet and phone in light/dark themes: full-width fields, readable text and visible footer');
  await cancel();
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole('button', { name: '新建', exact: true }).click();
  await expect(certField).toHaveValue('');
  await certField.fill(cert);
  await keyField.fill(key);
  await save.click();
  await expect(editor).toBeHidden();
  expect(additions).toBe(1);
  check('create uses the same form and submits exactly once');
  await open();
  await certField.fill(cert + '\nroute-draft');
  let refreshPrompt = false;
  page.once('dialog', async (dialog) => {
    refreshPrompt = dialog.type() === 'beforeunload';
    await dialog.dismiss();
  });
  await page.reload().catch(() => undefined);
  expect(refreshPrompt).toBe(true);
  await expect(certField).toHaveValue(cert + '\nroute-draft');
  check('browser refresh prompts before discarding unsaved certificate input');
  await page.evaluate(() => {
    location.hash = '#/help';
  });
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
  await expect(page.locator('.el-message-box')).toBeHidden();
  await expect(certField).toHaveValue(cert + '\nroute-draft');
  await expect(page).toHaveURL(/\/waf-host\/wafsslconfiglist$/);
  await page.evaluate(() => {
    location.hash = '#/help';
  });
  await page.getByRole('button', { name: '放弃修改', exact: true }).click();
  await expect(editor).toBeHidden();
  const stored = await page.evaluate(() =>
    JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }),
  );
  expect(stored).not.toContain('PRIVATE KEY');
  expect(stored).not.toContain('route-draft');
  check('navigation guards protect the draft without persisting certificate material');
  listFailure = true;
  await page.goto(origin + '/#/waf-host/wafsslconfiglist');
  await expect(page.locator('.certificate-list-error')).toBeVisible();
  listFailure = false;
  await page.locator('.certificate-list-error').getByRole('button', { name: '重试' }).click();
  await open();
  await cancel();
  check('failed list reads have an explicit recovery action');
  expect(report.errors).toEqual([]);
} finally {
  fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(report, null, 2));
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
