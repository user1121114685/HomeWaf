import { onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref } from 'vue';
import request from '@/utils/request';

export interface SiteTraffic {
  host_code: string;
  host: string;
  host_remark: string;
  total_count: number;
  attack_count: number;
  normal_count: number;
  traffic_in_mb: number;
  traffic_out_mb: number;
  uv_count: number;
  ip_count: number;
  avg_time_ms: number;
}
export interface TrafficOverview {
  total_ip: number;
  total_uv: number;
  total_pv: number;
  total_attack: number;
  total_in_mb: number;
  total_out_mb: number;
  site_list: SiteTraffic[];
}
export interface TrafficIP {
  ip: string;
  count: number;
  ip_belong?: string;
  ip_tags?: any[];
}
export interface TrafficRegion {
  name: string;
  value: number;
}
export interface TrafficTrend {
  AttackCountOfRange: Record<string, number>;
  NormalCountOfRange: Record<string, number>;
}
export interface TrafficRanks {
  AttackIPOfRange: TrafficIP[];
  NormalIPOfRange: TrafficIP[];
}
export interface TrafficQps {
  Current: number;
  Max: number;
  Points: { T: number; V: number }[];
}
interface TrafficData {
  overview: TrafficOverview | null;
  trend: TrafficTrend | null;
  ranks: TrafficRanks | null;
  regions: TrafficRegion[] | null;
  blockedRegions: TrafficRegion[] | null;
}
type Source = keyof TrafficData;
const endpoints: Record<Source, string> = {
  overview: 'wafstatsiteoverview',
  trend: 'wafstatsumdayrange',
  ranks: 'wafstatsumdaytopiprange',
  regions: 'analysis/wafanalysisdaycountryrange',
  blockedRegions: 'analysis/wafanalysisdaycountryrange',
};

export function useTrafficData(getRange: () => [string, string]) {
  const data = reactive<TrafficData>({ overview: null, trend: null, ranks: null, regions: null, blockedRegions: null });
  const errors = ref<string[]>([]),
    loading = ref(false),
    updatedAt = ref('');
  const qps = ref<TrafficQps | null>(null),
    qpsFailed = ref(false);
  let active = false,
    sequence = 0,
    qpsSequence = 0,
    failures = 0,
    qpsFailures = 0;
  let controller: AbortController | undefined, qpsController: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined, qpsTimer: ReturnType<typeof setTimeout> | undefined;

  async function refresh(clear = false) {
    if (!active) return;
    const id = ++sequence;
    controller?.abort();
    controller = new AbortController();
    clearTimeout(timer);
    if (clear) {
      for (const key of Object.keys(endpoints) as Source[]) data[key] = null;
      updatedAt.value = '';
    }
    loading.value = true;
    const [start, end] = getRange();
    const params = { start_day: start.replaceAll('-', ''), end_day: end.replaceAll('-', '') };
    const results = await Promise.allSettled(
      (Object.keys(endpoints) as Source[]).map(async (key) => {
        const result = await request({
          url: endpoints[key],
          method: 'get',
          params: {
            ...params,
            ...(key === 'regions' || key === 'blockedRegions'
              ? { attack_type: key === 'blockedRegions' ? '阻止' : '' }
              : {}),
          },
          background: true,
          timeout: 15000,
          signal: controller!.signal,
        });
        if (result.code !== 0) throw new Error(key);
        if (key === 'regions' || key === 'blockedRegions') {
          if (result.data !== null && !Array.isArray(result.data)) throw new Error(key);
          return {
            key,
            value: (result.data || []).map((r: TrafficRegion) => ({
              name: String(r.name || ''),
              value: Number(r.value) || 0,
            })),
          };
        }
        if (!result.data || typeof result.data !== 'object') throw new Error(key);
        if (key === 'overview' && !Array.isArray(result.data.site_list)) throw new Error(key);
        if (key === 'trend' && (!result.data.AttackCountOfRange || !result.data.NormalCountOfRange))
          throw new Error(key);
        return { key, value: result.data };
      }),
    );
    if (!active || id !== sequence) return;
    const failed: string[] = [];
    results.forEach((result, index) => {
      if (result.status === 'fulfilled') (data as any)[result.value.key] = result.value.value;
      else failed.push(Object.keys(endpoints)[index]);
    });
    errors.value = failed;
    if (results.some((r) => r.status === 'fulfilled')) updatedAt.value = new Date().toLocaleTimeString('en-GB');
    loading.value = false;
    failures = failed.length ? failures + 1 : 0;
    if (!document.hidden) timer = setTimeout(() => refresh(), Math.min(120000, 30000 * 2 ** failures));
  }

  async function refreshQps() {
    if (!active) return;
    const id = ++qpsSequence;
    qpsController?.abort();
    qpsController = new AbortController();
    clearTimeout(qpsTimer);
    try {
      const result = await request({
        url: 'wafstatqpstrend',
        method: 'get',
        params: { limit: 60 },
        background: true,
        timeout: 10000,
        signal: qpsController.signal,
      });
      if (!active || id !== qpsSequence) return;
      if (result.code !== 0 || !Number.isFinite(result.data?.Current)) throw new Error('QPS unavailable');
      qps.value = { ...result.data, Points: result.data.Points || [] };
      qpsFailed.value = false;
      qpsFailures = 0;
    } catch {
      if (!active || id !== qpsSequence) return;
      qpsFailed.value = true;
      qpsFailures++;
    }
    if (active && id === qpsSequence && !document.hidden)
      qpsTimer = setTimeout(refreshQps, Math.min(60000, 5000 * 2 ** qpsFailures));
  }
  function visible() {
    clearTimeout(timer);
    clearTimeout(qpsTimer);
    if (!document.hidden && active) {
      void refresh();
      void refreshQps();
    }
  }
  function start() {
    if (active) return;
    active = true;
    document.addEventListener('visibilitychange', visible);
    visible();
  }
  function stop() {
    active = false;
    sequence++;
    qpsSequence++;
    controller?.abort();
    qpsController?.abort();
    clearTimeout(timer);
    clearTimeout(qpsTimer);
    document.removeEventListener('visibilitychange', visible);
  }
  onMounted(start);
  onActivated(start);
  onDeactivated(stop);
  onBeforeUnmount(stop);
  return { data, errors, loading, updatedAt, qps, qpsFailed, refresh, refreshQps };
}
