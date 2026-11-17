// Runs a disposable native instance. No live credentials or external CA/DNS services are used.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import net from 'node:net';
import https from 'node:https';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { AesDecrypt } from '../src/utils/legacyCipher.ts';

const root = fileURLToPath(new URL('../..', import.meta.url));
const source = path.resolve(process.argv[2] || path.join(root, 'artifacts/HomeWaf64.exe'));
const runtimeRoot = path.join(root, '.cache/dev/runtime');
fs.mkdirSync(runtimeRoot, { recursive: true });
const runtime = fs.mkdtempSync(path.join(runtimeRoot, 'management-bootstrap-'));
const binary = path.join(runtime, path.basename(source));
fs.copyFileSync(source, binary);
const reserve = net.createServer();
await new Promise((resolve) => reserve.listen(0, '127.0.0.1', resolve));
const port = reserve.address().port;
await new Promise((resolve) => reserve.close(resolve));
fs.mkdirSync(path.join(runtime, 'conf'));
fs.writeFileSync(
  path.join(runtime, 'conf/config.yml'),
  `local_port: ${port}\nsecurity:\n  ssl_enable: true\n  ssl_auto_local: true\n  ip_whitelist: 127.0.0.1/32\nnotice:\n  isenable: false\n`,
);
const env = { ...process.env, HOMEWAF_DEV: '1', HOMEWAF_UPDATE_URL: '', HOMEWAF_RELEASE_URL: '' };
delete env.SamWafIDE;
const log = fs.openSync(path.join(runtime, 'runtime.log'), 'a');
const processHandle = spawn(binary, [], { cwd: runtime, env, windowsHide: true, stdio: ['ignore', log, log] });
const origin = `http://127.0.0.1:${port}`;
const shots = path.join(root, 'artifacts/install-hotfix');
fs.mkdirSync(shots, { recursive: true });
let browser;
async function wait(check, timeout = 60000) {
  const end = Date.now() + timeout;
  let last;
  while (Date.now() < end) {
    try {
      const result = await check();
      if (result) return result;
    } catch (e) {
      last = e;
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`Timed out: ${last?.message || 'condition not met'}; isolated log: ${runtime}`);
}
function tlsRequest(host, requestPath = '/', method = 'GET', body, key = '') {
  return new Promise((resolve, reject) => {
    const request = https.request(
      {
        hostname: '127.0.0.1',
        port,
        servername: net.isIP(host) ? undefined : host,
        ca: fs.readFileSync(path.join(runtime, 'data/ssl/manager/ca.crt')),
        path: requestPath,
        method,
        headers: { Host: `${host}:${port}`, 'Content-Type': 'application/json', ...(key ? { 'X-API-Key': key } : {}) },
        timeout: 3000,
      },
      (response) => {
        const chunks = [];
        response.on('data', (chunk) => chunks.push(chunk));
        response.on('end', () =>
          resolve({ status: response.statusCode, body: Buffer.concat(chunks).toString(), headers: response.headers }),
        );
      },
    );
    request.on('error', reject);
    request.on('timeout', () => request.destroy(new Error('TLS timeout')));
    request.end(body ? JSON.stringify(body) : undefined);
  });
}
try {
  await wait(async () => (await fetch(origin, { signal: AbortSignal.timeout(2000) })).status === 200);
  const initialText = fs.readFileSync(path.join(runtime, 'data/initial_password.txt'), 'utf8');
  const initial = initialText.match(/初始随机口令:\s*(.+)/)?.[1];
  if (!initial) throw new Error('Initial random password was not created');
  const info = spawnSync(binary, ['console-info', '--wait', '10s', '--show-initial-password'], {
    cwd: runtime,
    env,
    encoding: 'utf8',
    windowsHide: true,
  });
  expect(info.status).toBe(0);
  expect(info.stdout).toContain(initial);
  expect(info.stdout).toContain('https://');
  expect(info.stdout).toContain('SHA-256:');
  expect(info.stdout).not.toMatch(/Samwaf/i);
  const publicResponse = await (
    await fetch(origin + '/api/v1/public/managementTls', {
      headers: { 'X-Request-Time': String(Math.floor(Date.now() / 1000)), 'X-Request-Id': crypto.randomUUID() },
    })
  ).json();
  const publicInfo =
    typeof publicResponse.data === 'string' ? JSON.parse(AesDecrypt(publicResponse.data)) : publicResponse.data;
  expect(publicInfo.https_configured).toBe(true);
  expect(publicInfo.host_covered).toBe(true);
  expect(publicInfo.ca_pem).toContain('BEGIN CERTIFICATE');
  expect(publicInfo.ca_pem).not.toContain('PRIVATE KEY');
  expect((await tlsRequest('127.0.0.1')).status).toBe(200);
  console.log(
    'PASS: random password shown by CLI; HTTP and certificate-verified HTTPS serve the console; public CA has no private key',
  );

  browser = await chromium.launch({ channel: 'chrome', headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  const pageErrors = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));
  await page.goto(origin + '/#/login');
  await expect(page.getByTestId('management-trust')).toBeVisible();
  const downloadEvent = page.waitForEvent('download');
  await page.getByRole('button', { name: '下载 HomeWaf 根证书' }).click();
  expect((await downloadEvent).suggestedFilename()).toBe('homewaf-local-ca.crt');
  await page.locator('input[autocomplete="username"]').fill('admin');
  await page.locator('input[autocomplete="current-password"]').fill(initial);
  await page.locator('.login-password button[type="submit"]').click();
  const dialog = page.getByRole('dialog', { name: '修改密码', exact: true });
  await expect(dialog).toBeVisible();
  for (const dark of [false, true]) {
    await page.evaluate((value) => document.documentElement.classList.toggle('dark', value), dark);
    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
      await page.waitForTimeout(350);
      expect(await dialog.evaluate((el) => el.closest('.login-container') === null)).toBe(true);
      const box = await dialog.boundingBox();
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
      const submit = dialog.locator('button[type="submit"]');
      await submit.scrollIntoViewIfNeeded();
      await expect(submit).toBeInViewport();
      for (const field of await dialog.locator('input').all()) {
        await field.scrollIntoViewIfNeeded();
        await expect(field).toBeInViewport();
      }
      await page.screenshot({ path: path.join(shots, `password-${dark ? 'dark' : 'light'}-${width}.png`) });
    }
  }
  await page.keyboard.press('Escape');
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('.el-dialog__headerbtn')).toHaveCount(0);
  const fields = dialog.locator('input[type="password"]');
  await fields.nth(0).fill('weak');
  await fields.nth(1).fill('weak');
  await dialog.locator('button[type="submit"]').click();
  await expect(dialog.locator('.el-alert--error')).toBeVisible();
  await expect(fields.nth(0)).toHaveValue('weak');
  const fresh = 'Local-Flow!42-Independent';
  await fields.nth(0).fill(fresh);
  await fields.nth(1).fill(fresh);
  let submits = 0;
  page.on('request', (req) => {
    if (req.url().endsWith('/account/changemypwd')) submits++;
  });
  await dialog.locator('form').evaluate((form) => {
    form.requestSubmit();
    form.requestSubmit();
  });
  await expect(dialog).toBeHidden({ timeout: 15000 });
  await expect(page).not.toHaveURL(/#\/login/);
  expect(submits).toBe(1);
  expect(pageErrors).toEqual([]);
  await context.close();
  const again = await browser.newPage();
  await again.goto(origin + '/#/login');
  await again.locator('input[autocomplete="username"]').fill('admin');
  await again.locator('input[autocomplete="current-password"]').fill(fresh);
  await again.locator('.login-password button[type="submit"]').click();
  await expect(again).not.toHaveURL(/#\/login/);
  await expect(again.getByRole('dialog', { name: '修改密码', exact: true })).toHaveCount(0);
  await again.goto(origin + '/#/sys/VpConfig?section=access');
  const address = again.getByTestId('management-address');
  await expect(address).toBeVisible();
  await expect(address.getByRole('button', { name: '生成证书并应用管理地址' })).toBeEnabled();
  console.log(
    'PASS: first-login dialog works on desktop/tablet/phone in light/dark; forced change cannot close; failures preserve input; duplicate submit is suppressed; new password survives repeat login',
  );

  const init = spawnSync(binary, ['apikey', 'init', '-o', 'json'], {
    cwd: runtime,
    env,
    windowsHide: true,
    encoding: 'utf8',
  });
  expect(init.status).toBe(0);
  const key = fs.readFileSync(path.join(runtime, 'data/.keys/cli_api_key'), 'utf8').trim();
  await address.locator('input').first().fill('https://invalid.homewaf.test');
  await address.getByText('我已配置该域名解析，并确认切换后不再通过 IP 管理').click();
  await address.getByRole('button', { name: '生成证书并应用管理地址' }).click();
  await again.getByRole('button', { name: '生成并应用', exact: true }).click();
  await expect(address.locator('.el-alert--error')).toBeVisible();
  await expect(address.locator('input').first()).toHaveValue('https://invalid.homewaf.test');
  await address.locator('input').first().fill('console.homewaf.test');
  await address.getByText('我已配置该域名解析，并确认切换后不再通过 IP 管理').click();
  await address.getByRole('button', { name: '生成证书并应用管理地址' }).click();
  await again.getByRole('button', { name: '取消', exact: true }).last().click();
  await expect(address.locator('input').first()).toHaveValue('console.homewaf.test');
  await address.getByRole('button', { name: '生成证书并应用管理地址' }).click();
  await again.getByRole('button', { name: '生成并应用', exact: true }).click();
  await expect(address).toContainText('新 HTTPS 入口');
  await wait(async () => (await tlsRequest('console.homewaf.test')).status === 200);
  await again.evaluate(async () => {
    try {
      await window.vm.$request({ url: 'vipconfig/managementAddress', method: 'get' });
    } catch {}
  });
  await expect(address).toBeVisible();
  const rootDownload = again.waitForEvent('download');
  await address.getByRole('button', { name: '下载 HomeWaf 根证书' }).click();
  expect((await rootDownload).suggestedFilename()).toBe('homewaf-local-ca.crt');
  for (const width of [1440, 768, 390]) {
    await again.setViewportSize({ width, height: 1000 });
    await address.scrollIntoViewIfNeeded();
    await address.screenshot({ path: path.join(shots, `management-address-${width}.png`) });
  }
  expect((await fetch(origin, { redirect: 'manual' })).status).toBe(403);
  const denied = await new Promise((resolve, reject) => {
    const req = https.get(
      {
        hostname: '127.0.0.1',
        port,
        servername: 'console.homewaf.test',
        ca: fs.readFileSync(path.join(runtime, 'data/ssl/manager/ca.crt')),
        headers: { Host: `other.homewaf.test:${port}` },
      },
      (res) => {
        res.resume();
        resolve(res.statusCode);
      },
    );
    req.on('error', reject);
  });
  expect(denied).toBe(403);
  const restored = await tlsRequest(
    'console.homewaf.test',
    '/api/v1/vipconfig/managementAddress',
    'POST',
    { domain: '' },
    key,
  );
  expect(JSON.parse(restored.body).code).toBe(0);
  await wait(async () => (await tlsRequest('127.0.0.1')).status === 200);
  expect((await fetch(origin)).status).toBe(200);
  await again.goto(origin + '/#/sys/VpConfig?section=access');
  await again.reload();
  await expect(again.getByTestId('management-address').locator('input').first()).toHaveValue('');
  console.log(
    'PASS: management address UI preserves invalid drafts and cancellation, applies the domain, keeps root download available after old-origin denial, and reopens in restored IP mode',
  );
  console.log(
    'PASS: domain change persists, uses the same trusted root, serves a hostname-verified certificate, blocks IP/other names, and can recover to IP mode',
  );
} finally {
  await browser?.close();
  if (processHandle.exitCode === null) {
    processHandle.kill();
    await new Promise((resolve) => processHandle.once('exit', resolve));
  }
  try {
    const state = JSON.parse(fs.readFileSync(path.join(runtime, 'data/supervisor.state'), 'utf8'));
    for (const pid of state.pids || []) {
      if (Number.isInteger(pid) && pid > 1 && pid !== process.pid) {
        try {
          process.kill(pid);
        } catch {}
      }
    }
  } catch {}
  fs.closeSync(log);
}
