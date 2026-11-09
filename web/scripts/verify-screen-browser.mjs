import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { AesEncrypt, AesDecrypt } from '../src/utils/legacyCipher.ts';
const origin = process.env.SCREEN_TEST_ORIGIN || 'http://127.0.0.1:3001';
const dir = new URL('../../artifacts/', import.meta.url);
fs.mkdirSync(dir, { recursive: true });
const browser = await chromium.launch({ channel: 'chrome', headless: true });
let mode = 'normal',
  revoked = false,
  calls = 0;
const domesticTarget = {
  mode: 'auto',
  label: 'HomeWaf 防护节点',
  latitude: 0,
  longitude: 0,
  status: 'ready',
  has_coordinates: false,
  public_ip: '203.0.113.1',
  country: '中国',
  region: '上海',
  city: '上海市',
  source: 'ipip.net',
};
let target = { ...domesticTarget };
const shares = [];
const errors = [];
const token = 'hws_' + 'A'.repeat(43);
function snapshot(shared = false) {
  const end = Date.now(),
    empty = mode === 'empty';
  return {
    window_start: end - 86400000,
    window_end: end,
    timezone: 'UTC+08:00',
    summary: empty
      ? { visits: 0, blocked: 0, visitors: 0, attack_ips: 0 }
      : { visits: 18972, blocked: 11721, visitors: 5268, attack_ips: 326 },
    ips: empty
      ? []
      : Array.from({ length: 5 }, (_, i) => ({
          ip: shared ? '203.0.*.*' : '203.0.113.' + (20 + i),
          count: 204 - i * 30,
          country: '中国',
        })),
    regions: empty
      ? []
      : [
          { name: '中国', value: 5800 },
          { name: '美国', value: 3200 },
          { name: '德国', value: 1460 },
          { name: '巴西', value: 800 },
          { name: '澳大利亚', value: 461 },
        ],
    events: empty
      ? []
      : Array.from({ length: 8 }, (_, i) => ({
          req_uuid: 'fixture-' + i,
          src_ip: shared ? '203.0.*.*' : '203.0.113.' + (20 + i),
          country: i % 2 ? '中国' : '美国',
          province: '',
          city: '',
          host: shared ? '受保护站点' : 'demo.example.test',
          rule: shared ? '攻击已拦截' : 'SQL 注入',
          action: '阻止',
          create_time: new Date(end - i * 1000).toLocaleString('sv-SE'),
          unix_add_time: end - i * 1000,
        })),
    qps: {
      Current: empty ? 0 : 24,
      Max: empty ? 0 : 93,
      Points: empty
        ? []
        : Array.from({ length: 120 }, (_, i) => ({
            T: Math.floor(end / 1000) - 119 + i,
            V: i % 17 === 0 ? 93 : 12 + (i % 11),
          })),
    },
    target:
      mode === 'unlocated'
        ? { ...target, mode: 'auto', status: 'unavailable', has_coordinates: false }
        : mode === 'overseas'
          ? {
              ...target,
              mode: 'auto',
              status: 'ready',
              country: 'US',
              city: 'Los Angeles',
              public_ip: '203.0.113.2',
              latitude: 34.0559,
              longitude: -118.2666,
              has_coordinates: true,
            }
          : mode === 'domestic'
            ? domesticTarget
            : target,
    partial: mode === 'partial',
    issues: mode === 'partial' ? ['部分日志分区不可用'] : [],
    log_mode: 'db',
    visitor_basis: '按 IP 去重',
    share_expires_at: shared ? Date.now() + 3600000 : undefined,
  };
}
async function mock(context) {
  await context.route(/^https?:\/\/(?!127\.0\.0\.1|localhost)/, (route) => {
    errors.push('External map request: ' + new URL(route.request().url()).hostname);
    return route.abort();
  });
  await context.route('**/api/**', async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname.split('/api/v1/')[1];
    if (path.includes('handshake')) return route.fulfill({ json: { code: 1, msg: 'test legacy transport' } });
    let data = {};
    if (path === 'public/dashboard-share') {
      if (req.headers()['x-token']) throw new Error('Shared screen sent an admin token');
      if (revoked || req.headers()['x-dashboard-token'] !== token)
        return route.fulfill({ status: 401, json: { code: -1, msg: 'revoked' } });
      return route.fulfill({ json: { code: 0, data: snapshot(true) } });
    }
    if (path === 'dashboard/snapshot') {
      calls++;
      if (mode === 'error') return route.fulfill({ status: 503, body: 'fixture failure' });
      data = snapshot();
    }
    if (path === 'dashboard/target') {
      target = JSON.parse(AesDecrypt(req.postData()));
      data = target;
    }
    if (path === 'dashboard/shares' && req.method() === 'GET') data = shares;
    if (path === 'dashboard/shares' && req.method() === 'POST') {
      const body = JSON.parse(AesDecrypt(req.postData()));
      const share = {
        id: 'a'.repeat(32),
        label: body.label,
        scope: 'screen:read:redacted:all-sites',
        created_at: Date.now(),
        expires_at: Date.now() + body.hours * 3600000,
        revoked_at: 0,
      };
      shares.push(share);
      data = { share, token };
    }
    if (path === 'dashboard/shares/revoke') {
      revoked = true;
      shares[0].revoked_at = Date.now();
    }
    return route.fulfill({ json: { code: 0, data: AesEncrypt(JSON.stringify(data)) } });
  });
  await context.routeWebSocket('**/api/**', (ws) => ws.close());
}
const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
await mock(context);
const page = await context.newPage();
page.on('console', (message) => {
  if (message.type() === 'error') console.error('Browser:', message.text());
});
page.on('pageerror', (e) => errors.push(e.message));
async function shot(name) {
  await page.screenshot({ path: fileURLToPath(new URL(name, dir)), fullPage: true });
}
async function layout(width) {
  const result = await page.evaluate(() => {
    const chart = document.querySelector('.qps-panel').getBoundingClientRect();
    const ranking = document.querySelector('.ip-ranking li:last-child');
    return {
      horizontal: document.documentElement.scrollWidth > innerWidth + 1,
      vertical: document.documentElement.scrollHeight > innerHeight + 1,
      overlap: document.querySelector('.screen-main').getBoundingClientRect().bottom > chart.top - 8,
      rank:
        ranking &&
        ranking.getBoundingClientRect().bottom >
          document.querySelector('.ranking-panel .panel-foot').getBoundingClientRect().top + 1,
    };
  });
  if (result.horizontal || (width > 1100 && (result.vertical || result.overlap || result.rank)))
    throw new Error('Layout failure ' + width + ': ' + JSON.stringify(result));
}
async function readMapState(p) {
  return p.locator('.world-map').evaluate((el) => {
    const m = el.__vueParentComponent.exposed.getMap();
    return {
      center: m.getCenter().toArray(),
      zoom: m.getZoom(),
      pitch: m.getPitch(),
      bearing: m.getBearing(),
      moving: m.isMoving(),
      projection: m.getProjection().type,
      worldCopies: m.getRenderWorldCopies(),
      chinaBorder: m.getLayoutProperty('china-border', 'visibility'),
      chinaState: m.getFeatureState({ source: 'countries', id: 'CHN' }),
      countryCount: new Set(m.queryRenderedFeatures({ layers: ['country-fill'] }).map((f) => f.properties.id)).size,
      background: m.getPaintProperty('ocean', 'background-color'),
      originLabels: m.queryRenderedFeatures({ layers: ['origin-labels'] }).map((f) => f.properties.name),
    };
  });
}
async function mapSource(p, id) {
  return p
    .locator('.world-map')
    .evaluate((el, id) => el.__vueParentComponent.exposed.getMap().getSource(id).getData(), id);
}
async function settleMap(p) {
  await expect.poll(async () => (await readMapState(p)).moving).toBe(false);
}
async function verifySourcePopup(p, shared = false) {
  const canvas = p.locator('.maplibregl-canvas');
  await canvas.scrollIntoViewIfNeeded();
  await expect.poll(async () => (await readMapState(p)).originLabels).toContain('中国');
  const box = await canvas.boundingBox();
  const point = await p.locator('.world-map').evaluate(async (el) => {
    const m = el.__vueParentComponent.exposed.getMap();
    const data = await m.getSource('origins').getData();
    const feature = data.features.find((f) => f.properties.id === 'CHN');
    const position = m.project(feature.geometry.coordinates);
    return { x: position.x, y: position.y };
  });
  await p.mouse.move(box.x + point.x, box.y + point.y);
  await expect(p.locator('.source-popup-title')).toHaveText('中国');
  await expect(p.locator('.source-popup-count')).toContainText('5,800');
  await expect(p.locator('.source-popup-ip').first()).toHaveText(shared ? '203.0.*.*' : '203.0.113.21');
  await p.locator('.source-popup-ip').first().hover();
  await p.waitForTimeout(250);
  await expect(p.locator('.source-popup')).toBeVisible();
  const popupBox = await p.locator('.origin-ip-popup').boundingBox();
  expect(popupBox.x).toBeGreaterThanOrEqual(box.x - 1);
  expect(popupBox.x + popupBox.width).toBeLessThanOrEqual(box.x + box.width + 1);
  expect(popupBox.y).toBeGreaterThanOrEqual(box.y - 1);
  expect(popupBox.y + popupBox.height).toBeLessThanOrEqual(box.y + box.height + 1);
  if (shared) await expect(p.locator('.source-popup')).not.toContainText('203.0.113.21');
  if (!shared)
    await p.screenshot({
      path: fileURLToPath(
        new URL(
          p.viewportSize().width < 600
            ? 'homewaf-screen-origin-ip-popup-mobile.png'
            : 'homewaf-screen-origin-ip-popup.png',
          dir,
        ),
      ),
      fullPage: false,
    });
  await p.getByRole('button', { name: '关闭来源详情', exact: true }).click();
  // Click also pins the popup, supporting touch screens and text selection.
  await p.mouse.click(box.x + point.x, box.y + point.y);
  await p.mouse.move(box.x + 10, box.y + box.height - 15);
  await p.waitForTimeout(250);
  await expect(p.locator('.source-popup')).toBeVisible();
  await p.getByRole('button', { name: '关闭来源详情', exact: true }).click();
}
async function verifyMapControls() {
  const root = page.locator('.world-map'),
    canvas = page.locator('.maplibregl-canvas');
  const before = await readMapState(page);
  const nodeBefore = await page.locator('.node-symbol').boundingBox();
  await page.getByRole('button', { name: '放大地图', exact: true }).click();
  await expect(root).toHaveAttribute('data-zoom', '1.250');
  expect((await page.locator('.node-symbol').boundingBox()).width).toBeCloseTo(nodeBefore.width, 1);
  await page.getByRole('button', { name: '缩小地图', exact: true }).click();
  await expect(root).toHaveAttribute('data-zoom', '1.000');
  const box = await canvas.boundingBox();
  await page.mouse.move(box.x + box.width * 0.6, box.y + box.height * 0.6);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.6 + 90, box.y + box.height * 0.6 + 45, { steps: 8 });
  await page.mouse.up();
  await settleMap(page);
  const panned = await readMapState(page);
  expect(panned.center).not.toEqual(before.center);
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.getByRole('button', { name: '刷新', exact: true })).toBeEnabled();
  expect((await readMapState(page)).center).toEqual(panned.center);
  await page.getByRole('button', { name: '复位地图', exact: true }).click();
  expect((await readMapState(page)).center).toEqual(before.center);
  await page.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.55);
  await page.mouse.wheel(0, -240);
  await expect.poll(async () => Number(await root.getAttribute('data-zoom'))).toBeGreaterThan(1.2);
  await settleMap(page);
  await shot('homewaf-screen-map-manual-zoom.png');
  const zoomIn = page.getByRole('button', { name: '放大地图', exact: true });
  for (let i = 0; i < 15 && (await zoomIn.isEnabled()); i++) await zoomIn.click();
  await expect(root).toHaveAttribute('data-zoom', '6.000');
  await expect(zoomIn).toBeDisabled();
  await page.getByRole('button', { name: '复位地图', exact: true }).click();
  await canvas.focus();
  await page.keyboard.press('ArrowRight');
  await settleMap(page);
  expect((await readMapState(page)).center).not.toEqual(before.center);
  await page.keyboard.press('Home');
  expect((await readMapState(page)).center).toEqual(before.center);
  // Consecutive real drags travel several world widths without seam saturation.
  for (let i = 0; i < 7; i++) {
    const prior = await readMapState(page);
    await page.mouse.move(box.x + box.width * 0.15, box.y + box.height * 0.52);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width * 0.85, box.y + box.height * 0.52, { steps: 24 });
    await page.mouse.up();
    await settleMap(page);
    const moved = await readMapState(page);
    expect(moved.center[0]).not.toBeCloseTo(prior.center[0], 1);
    expect(moved.countryCount).toBeGreaterThan(40);
    expect(moved.pitch).toBe(0);
    expect(moved.bearing).toBe(0);
  }
  await shot('homewaf-screen-map-horizontal-wrap.png');
  await page.getByRole('button', { name: '复位地图', exact: true }).click();
  // Click a real rendered country polygon, not a synthetic DOM marker.
  const point = await root.evaluate((el) => {
    const p = el.__vueParentComponent.exposed.getMap().project([104, 37]);
    return [p.x, p.y];
  });
  await page.mouse.click(box.x + point[0], box.y + point[1]);
  await expect(page.locator('.map-callout strong')).toHaveText('中国');
}
async function verifyTouchMap() {
  const canvas = page.locator('.maplibregl-canvas'),
    root = page.locator('.world-map');
  await canvas.scrollIntoViewIfNeeded();
  const box = await canvas.boundingBox(),
    x = box.x + box.width * 0.5,
    y = box.y + box.height * 0.55;
  const cdp = await context.newCDPSession(page);
  await cdp.send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 2 });
  try {
    await cdp.send('Input.dispatchTouchEvent', {
      type: 'touchStart',
      touchPoints: [
        { x: x - 25, y, id: 0 },
        { x: x + 25, y, id: 1 },
      ],
    });
    for (let d = 30; d <= 65; d += 5)
      await cdp.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [
          { x: x - d, y, id: 0 },
          { x: x + d, y, id: 1 },
        ],
      });
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
    await expect.poll(async () => Number(await root.getAttribute('data-zoom'))).toBeGreaterThan(1.5);
    await settleMap(page);
    const before = await readMapState(page);
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y, id: 0 }] });
    for (let i = 1; i <= 5; i++)
      await cdp.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: x + i * 7, y: y + i * 4, id: 0 }],
      });
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchCancel', touchPoints: [] });
    await settleMap(page);
    expect((await readMapState(page)).center).not.toEqual(before.center);
    await shot('homewaf-screen-map-touch-zoom.png');
    await page.getByRole('button', { name: '复位地图', exact: true }).click();
    await expect(root).toHaveAttribute('data-zoom', '1.000');
  } finally {
    await cdp.send('Emulation.setTouchEmulationEnabled', { enabled: false });
    await cdp.detach();
  }
}

try {
  await page.goto(origin + '/#/dashboard/screen');
  await expect(page).toHaveURL(/login/);
  await page.addInitScript(
    ({ origin }) => {
      if (location.origin === origin) localStorage.setItem('access_token', 'isolated-screen-test');
    },
    { origin },
  );
  await page.goto('about:blank');
  await page.goto(origin + '/#/dashboard/screen');
  await expect(page.getByText('数据已连接', { exact: false })).toBeVisible();
  await expect(page.locator('.metric-copy strong').nth(2)).toHaveText('5,268');
  await expect(page.locator('.world-map')).toHaveAttribute('data-ready', 'true');
  await expect.poll(async () => (await mapSource(page, 'flights')).features.length).toBeGreaterThan(0);
  await expect(page.locator('.protection-node')).toHaveCount(1);
  await expect(page.locator('.protection-node')).toContainText('上海市 · 城市示意位置');
  expect(Number(await page.locator('.protection-node').getAttribute('data-longitude'))).toBeCloseTo(121.4346, 1);
  await expect(page.locator('.unblocked-summary strong')).toHaveText('7,251次');
  await expect(page.getByRole('button', { name: '功能接入清单', exact: false })).toHaveCount(0);
  await expect(page.locator('.capability-dialog')).toHaveCount(0);
  await expect(page.getByText('攻击事件飞线', { exact: false })).toHaveCount(0);
  const mapState = await readMapState(page);
  expect(mapState.projection).toBe('mercator');
  expect(mapState.chinaBorder).toBe('visible');
  expect(mapState.countryCount).toBeGreaterThan(100);
  expect(mapState.chinaState.count).toBe(5800);
  expect(mapState.worldCopies).toBe(true);
  expect(mapState.pitch).toBe(0);
  expect(mapState.bearing).toBe(0);
  await expect(page.locator('.world-map')).toHaveAttribute('data-zoom', '1.000');
  await expect(page.locator('.land-dots,.world-tile,svg.map-canvas')).toHaveCount(0);
  await layout(1920);
  await shot('homewaf-screen-map-default.png');
  await verifySourcePopup(page);
  await verifyMapControls();
  await page.getByRole('button', { name: '切换深色主题' }).click();
  await expect.poll(async () => (await readMapState(page)).background).toBe('#0b1220');
  await page.waitForTimeout(100);
  await shot('homewaf-screen-map-dark.png');
  await page.getByRole('button', { name: '切换浅色主题' }).click();
  await page.getByRole('button', { name: '防护节点', exact: true }).click();
  await page.getByLabel('节点名称').fill('本地防护中心');
  await page.getByLabel('显示方式').selectOption('coordinates');
  await page.getByLabel('经度', { exact: true }).fill('121.47');
  await page.getByLabel('纬度', { exact: true }).fill('31.23');
  await page.getByRole('button', { name: '保存节点' }).click();
  await expect(page.locator('.protection-node')).toContainText('本地防护中心');
  expect(Number(await page.locator('.protection-node').getAttribute('data-longitude'))).toBeCloseTo(121.47, 2);
  expect(Number(await page.locator('.protection-node').getAttribute('data-latitude'))).toBeCloseTo(31.23, 2);
  await page.getByRole('button', { name: '切换深色主题' }).click();
  await expect(page.locator('.security-screen')).toHaveClass(/is-dark/);
  await shot('homewaf-screen-world-dark.png');
  await page.getByRole('button', { name: '全屏', exact: true }).click();
  await expect.poll(() => page.evaluate(() => Boolean(document.fullscreenElement))).toBe(true);
  await page.getByRole('button', { name: '退出全屏' }).click();
  await page.getByRole('button', { name: '暂停自动刷新' }).click();
  const before = calls;
  await page.waitForTimeout(5400);
  if (calls !== before) throw new Error('Polling while paused');
  await page.getByRole('button', { name: '恢复自动刷新' }).click();
  await expect.poll(() => calls).toBeGreaterThan(before);
  await page.getByRole('button', { name: '只读分享', exact: true }).click();
  await page.getByRole('button', { name: '创建只读链接' }).click();
  const link = page.locator('.new-share input');
  await expect(link).toBeVisible();
  const shareURL = await link.inputValue();
  const guest = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await mock(guest);
  const shared = await guest.newPage();
  shared.on('pageerror', (e) => errors.push(e.message));
  await shared.goto(shareURL);
  await expect(shared.getByText('数据已连接', { exact: false })).toBeVisible();
  await expect(shared.getByRole('button', { name: '只读分享', exact: true })).toHaveCount(0);
  await expect(shared.getByRole('link', { name: '返回控制台', exact: false })).toHaveCount(0);
  await expect(shared.locator('.event-line .mono').first()).toHaveText('203.0.*.*');
  await expect(shared.locator('.world-map')).toHaveAttribute('data-ready', 'true');
  await expect.poll(async () => (await readMapState(shared)).chinaBorder).toBe('none');
  await expect(shared.locator('.world-map')).toHaveAttribute('data-zoom', '1.000');
  await verifySourcePopup(shared, true);
  await shared.getByRole('button', { name: '放大地图', exact: true }).click();
  await expect(shared.locator('.world-map')).toHaveAttribute('data-zoom', '1.250');
  await expect(shared.locator('.unblocked-summary strong')).toHaveText('7,251次');
  await expect(shared.locator('.capability-dialog')).toHaveCount(0);
  await page.getByRole('button', { name: '撤销', exact: true }).click();
  await shared.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(shared.getByRole('alert')).toContainText('分享链接无效');
  await expect(shared.locator('.metric-copy strong').first()).toHaveText('—');
  await guest.unrouteAll({ behavior: 'ignoreErrors' });
  await guest.close();
  await page.keyboard.press('Escape');
  for (const size of [
    { width: 1366, height: 768 },
    { width: 1600, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(size);
    await page.waitForTimeout(200);
    await layout(size.width);
    if (size.width === 390) await shot('homewaf-screen-world-mobile.png');
  }
  await page.setViewportSize({ width: 1920, height: 1080 });
  mode = 'overseas';
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.locator('.world-map')).toHaveAttribute('data-map-view', 'world');
  await expect.poll(async () => (await readMapState(page)).chinaBorder).toBe('none');
  await expect(page.locator('.protection-node')).toContainText('Los Angeles');
  await shot('homewaf-screen-overseas-flat.png');
  mode = 'domestic';
  await expect(page.getByRole('button', { name: '刷新', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.locator('.world-map')).toHaveAttribute('data-map-view', 'world');
  await expect.poll(async () => (await readMapState(page)).chinaBorder).toBe('visible');
  await expect(page.locator('.world-map')).toHaveAttribute('data-zoom', '1.000');
  for (const size of [
    { width: 1366, height: 768 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(size);
    await page.waitForTimeout(200);
    await layout(size.width);
    if (size.width === 390) {
      await verifyTouchMap();
      await verifySourcePopup(page);
      await shot('homewaf-screen-map-mobile.png');
    }
  }
  await page.setViewportSize({ width: 1920, height: 1080 });
  mode = 'unlocated';
  await expect(page.getByRole('button', { name: '刷新', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.locator('.node-location-status')).toContainText('定位待确认');
  await expect(page.locator('.protection-node')).toHaveCount(0);
  await expect.poll(async () => (await mapSource(page, 'flights')).features.length).toBe(0);
  await expect(page.locator('.metric-copy strong').first()).toHaveText('18,972');
  mode = 'partial';
  await expect(page.getByRole('button', { name: '刷新', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.getByText('统计不完整：', { exact: false })).toBeVisible();
  mode = 'empty';
  await expect(page.getByRole('button', { name: '刷新', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(page.getByText('近 24 小时暂无攻击 IP', { exact: false })).toBeVisible();
  await expect(page.locator('.metric-copy strong').first()).toHaveText('0');
  await expect(page.locator('.unblocked-summary strong')).toHaveText('0次');
  await expect.poll(async () => (await readMapState(page)).chinaBorder).toBe('none');
  mode = 'error';
  await page.reload();
  await expect(page.locator('.metric-stack .source-status')).toHaveText('连接异常 · 暂无数据');
  await expect(page.locator('.metric-copy strong').first()).toHaveText('—');
  await expect(page.locator('.unblocked-summary strong')).toHaveText('—次');
  if (errors.length) throw new Error(errors.join('\n'));
  console.log(
    'PASS: MapLibre flat countries and borders, real count colors, no grid/remote maps, horizontal wrapping, pan/zoom/reset, touch gestures, stable refresh, sharing and desktop/mobile layouts.',
  );
} catch (error) {
  await shot('homewaf-screen-maplibre-failure.png');
  console.error((await page.locator('body').innerText()).slice(-3000));
  throw error;
} finally {
  await context.unrouteAll({ behavior: 'ignoreErrors' });
  await browser.close();
}
