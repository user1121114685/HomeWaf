import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { v4 as uuidv4 } from 'uuid';
import request from '@/utils/request';
import proxy from '@/config/host';
import {
  sourceLabels,
  validateSnapshot,
  type ScreenData,
  type ScreenSnapshot,
  type Source,
  type SourceKey,
} from './data';

export function useScreenData(options: { shared: boolean; token: string }) {
  const sources = reactive(
    Object.fromEntries(Object.keys(sourceLabels).map((key) => [key, { data: null, updatedAt: 0, failed: false }])),
  ) as { [K in SourceKey]: Source<ScreenData[K]> };
  const snapshot = ref<ScreenSnapshot | null>(null);
  const sharedError = ref('');
  const paused = ref(false),
    busy = ref(false),
    now = ref(new Date());
  const controller = new AbortController();
  const keys = Object.keys(sourceLabels) as SourceKey[];
  let stopped = false,
    failures = 0;
  let timer: ReturnType<typeof setTimeout>, clock: ReturnType<typeof setInterval>;
  function schedule() {
    clearTimeout(timer);
    if (!stopped && !paused.value && !document.hidden && !sharedError.value)
      timer = setTimeout(refresh, Math.min(60000, 5000 * 2 ** failures));
  }
  async function refresh() {
    if (busy.value || stopped) return;
    busy.value = true;
    clearTimeout(timer);
    try {
      let data: unknown;
      if (options.shared) {
        const base = proxy[import.meta.env.MODE || 'development'].API;
        const response = await fetch(base + '/public/dashboard-share', {
          headers: {
            'X-Dashboard-Token': options.token,
            'X-Request-Time': String(Math.floor(Date.now() / 1000)),
            'X-Request-Id': uuidv4(),
          },
          credentials: 'omit',
          cache: 'no-store',
          signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]),
        });
        if (response.status === 401 || response.status === 403) {
          sharedError.value =
            response.status === 403
              ? '当前来源无权访问此分享，请联系分享者检查访问范围。'
              : '分享链接无效、已过期或已撤销，请联系分享者。';
          snapshot.value = null;
          for (const key of keys) Object.assign(sources[key], { data: null, updatedAt: 0, failed: true });
          return;
        }
        const envelope = await response.json();
        if (!response.ok || envelope.code !== 0) throw new Error('Share data unavailable');
        data = envelope.data;
      } else {
        const envelope = await request({
          url: 'dashboard/snapshot',
          method: 'get',
          background: true,
          timeout: 15000,
          signal: controller.signal,
        });
        if (envelope.code !== 0) throw new Error('Screen data unavailable');
        data = envelope.data;
      }
      if (stopped) return;
      if (!validateSnapshot(data)) throw new Error('Invalid screen snapshot');
      snapshot.value = data;
      const values: ScreenData = {
        summary: data.summary,
        ips: { AttackIPOfRange: data.ips },
        regions: data.regions,
        events: { list: data.events, partial: data.partial },
        qps: data.qps,
      };
      for (const key of keys) Object.assign(sources[key], { data: values[key], updatedAt: Date.now(), failed: false });
      failures = 0;
    } catch {
      if (!stopped) {
        failures++;
        for (const key of keys) sources[key].failed = true;
      }
    } finally {
      busy.value = false;
      schedule();
    }
  }
  function togglePause() {
    paused.value = !paused.value;
    clearTimeout(timer);
    if (!paused.value) void refresh();
  }
  function onVisibility() {
    clearTimeout(timer);
    if (!document.hidden && !paused.value) void refresh();
  }
  function status(key: SourceKey) {
    if (sharedError.value) return '分享不可用';
    const source = sources[key];
    if (source.failed) return source.updatedAt ? '更新失败 · 保留上次数据' : '连接异常 · 暂无数据';
    if (paused.value) return '自动刷新已暂停';
    if (!source.updatedAt) return '正在连接';
    if (now.value.getTime() - source.updatedAt > 20000) return '数据已过期 · 等待更新';
    return '';
  }
  const issues = computed(() =>
    keys.filter((key) => (Boolean(status(key)) && sources[key].updatedAt > 0) || sources[key].failed),
  );
  onMounted(() => {
    void refresh();
    clock = setInterval(() => {
      now.value = new Date();
    }, 1000);
    document.addEventListener('visibilitychange', onVisibility);
  });
  onBeforeUnmount(() => {
    stopped = true;
    controller.abort();
    clearTimeout(timer);
    clearInterval(clock);
    document.removeEventListener('visibilitychange', onVisibility);
  });
  return { sources, snapshot, sharedError, paused, busy, now, issues, status, refresh, togglePause };
}
