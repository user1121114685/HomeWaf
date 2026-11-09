export interface Summary {
  visits: number;
  blocked: number;
  visitors: number;
  attack_ips: number;
}
export interface RankedIP {
  ip: string;
  count: number;
  country?: string;
}
export interface Region {
  name: string;
  value: number;
}
export interface SecurityEvent {
  req_uuid: string;
  src_ip: string;
  country?: string;
  province?: string;
  city?: string;
  create_time: string;
  host: string;
  rule: string;
  action: string;
  unix_add_time: number;
}
export interface ScreenTarget {
  mode: 'auto' | 'coordinates' | 'logical';
  label: string;
  latitude: number;
  longitude: number;
  status?: 'locating' | 'ready' | 'unavailable';
  public_ip?: string;
  country?: string;
  region?: string;
  city?: string;
  has_coordinates?: boolean;
  source?: string;
  checked_at?: number;
  message?: string;
}
export interface ScreenData {
  summary: Summary;
  ips: { AttackIPOfRange: RankedIP[] | null };
  regions: Region[] | null;
  events: { list: SecurityEvent[] | null; partial?: boolean };
  qps: { Current: number; Max: number; Points: { T: number; V: number }[] | null };
}
export interface ScreenSnapshot {
  window_start: number;
  window_end: number;
  timezone: string;
  summary: Summary;
  ips: RankedIP[];
  regions: Region[];
  events: SecurityEvent[];
  qps: ScreenData['qps'];
  target: ScreenTarget;
  partial: boolean;
  issues: string[];
  log_mode: string;
  visitor_basis: string;
  share_expires_at?: number;
}
export type SourceKey = keyof ScreenData;
export interface Source<T> {
  data: T | null;
  updatedAt: number;
  failed: boolean;
}
export const sourceLabels: Record<SourceKey, string> = {
  summary: '近 24 小时汇总',
  ips: '攻击 IP',
  regions: '地域分布',
  events: '攻击事件',
  qps: '实时 QPS',
};
export function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0;
}
export function validatePayload(key: SourceKey, data: unknown): boolean {
  if (key === 'regions')
    return data === null || (Array.isArray(data) && data.every((r) => typeof r?.name === 'string' && isCount(r.value)));
  if (!data || typeof data !== 'object' || Array.isArray(data)) return false;
  const d = data as Record<string, any>;
  if (key === 'summary') return ['visits', 'blocked', 'visitors', 'attack_ips'].every((k) => isCount(d[k]));
  if (key === 'ips')
    return (
      d.AttackIPOfRange === null ||
      (Array.isArray(d.AttackIPOfRange) &&
        d.AttackIPOfRange.every((r) => typeof r?.ip === 'string' && isCount(r.count)))
    );
  if (key === 'events')
    return (
      d.list === null ||
      (Array.isArray(d.list) &&
        d.list.every(
          (r) => typeof r?.src_ip === 'string' && typeof r?.req_uuid === 'string' && isCount(r.unix_add_time),
        ))
    );
  return (
    isCount(d.Current) &&
    isCount(d.Max) &&
    (d.Points === null || (Array.isArray(d.Points) && d.Points.every((p) => isCount(p?.T) && isCount(p?.V))))
  );
}
export function validateSnapshot(d: any): d is ScreenSnapshot {
  return (
    d &&
    isCount(d.window_start) &&
    isCount(d.window_end) &&
    d.window_end - d.window_start === 86400000 &&
    validatePayload('summary', d.summary) &&
    validatePayload('ips', { AttackIPOfRange: d.ips }) &&
    validatePayload('regions', d.regions) &&
    validatePayload('events', { list: d.events }) &&
    validatePayload('qps', d.qps) &&
    ['auto', 'logical', 'coordinates'].includes(d.target?.mode) &&
    typeof d.target.label === 'string' &&
    Number.isFinite(d.target.latitude) &&
    Number.isFinite(d.target.longitude) &&
    Math.abs(d.target.latitude) <= 90 &&
    Math.abs(d.target.longitude) <= 180 &&
    Array.isArray(d.issues) &&
    d.issues.every((issue: unknown) => typeof issue === 'string')
  );
}
export function formatCount(value: unknown) {
  return isCount(value) ? value.toLocaleString('en-US') : '—';
}
export function blockRate(summary: Summary | null) {
  return summary && summary.visits > 0 ? ((summary.blocked / summary.visits) * 100).toFixed(1) + '%' : '—';
}
export function eventLocation(event: SecurityEvent) {
  const parts = [event.country, event.province, event.city].filter((v) => v && v !== '0' && v !== '未知');
  return [...new Set(parts)].join(' · ') || '归属地未知';
}
