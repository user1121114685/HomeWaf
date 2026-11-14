<template>
  <div
    class="world-map"
    data-map-view="world"
    data-map-engine="maplibre"
    :data-ready="ready"
    :data-zoom="scale.toFixed(3)"
  >
    <div
      ref="container"
      class="map-canvas"
      aria-label="平面国家地图：拖动平移，滚轮或双指缩放，方向键移动，Home 复位"
    />
    <div class="map-toolbar" role="group" aria-label="地图视角控制">
      <button
        type="button"
        aria-label="缩小地图"
        title="缩小地图"
        :disabled="!ready || scale <= 1.001"
        @click="zoomBy(-1)"
      >
        −
      </button>
      <output aria-label="地图缩放比例">{{ Math.round(scale * 100) }}%</output>
      <button
        type="button"
        aria-label="放大地图"
        title="放大地图"
        :disabled="!ready || scale >= MAX_SCALE - 0.001"
        @click="zoomBy(1)"
      >
        +
      </button>
      <button
        type="button"
        class="map-reset"
        aria-label="复位地图"
        title="恢复默认世界视图"
        :disabled="!ready"
        @click="resetView"
      >
        复位
      </button>
    </div>
    <div v-if="mapError" class="map-error" role="alert">
      {{ mapError }}<button type="button" @click="initializeMap">重试地图</button>
    </div>
    <div v-else-if="!ready" class="map-loading" role="status">正在加载地图…</div>
    <div v-if="!targetPoint && !mapError" class="node-location-status" role="status">{{ nodeLocationStatus }}</div>
    <div v-if="active" class="map-callout">
      <span class="map-signal"></span><strong>{{ active.name }}</strong
      ><span>{{ formatCount(active.value) }} <small>次拦截</small></span>
    </div>
    <div v-else class="map-callout map-empty">{{ status || '近 24 小时暂无可定位的拦截来源' }}</div>
    <div class="map-caption">
      <span class="map-legend"
        ><i class="neutral-swatch"></i>国家 / 地区 <span>拦截少</span><i class="attack-scale"></i><span>多</span></span
      >
      <span v-if="highlightChina" class="focus-caption">红线标记中国边界</span>
      <span v-if="regionData.unmapped">{{ regionData.unmapped }} 个来源地区未定位</span>
      <span class="map-help">拖动平移 · 滚轮 / 双指缩放</span>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type PropType } from 'vue';
import { Map as LibreMap, Marker, Popup, setWorkerUrl, type GeoJSONSource, type MapMouseEvent } from 'maplibre-gl';
import mapWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
import 'maplibre-gl/dist/maplibre-gl.css';
import countriesUrl from './world-countries.geojson?url';
import { locateCity } from './cityLocation';
import {
  aggregateRegions,
  countryCoordinate,
  countryFill,
  countryId,
  countryLocations,
  createMapStyle,
  DEFAULT_CENTER,
  emptyCollection,
  flightCoordinates,
  lineCollection,
  MAX_SCALE,
  pointCollection,
  sourceIPDetails,
  type Coordinate,
} from './mapData';
import { formatCount, type Region, type SecurityEvent, type ScreenTarget } from './data';

// Bundle the worker and its imports locally for both Vite and embedded builds.
setWorkerUrl(mapWorkerUrl);

const props = defineProps({
  regions: { type: Array as PropType<Region[]>, required: true },
  status: { type: String, default: '' },
  events: { type: Array as PropType<SecurityEvent[]>, default: () => [] },
  target: { type: Object as PropType<ScreenTarget | null>, default: null },
  serverTime: { type: Number, default: 0 },
  paused: Boolean,
  dark: Boolean,
});
const container = ref<HTMLDivElement>();
const ready = ref(false),
  scale = ref(1),
  selected = ref(''),
  mapError = ref('');
let map: LibreMap | undefined, marker: Marker | undefined, observer: ResizeObserver | undefined;
let sourcePopup: Popup | undefined;
let popupSource = '',
  popupPinned = false;
let popupCloseTimer: ReturnType<typeof setTimeout> | undefined;
let baseZoom = 0,
  touched = false,
  frame = 0,
  updatingHeads = false;
let timer: ReturnType<typeof setInterval> | undefined;
const reducedMotion = ref(false);
let media: MediaQueryList | undefined;
const regionData = computed(() => aggregateRegions(props.regions));
const active = computed(() => {
  const selectedRegion = countryLocations[selected.value];
  return selectedRegion && props.regions.length
    ? { name: selectedRegion.name, value: regionData.value.mapped.find((p) => p.id === selected.value)?.value || 0 }
    : regionData.value.mapped[0];
});
const highlightChina = computed(
  () =>
    props.target?.status === 'ready' &&
    Boolean(props.target.public_ip) &&
    countryId(props.target.country || '') === 'CHN',
);
const targetPoint = computed(() => {
  const target = props.target;
  if (!target) return null;
  const manual = target.mode === 'coordinates';
  if (!manual && target.status !== 'ready') return null;
  const location =
    manual || target.has_coordinates ? target : locateCity(target.country || '', target.city || '', target.region);
  if (!location || !Number.isFinite(location.longitude) || !Number.isFinite(location.latitude)) return null;
  return {
    coordinates: [location.longitude, location.latitude] as Coordinate,
    label: target.label,
    detail: manual
      ? '自定义部署坐标'
      : `${target.city || target.country} · ${target.has_coordinates ? '公网 IP 近似定位' : '城市示意位置'}`,
  };
});
const nodeLocationStatus = computed(() => {
  if (!props.target || props.target.status === 'locating') return '防护节点 · 正在获取公网 IP 与城市…';
  return props.target.status === 'ready'
    ? `防护节点 · ${props.target.city || '城市未知'}，地图点位未匹配`
    : '防护节点 · 定位待确认';
});

interface Flight {
  id: string;
  country: string;
  started: number;
  until: number;
}
const flights = ref<Flight[]>([]),
  seen = new Set<string>();
const paths = computed(() =>
  targetPoint.value
    ? flights.value.flatMap((f) => {
        const source = countryCoordinate(f.country);
        return source ? [{ ...f, coordinates: flightCoordinates(source, targetPoint.value!.coordinates) }] : [];
      })
    : [],
);
watch(
  () => props.events,
  (events) => {
    if (!events.length) {
      flights.value = [];
      return;
    }
    for (const event of [...events].reverse()) {
      if (seen.has(event.req_uuid)) continue;
      seen.add(event.req_uuid);
      if (event.unix_add_time < props.serverTime - 15000 || !countryCoordinate(event.country || '')) continue;
      const now = Date.now();
      flights.value.push({ id: event.req_uuid, country: event.country || '', started: now, until: now + 7000 });
    }
    flights.value = flights.value.slice(-16);
    if (seen.size > 512) for (const id of [...seen].slice(0, 256)) seen.delete(id);
  },
  { immediate: true },
);

function source(id: string) {
  return map?.getSource<GeoJSONSource>(id);
}
function setData(id: string, data: ReturnType<typeof emptyCollection>) {
  // MapLibre 6 loads GeoJSON asynchronously. A removed map must not leave an
  // unhandled rejection when navigating away while a worker is updating.
  const current = map;
  void source(id)
    ?.setData(data)
    .catch(() => {
      if (current === map && ready.value) mapError.value = '地图数据加载失败，请重试。';
    });
}
function updateRegions() {
  if (!ready.value || !map) return;
  map.removeFeatureState({ source: 'countries' });
  const largest = Math.max(1, ...regionData.value.mapped.map((r) => r.value));
  for (const region of regionData.value.mapped)
    map.setFeatureState(
      { source: 'countries', id: region.id },
      { count: region.value, intensity: Math.sqrt(region.value / largest) },
    );
  setData(
    'origins',
    pointCollection(
      regionData.value.mapped.map((r) => ({
        coordinates: r.coordinates as Coordinate,
        properties: { id: r.id, name: r.name, count: r.value },
      })),
    ),
  );
}
function updateChina() {
  if (ready.value) map?.setLayoutProperty('china-border', 'visibility', highlightChina.value ? 'visible' : 'none');
}
function updateTarget() {
  if (!ready.value || !map) return;
  const target = targetPoint.value;
  if (!target) {
    marker?.remove();
    marker = undefined;
    return;
  }
  if (!marker) {
    const el = document.createElement('div');
    el.className = 'protection-node';
    const symbol = document.createElement('span');
    symbol.className = 'node-symbol';
    symbol.textContent = '✓';
    const label = document.createElement('span');
    label.className = 'node-copy';
    label.append(document.createElement('strong'), document.createElement('small'));
    el.append(symbol, label);
    marker = new Marker({ element: el, anchor: 'center', subpixelPositioning: true })
      .setLngLat(target.coordinates)
      .addTo(map);
  }
  marker.setLngLat(target.coordinates);
  const el = marker.getElement();
  el.querySelector('strong')!.textContent = target.label;
  el.querySelector('small')!.textContent = target.detail;
  el.title = `${target.label} · ${target.detail} · ${props.target?.public_ip || '管理员配置的部署坐标'}`;
  el.dataset.longitude = String(target.coordinates[0]);
  el.dataset.latitude = String(target.coordinates[1]);
  updateCamera();
}
function updateCamera() {
  if (!map) return;
  scale.value = Math.max(1, Math.min(MAX_SCALE, 2 ** (map.getZoom() - baseZoom)));
  if (marker)
    marker
      .getElement()
      .classList.toggle('label-left', map.project(marker.getLngLat()).x > map.getContainer().clientWidth * 0.7);
}
function fitDefault() {
  if (!map || !container.value) return;
  // Fit one full longitude span on desktop, and useful country shapes on mobile.
  baseZoom = Math.max(0, Math.log2(Math.max(container.value.clientWidth, container.value.clientHeight) / 512));
  map.setMinZoom(baseZoom);
  map.setMaxZoom(baseZoom + Math.log2(MAX_SCALE));
  map.jumpTo({ center: DEFAULT_CENTER, zoom: baseZoom, bearing: 0, pitch: 0 });
  updateCamera();
}
function resetView() {
  touched = false;
  map?.stop();
  fitDefault();
}
function zoomBy(direction: number) {
  if (!map) return;
  touched = true;
  map.jumpTo({
    zoom: Math.max(baseZoom, Math.min(baseZoom + Math.log2(MAX_SCALE), map.getZoom() + direction * Math.log2(1.25))),
  });
}
function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') closeSourcePopup();
  if (event.key === 'Home' || event.key === '0') {
    event.preventDefault();
    resetView();
  }
}
function updateTheme() {
  if (!map || !ready.value) return;
  map.setPaintProperty('ocean', 'background-color', props.dark ? '#0b1220' : '#f0f2fa');
  map.setPaintProperty('country-fill', 'fill-color', countryFill(props.dark));
  map.setPaintProperty('country-borders', 'line-color', props.dark ? '#73859b' : '#839caf');
  map.setPaintProperty('origin-labels', 'text-color', props.dark ? '#f1e2d1' : '#563b2d');
  map.setPaintProperty('origin-labels', 'text-halo-color', props.dark ? '#111c2c' : '#fff8ef');
}
function closeSourcePopup() {
  clearTimeout(popupCloseTimer);
  const popup = sourcePopup;
  sourcePopup = undefined;
  popupSource = '';
  popupPinned = false;
  popup?.remove();
}
function schedulePopupClose() {
  if (popupPinned) return;
  clearTimeout(popupCloseTimer);
  popupCloseTimer = setTimeout(closeSourcePopup, 180);
}
function popupContent(id: string) {
  const region = regionData.value.mapped.find((entry) => entry.id === id);
  if (!region) return null;
  const root = document.createElement('section');
  root.className = 'source-popup';
  root.setAttribute('aria-label', `${region.name} 来源 IP`);
  const title = document.createElement('strong');
  title.className = 'source-popup-title';
  title.textContent = region.name;
  const count = document.createElement('p');
  count.className = 'source-popup-count';
  count.textContent = `近 24 小时拦截 ${formatCount(region.value)} 次`;
  const hint = document.createElement('p');
  hint.className = 'source-popup-hint';
  hint.textContent = '国家／地区示意位置 · 最近事件来源 IP';
  root.append(title, count, hint);
  const ips = sourceIPDetails(id, props.events);
  if (!ips.length) {
    const empty = document.createElement('p');
    empty.className = 'source-popup-empty';
    empty.textContent = '最近事件中暂无该地区 IP';
    root.append(empty);
  } else {
    const list = document.createElement('ul');
    for (const entry of ips) {
      const item = document.createElement('li');
      const ip = document.createElement('span');
      ip.className = 'source-popup-ip';
      ip.textContent = entry.ip;
      const location = document.createElement('small');
      location.textContent = entry.location;
      item.append(ip, location);
      list.append(item);
    }
    root.append(list);
    const foot = document.createElement('p');
    foot.className = 'source-popup-hint';
    foot.textContent = '展示最近事件中的 IP，最多 8 个';
    root.append(foot);
  }
  return root;
}
function refreshSourcePopup() {
  if (!sourcePopup || !popupSource) return;
  const content = popupContent(popupSource);
  if (content) sourcePopup.setDOMContent(content);
  else closeSourcePopup();
}
function showSourcePopup(id: string, coordinate: Coordinate, pinned = false) {
  if (!map || !ready.value) return;
  clearTimeout(popupCloseTimer);
  if (sourcePopup && popupSource === id) {
    popupPinned = popupPinned || pinned;
    return;
  }
  const content = popupContent(id);
  if (!content) return;
  closeSourcePopup();
  popupSource = id;
  popupPinned = pinned;
  const compact = map.getContainer().clientHeight < 400 || map.getContainer().clientWidth < 600;
  const popup = new Popup({
    closeButton: true,
    closeOnClick: false,
    focusAfterOpen: false,
    maxWidth: '280px',
    offset: compact ? 0 : 15,
    anchor: compact ? 'center' : undefined,
    className: 'origin-ip-popup',
  })
    .setLngLat(compact ? map.getCenter() : coordinate)
    .setDOMContent(content)
    .addTo(map);
  sourcePopup = popup;
  popup.getElement().querySelector('button')?.setAttribute('aria-label', '关闭来源详情');
  popup.getElement().addEventListener('mouseenter', () => clearTimeout(popupCloseTimer));
  popup.getElement().addEventListener('pointerdown', () => {
    popupPinned = true;
    clearTimeout(popupCloseTimer);
  });
  popup.getElement().addEventListener('mouseleave', schedulePopupClose);
  popup.on('close', () => {
    if (sourcePopup === popup) {
      sourcePopup = undefined;
      popupSource = '';
      popupPinned = false;
    }
  });
}
function interactWithSource(event: MapMouseEvent, pinned = false) {
  if (!map || !ready.value) return;
  const feature = map.queryRenderedFeatures(event.point, {
    layers: ['origin-labels', 'origin-points', 'origin-halo'],
  })[0];
  const id = feature?.properties?.id;
  const country = countryLocations[String(id)];
  if (!country) {
    if (pinned) closeSourcePopup();
    else schedulePopupClose();
    return;
  }
  if (!pinned && popupPinned) return;
  const coordinate = [...country.coordinates] as Coordinate;
  coordinate[0] += Math.round((event.lngLat.lng - coordinate[0]) / 360) * 360;
  showSourcePopup(String(id), coordinate, pinned);
  if (pinned) selected.value = String(id);
}
function stopAnimation() {
  cancelAnimationFrame(frame);
  frame = 0;
}
function animateFlights() {
  stopAnimation();
  if (!ready.value) return;
  if (props.paused || reducedMotion.value || !paths.value.length || document.hidden) {
    setData('flight-heads', emptyCollection());
    return;
  }
  let last = 0;
  const tick = (time: number) => {
    if (!map || !ready.value) return;
    if (time - last >= 40 && !updatingHeads) {
      last = time;
      const now = Date.now();
      const points = paths.value.map((flight) => {
        const progress = (((now - flight.started) % 2400) / 2400) * (flight.coordinates.length - 1);
        const i = Math.floor(progress),
          t = progress - i;
        const a = flight.coordinates[i],
          b = flight.coordinates[Math.min(i + 1, flight.coordinates.length - 1)];
        return { coordinates: [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t] as Coordinate };
      });
      const heads = source('flight-heads');
      if (heads) {
        updatingHeads = true;
        void heads
          .setData(pointCollection(points))
          .catch(() => {})
          .finally(() => {
            updatingHeads = false;
          });
      }
    }
    frame = requestAnimationFrame(tick);
  };
  frame = requestAnimationFrame(tick);
}
function updateFlights() {
  if (!ready.value) return;
  setData('flights', lineCollection(paths.value.map((f) => f.coordinates)));
  animateFlights();
}
function removeMap() {
  closeSourcePopup();
  stopAnimation();
  ready.value = false;
  observer?.disconnect();
  observer = undefined;
  marker?.remove();
  marker = undefined;
  map?.getCanvas().removeEventListener('keydown', onKey);
  map?.remove();
  map = undefined;
}
function initializeMap() {
  if (!container.value) return;
  removeMap();
  mapError.value = '';
  touched = false;
  try {
    const instance = new LibreMap({
      container: container.value,
      style: createMapStyle(countriesUrl, props.dark),
      center: DEFAULT_CENTER,
      zoom: 0,
      minZoom: 0,
      maxZoom: 5,
      bearing: 0,
      pitch: 0,
      maxPitch: 0,
      renderWorldCopies: true,
      dragRotate: false,
      pitchWithRotate: false,
      touchPitch: false,
      attributionControl: { compact: true },
      fadeDuration: 0,
      locale: { 'AttributionControl.ToggleAttribution': '地图来源', 'Popup.Close': '关闭来源详情' },
    });
    map = instance;
    instance.touchZoomRotate.disableRotation();
    instance.keyboard.disableRotation();
    instance.getCanvas().setAttribute('aria-label', '平面国家地图，方向键平移，加减键缩放，Home 复位');
    instance.getCanvas().addEventListener('keydown', onKey);
    instance.on('movestart', (event) => {
      if (event.originalEvent) touched = true;
    });
    instance.on('move', updateCamera);
    instance.on('movestart', closeSourcePopup);
    instance.on('mousemove', (event) => {
      if (!instance.isMoving()) interactWithSource(event);
    });
    instance.on('mouseout', schedulePopupClose);
    instance.on('click', (event) => interactWithSource(event, true));
    instance.on('error', (event) => {
      console.error('MapLibre:', event.error);
      mapError.value = '地图加载失败，请重试。';
    });
    instance.on('webglcontextlost', () => {
      stopAnimation();
      mapError.value = '地图绘图环境暂时不可用，请重试。';
    });
    instance.on('load', () => {
      if (map !== instance) return;
      ready.value = true;
      mapError.value = '';
      fitDefault();
      updateRegions();
      updateTarget();
      updateChina();
      updateFlights();
      observer = new ResizeObserver(() => {
        instance.resize();
        if (!touched) fitDefault();
        else updateCamera();
      });
      observer.observe(container.value!);
    });
    instance.on('click', 'country-fill', (event) => {
      selected.value = String(event.features?.[0]?.properties.id || '');
    });
    instance.on('click', 'origin-points', (event) => {
      selected.value = String(event.features?.[0]?.properties.id || '');
    });
    instance.on('mouseenter', 'country-fill', () => {
      instance.getCanvas().style.cursor = 'pointer';
    });
    instance.on('mouseleave', 'country-fill', () => {
      instance.getCanvas().style.cursor = '';
    });
  } catch {
    removeMap();
    mapError.value = '地图需要浏览器支持 WebGL，请启用硬件加速后重试。';
  }
}
watch(regionData, updateRegions);
watch([regionData, () => props.events], refreshSourcePopup);
watch(targetPoint, updateTarget);
watch(highlightChina, updateChina);
watch(paths, updateFlights);
watch([() => props.paused, reducedMotion], animateFlights);
watch(() => props.dark, updateTheme);
watch(selected, (id) => {
  if (ready.value) map?.setFilter('selected-border', ['==', ['get', 'id'], id]);
});
function onMotionChange() {
  reducedMotion.value = Boolean(media?.matches);
}
onMounted(() => {
  media = window.matchMedia('(prefers-reduced-motion: reduce)');
  onMotionChange();
  media.addEventListener('change', onMotionChange);
  document.addEventListener('visibilitychange', animateFlights);
  initializeMap();
  timer = setInterval(() => {
    const now = Date.now();
    if (flights.value.some((f) => f.until <= now)) flights.value = flights.value.filter((f) => f.until > now);
  }, 500);
});
onBeforeUnmount(() => {
  clearInterval(timer);
  media?.removeEventListener('change', onMotionChange);
  document.removeEventListener('visibilitychange', animateFlights);
  removeMap();
});
defineExpose({ getMap: () => map, resetView });
</script>
<style scoped>
.world-map {
  position: relative;
  width: 100%;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border-radius: 7px;
}
.map-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
.map-canvas :deep(.maplibregl-canvas:focus-visible) {
  outline: 1px solid var(--screen-accent);
  outline-offset: -1px;
}
.map-canvas :deep(.maplibregl-ctrl-attrib) {
  font-size: 13px;
  color: var(--screen-muted);
  background: var(--screen-panel);
}
.map-canvas :deep(.maplibregl-ctrl-attrib a) {
  color: inherit;
}
.map-canvas :deep(.origin-ip-popup) {
  z-index: 4;
}
.map-canvas :deep(.origin-ip-popup.maplibregl-popup-anchor-center .maplibregl-popup-tip) {
  display: none;
}
.map-canvas :deep(.origin-ip-popup.maplibregl-popup-anchor-center .source-popup ul) {
  max-height: 112px;
}
.map-canvas :deep(.origin-ip-popup .maplibregl-popup-content) {
  padding: 14px 16px;
  border: 1px solid var(--screen-border);
  border-radius: 8px;
  color: var(--screen-text);
  background: var(--screen-panel);
  box-shadow: none;
}
.map-canvas :deep(.origin-ip-popup .maplibregl-popup-close-button) {
  right: 5px;
  top: 3px;
  color: var(--screen-muted);
  font-size: 20px;
  padding: 0 5px;
}
.map-canvas :deep(.maplibregl-popup-anchor-top .maplibregl-popup-tip),
.map-canvas :deep(.maplibregl-popup-anchor-top-left .maplibregl-popup-tip),
.map-canvas :deep(.maplibregl-popup-anchor-top-right .maplibregl-popup-tip) {
  border-bottom-color: var(--screen-panel);
}
.map-canvas :deep(.maplibregl-popup-anchor-bottom .maplibregl-popup-tip),
.map-canvas :deep(.maplibregl-popup-anchor-bottom-left .maplibregl-popup-tip),
.map-canvas :deep(.maplibregl-popup-anchor-bottom-right .maplibregl-popup-tip) {
  border-top-color: var(--screen-panel);
}
.map-canvas :deep(.maplibregl-popup-anchor-left .maplibregl-popup-tip) {
  border-right-color: var(--screen-panel);
}
.map-canvas :deep(.maplibregl-popup-anchor-right .maplibregl-popup-tip) {
  border-left-color: var(--screen-panel);
}
.map-canvas :deep(.source-popup) {
  min-width: 200px;
  font:
    12px/1.5 'Segoe UI',
    'Microsoft YaHei',
    sans-serif;
}
.map-canvas :deep(.source-popup-title) {
  font-size: 14px;
  padding-right: 15px;
}
.map-canvas :deep(.source-popup-count) {
  margin: 4px 0;
  color: var(--screen-text);
}
.map-canvas :deep(.source-popup-hint),
.map-canvas :deep(.source-popup-empty) {
  color: var(--screen-muted);
  font-size: 13px;
}
.map-canvas :deep(.source-popup ul) {
  list-style: none;
  margin: 8px 0;
  padding: 0;
  max-height: 132px;
  overflow-y: auto;
}
.map-canvas :deep(.source-popup li) {
  padding: 6px 0;
  border-top: 1px solid var(--screen-border);
}
.map-canvas :deep(.source-popup-ip) {
  display: block;
  font:
    11px/1.5 Consolas,
    monospace;
  overflow-wrap: anywhere;
  user-select: text;
}
.map-canvas :deep(.source-popup li small) {
  display: block;
  color: var(--screen-muted);
  font-size: 13px;
}
.map-toolbar {
  position: absolute;
  top: 10px;
  right: 10px;
  display: flex;
  align-items: center;
  padding: 3px;
  gap: 2px;
  border: 1px solid var(--screen-border);
  border-radius: 7px;
  background: var(--screen-panel);
}
.map-toolbar button {
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--screen-text);
  font-size: 18px;
  padding: 0;
}
.map-toolbar button:hover:not(:disabled) {
  background: var(--screen-soft);
}
.map-toolbar button:disabled {
  opacity: 0.35;
  cursor: default;
}
.map-toolbar output {
  width: 43px;
  text-align: center;
  font-size: 13px;
  color: var(--screen-muted);
  font-variant-numeric: tabular-nums;
}
.map-toolbar .map-reset {
  width: 42px;
  font-size: 13px;
  border-left: 1px solid var(--screen-border);
  border-radius: 0 4px 4px 0;
}
.node-location-status {
  pointer-events: none;
  position: absolute;
  bottom: 76px;
  left: 10px;
  right: 10px;
  text-align: center;
  font-size: 13px;
  color: var(--screen-muted);
}
.map-canvas :deep(.protection-node) {
  width: 26px;
  height: 26px;
  pointer-events: none;
}
.map-canvas :deep(.node-symbol) {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border: 2px solid #5aaba6;
  border-radius: 50%;
  background: var(--screen-panel);
  color: #428d87;
  font-size: 17px;
  font-weight: 500;
  box-shadow: 0 0 0 5px #5aaba622;
}
.map-canvas :deep(.node-copy) {
  position: absolute;
  top: -3px;
  left: 35px;
  white-space: nowrap;
  padding: 4px 6px;
  border-radius: 4px;
  background: var(--screen-panel);
}
.map-canvas :deep(.label-left .node-copy) {
  left: auto;
  right: 35px;
  text-align: right;
}
.map-canvas :deep(.node-copy strong) {
  display: block;
  font-size: 13px;
  color: var(--screen-text);
  font-weight: 500;
}
.map-canvas :deep(.node-copy small) {
  display: block;
  font-size: 9px;
  color: var(--screen-muted);
}
.map-callout {
  position: absolute;
  left: 10px;
  top: 10px;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 12px;
  background: var(--screen-panel);
  border: 1px solid var(--screen-border);
  border-radius: 8px;
  font-size: 13px;
  pointer-events: none;
}
.map-callout strong {
  color: var(--screen-text);
  font-weight: 500;
}
.map-callout span {
  color: #b65e46;
  font-variant-numeric: tabular-nums;
}
.map-callout small,
.map-empty {
  font-size: 13px;
  color: var(--screen-muted);
}
.map-signal {
  width: 7px;
  height: 7px;
  background: #d27a5b;
  border-radius: 50%;
  box-shadow: 0 0 0 4px #d27a5b1b;
}
.map-caption {
  position: absolute;
  bottom: 30px;
  width: fit-content;
  max-width: calc(100% - 24px);
  left: 50%;
  transform: translateX(-50%);
  font-size: 13px;
  color: var(--screen-muted);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 6px 12px;
  padding: 6px 10px;
  background: var(--screen-panel);
  border-radius: 5px;
  pointer-events: none;
}
.map-legend {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}
.neutral-swatch {
  width: 10px;
  height: 10px;
  border: 1px solid #839caf;
  background: #cddbe5;
  border-radius: 2px;
}
.attack-scale {
  width: 60px;
  height: 7px;
  background: linear-gradient(90deg, #f1d9b6, #e6ac69, #dc7857, #cd4f4e);
  border-radius: 2px;
}
.focus-caption {
  color: #df4148;
}
.map-error,
.map-loading {
  position: absolute;
  inset: 40% 12px auto;
  padding: 12px;
  border-radius: 6px;
  background: var(--screen-panel);
  text-align: center;
  color: var(--screen-muted);
  font-size: 13px;
}
.map-error button {
  margin-left: 12px;
  border: 1px solid var(--screen-border);
  border-radius: 4px;
  color: var(--screen-text);
  background: var(--screen-panel);
  padding: 5px 9px;
}
@media (max-width: 1100px) {
  .world-map {
    min-height: 300px;
  }
}
@media (max-width: 600px) {
  .world-map {
    min-height: 300px;
  }
  .map-callout {
    left: 0;
    top: 0;
    padding: 8px;
    font-size: 13px;
    gap: 7px;
    max-width: calc(100% - 170px);
    flex-wrap: wrap;
    white-space: normal;
  }
  .map-toolbar {
    top: 0;
    right: 0;
  }
  .map-caption {
    width: calc(100% - 20px);
    bottom: 26px;
    gap: 4px 8px;
  }
  .map-help {
    display: none;
  }
  .map-canvas :deep(.node-copy) {
    max-width: 150px;
    white-space: normal;
    width: max-content;
  }
}
</style>
