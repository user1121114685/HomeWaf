import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { trafficLocation, trafficRegions } from '../src/pages/dashboard/base/components/trafficLocation.ts';
import { locateCity } from '../src/pages/dashboard/screen/cityLocation.ts';
import { validateStyleMin } from '@maplibre/maplibre-gl-style-spec';
import {
  aggregateRegions,
  countryCoordinate,
  countryId,
  createMapStyle,
  flightCoordinates,
  flightProgress,
  flightTrack,
  flightPosition,
  sourceIPDetails,
} from '../src/pages/dashboard/screen/mapData.ts';
import {
  blockRate,
  eventLocation,
  formatCount,
  validatePayload,
  validateSnapshot,
} from '../src/pages/dashboard/screen/data.ts';

test('domestic provider Chongqing labels resolve to the local Chongqing map position', () => {
  const point=locateCity('中国','重庆','重庆');
  assert.ok(point && point.longitude>105 && point.longitude<108 && point.latitude>28 && point.latitude<31);
  assert.deepEqual(locateCity('中国','重庆市','重庆市'),point);
});
test('dashboard locations preserve country and city without merging distinct cities', () => {
  assert.equal(trafficLocation({ country: '中国', province: '重庆市', city: '重庆市' }), '中国 · 重庆市');
  assert.equal(trafficLocation({ country: 'United States', province: 'Florida', city: 'Miami' }), '美国 · Florida · Miami');
  assert.equal(trafficLocation({ country: '中国', province: '浙江省', city: '0' }), '中国 · 浙江省 · 城市未知');
  const regions = trafficRegions([
    { name: '中国', city: '重庆市', value: 10 },
    { name: 'China', city: '重庆市', value: 2 },
    { name: '中国', city: '杭州市', value: 4 },
    { name: 'United States', city: 'Miami', value: 3 },
    { name: '内网', city: '', value: 100 },
  ]);
  assert.deepEqual(regions, [
    { name: '中国 · 重庆市', value: 12 },
    { name: '中国 · 杭州市', value: 4 },
    { name: '美国 · Miami', value: 3 },
  ]);
});
test('missing metrics stay unknown while valid zero stays zero', () => {
  for (const value of [undefined, null, '', '0', NaN, Infinity, -1]) assert.equal(formatCount(value), '—');
  assert.equal(formatCount(0), '0');
  assert.equal(formatCount(123456), '123,456');
  assert.equal(validatePayload('summary', {}), false);
  assert.equal(validatePayload('summary', { visits: 0, blocked: 0, visitors: 0, attack_ips: 0 }), true);
});
test('block percentage uses the rolling window counts', () => {
  assert.equal(blockRate(null), '—');
  assert.equal(blockRate({ visits: 0, blocked: 0, visitors: 0, attack_ips: 0 }), '—');
  assert.equal(blockRate({ visits: 80, blocked: 20, visitors: 10, attack_ips: 3 }), '25.0%');
});
test('snapshot accepts exactly 24 hours and requires consistent numeric fields', () => {
  const snapshot = {
    window_start: 1000,
    window_end: 86401000,
    summary: { visits: 1, blocked: 0, visitors: 1, attack_ips: 0 },
    ips: [],
    regions: [],
    events: [],
    qps: { Current: 0, Max: 0, Points: [] },
    target: { mode: 'logical', label: 'HomeWaf', latitude: 0, longitude: 0 },
    issues: [],
  };
  assert.equal(validateSnapshot(snapshot), true);
  assert.equal(validateSnapshot({ ...snapshot, window_end: 1001 }), false);
  assert.equal(validateSnapshot({ ...snapshot, summary: { ...snapshot.summary, visitors: '1' } }), false);
  assert.equal(validateSnapshot({ ...snapshot, target: { ...snapshot.target, latitude: 91 } }), false);
});
test('unknown geography is preserved and duplicate place names are removed', () => {
  assert.equal(eventLocation({ country: '0', province: '未知', city: '' }), '归属地未知');
  assert.equal(eventLocation({ country: '中国', province: '北京', city: '北京' }), '中国 · 北京');
});

test('country aliases aggregate real counts without fabricating unknown regions', () => {
  const result = aggregateRegions([
    { name: '中国', value: 10 },
    { name: 'CN', value: 5 },
    { name: '美国', value: 7 },
    { name: '未知', value: 8 },
    { name: 'US', value: 0 },
  ]);
  assert.equal(result.unmapped, 1);
  assert.equal(result.mapped.length, 2);
  assert.equal(result.mapped[0].id, 'CHN');
  assert.equal(result.mapped[0].value, 15);
  assert.equal(countryId(' mainLAND CHINA '), 'CHN');
  assert.equal(countryCoordinate('未知'), null);
  assert.ok(countryCoordinate('中国')[0] > 70);
});
test('map style is valid, flat, local and contains countries with borders only', () => {
  for (const dark of [false, true]) {
    const style = createMapStyle('/assets/world-countries.geojson', dark);
    assert.deepEqual(validateStyleMin(style), []);
    assert.equal(style.projection.type, 'mercator');
    assert.equal(style.glyphs, undefined);
    assert.equal(style.sprite, undefined);
    assert.equal(style.terrain, undefined);
    assert.ok(style.layers.some((l) => l.id === 'country-fill' && l.type === 'fill'));
    assert.ok(style.layers.some((l) => l.id === 'country-borders' && l.type === 'line'));
    assert.ok(!JSON.stringify(style).match(/graticule|satellite|https?:|raster|extrusion/));
    assert.equal(style.layers.find((l) => l.id === 'china-border').paint['line-color'], '#df4148');
    const labels = style.layers.find((l) => l.id === 'origin-labels');
    assert.equal(labels.type, 'symbol');
    assert.equal(labels.source, 'origins');
    assert.deepEqual(labels.layout['text-field'], ['get', 'name']);
    assert.equal(labels.layout['text-allow-overlap'], false);
  }
});
test('origin popup only shows matching recent IPs and preserves shared redaction', () => {
  const events = [
    { country: 'US', src_ip: '203.0.113.1', city: 'Los Angeles' },
    { country: '中国', src_ip: '203.0.113.2', province: '上海', city: '上海' },
    { country: 'CN', src_ip: '203.0.113.2', city: '上海' },
    { country: 'CN', src_ip: '2001:db8::2', city: '北京' },
  ];
  assert.deepEqual(
    sourceIPDetails('CHN', events).map((e) => e.ip),
    ['203.0.113.2', '2001:db8::2'],
  );
  assert.equal(sourceIPDetails('CHN', events)[0].location, '中国 · 上海');
  assert.deepEqual(sourceIPDetails('DEU', events), []);
  assert.equal(sourceIPDetails('CHN', [{ country: 'CN', src_ip: '203.0.*.*' }])[0].ip, '203.0.*.*');
  assert.equal(
    sourceIPDetails(
      'CHN',
      Array.from({ length: 20 }, (_, i) => ({ country: 'CN', src_ip: '203.0.113.' + i })),
    ).length,
    8,
  );
});
test('local country assets retain closed geographic polygons and stable feature IDs', () => {
  const geo = JSON.parse(
    fs.readFileSync(new URL('../src/pages/dashboard/screen/world-countries.geojson', import.meta.url), 'utf8'),
  );
  assert.ok(geo.features.length >= 170);
  assert.equal(new Set(geo.features.map((f) => f.id)).size, geo.features.length);
  for (const feature of geo.features) {
    const polygons =
      feature.geometry.type === 'Polygon' ? [feature.geometry.coordinates] : feature.geometry.coordinates;
    for (const polygon of polygons)
      for (const ring of polygon) {
        assert.ok(ring.length >= 4);
        assert.deepEqual(ring.at(-1), ring[0]);
        for (const [lon, lat] of ring)
          assert.ok(Number.isFinite(lon) && Number.isFinite(lat) && Math.abs(lon) <= 180 && Math.abs(lat) <= 90);
      }
  }
  assert.ok(geo.features.find((f) => f.id === 'CHN'));
});
test('flights across the dateline follow the short route and retain endpoints', () => {
  for (const [source, target] of [
    [
      [179, 30],
      [-179, 35],
    ],
    [
      [-170, 25],
      [170, 30],
    ],
    [
      [106, 32],
      [121, 31],
    ],
  ]) {
    const path = flightCoordinates(source, target);
    assert.equal(path.length, 49);
    assert.ok(Math.abs(path[0][1] - source[1]) < 1e-8);
    assert.ok(Math.abs(path.at(-1)[0] - target[0]) < 1e-8);
    assert.ok(Math.abs(path.at(-1)[1] - target[1]) < 1e-8);
    for (let i = 1; i < path.length; i++) assert.ok(Math.abs(path[i][0] - path[i - 1][0]) < 10);
  }
});

test('attack flight accelerates through 75 percent of distance then decelerates without a speed jump', () => {
  assert.equal(flightProgress(0),0);
  assert.equal(flightProgress(0.75),0.75);
  assert.equal(flightProgress(1),1);
  const velocity=t=>(flightProgress(t+0.00001)-flightProgress(t-0.00001))/0.00002;
  for(const t of [0.1,0.2,0.3,0.4,0.5,0.6]) assert.ok(velocity(t+0.05)>velocity(t));
  for(const t of [0.8,0.85,0.9]) assert.ok(velocity(t+0.025)<velocity(t));
  assert.ok(Math.abs(velocity(0.74999)-velocity(0.75001))<0.00001);
  assert.ok(velocity(0)<0.00001 && velocity(1)<0.00001);
  const track=flightTrack(flightCoordinates([179,30],[-179,35]));
  const start=flightPosition(track,0),end=flightPosition(track,1);
  assert.ok(Math.abs(start[1]-30)<1e-8 && Math.abs(end[0]+179)<1e-8 && Math.abs(end[1]-35)<1e-8);
  assert.ok(track.points.every((point,i)=>i===0 || Math.abs(point[0]-track.points[i-1][0])<0.05));
});
