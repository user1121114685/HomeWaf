// HomeWaf modification (2026-11-14): readable, ordered access-log views.
import dayjs from 'dayjs';

export const recommendedLogColumns = ['outcome', 'create_time', 'service', 'src_ip', 'url', 'status_code', 'op'];

export function requestOutcome(row: Record<string, any>) {
  const blocked = ['阻止', '禁止'].includes(row.action);
  const unknown = row.action !== '放行' && !blocked;
  const matched = !!row.rule || Number(row.log_only_mode) === 1;
  const failed = Number(row.status_code) >= 400;
  const observeOnly = row.action === '放行' && Number(row.log_only_mode) === 1;
  const observed = observeOnly && !failed;
  const abnormal = blocked || (matched && !observeOnly) || failed || unknown;
  return { abnormal, observed, observeOnly, blocked, matched, failed, unknown };
}

export function normalizeLogColumns(columns: unknown, available: string[]) {
  if (!Array.isArray(columns) || !columns.length) return [...recommendedLogColumns];
  const renamed: Record<string, string> = {
    action: 'outcome',
    host: 'service',
    status: 'status_code',
    host_nickname: 'service',
  };
  const result = [...new Set(columns.map((key) => renamed[key] || key))].filter((key) => available.includes(key));
  if (!result.includes('op')) result.push('op');
  return result;
}

export function logDateRange(preset: string, now = dayjs()) {
  const end = now.endOf('day');
  if (preset === '24h') return [now.subtract(24, 'hour'), now].map((date) => date.format('YYYY-MM-DD HH:mm:ss'));
  return [preset === '7d' ? now.subtract(6, 'day').startOf('day') : now.startOf('day'), end].map((date) =>
    date.format('YYYY-MM-DD HH:mm:ss'),
  );
}

export function logTimeBounds(range: unknown) {
  if (!Array.isArray(range) || range.length !== 2 || !range[0] || !range[1]) return null;
  const begin = dayjs(range[0]);
  const end = /^\d{4}-\d{2}-\d{2}$/.test(String(range[1])) ? dayjs(range[1]).endOf('day') : dayjs(range[1]);
  if (!begin.isValid() || !end.isValid() || begin.valueOf() > end.valueOf()) return null;
  return { unix_add_time_begin: String(begin.valueOf()), unix_add_time_end: String(end.valueOf()) };
}

export function logColumns(text: (zh: string, en: string) => string) {
  return [
    { colKey: 'outcome', title: text('访问情况', 'Result'), width: 150 },
    { colKey: 'create_time', title: text('时间', 'Time'), width: 170, sorter: true, sortKey: 'unix_add_time' },
    { colKey: 'service', title: text('服务', 'Service'), minWidth: 145, ellipsis: true },
    { colKey: 'src_ip', title: text('来源 IP', 'Source IP'), minWidth: 150 },
    { colKey: 'url', title: text('访问路径', 'Request path'), minWidth: 210 },
    { colKey: 'status_code', title: text('响应码', 'HTTP status'), width: 95 },
    { colKey: 'op', title: text('操作', 'Actions'), width: 90 },
    { colKey: 'rule', title: text('命中规则', 'Matched rule'), width: 170, ellipsis: true },
    { colKey: 'time_spent', title: text('耗时(ms)', 'Duration (ms)'), width: 110, sorter: true },
    { colKey: 'method', title: text('请求方法', 'Method'), width: 110 },
    { colKey: 'country', title: text('国家 / 地区', 'Country / region'), width: 130, ellipsis: true },
    { colKey: 'province', title: text('省份', 'Province'), width: 110, ellipsis: true },
    { colKey: 'city', title: text('城市', 'City'), width: 110, ellipsis: true },
    {
      colKey: 'guest_identification',
      title: text('访客身份', 'Visitor'),
      width: 150,
      ellipsis: true,
      filter: { type: 'input' },
    },
    {
      colKey: 'req_uuid',
      title: text('请求编号', 'Request ID'),
      width: 230,
      ellipsis: true,
      filter: { type: 'input' },
    },
    { colKey: 'user_agent', title: 'User-Agent', width: 240, ellipsis: true, filter: { type: 'input' } },
    { colKey: 'referer', title: 'Referer', width: 240, ellipsis: true, filter: { type: 'input' } },
    {
      colKey: 'header',
      title: text('请求头', 'Request headers'),
      width: 260,
      ellipsis: true,
      filter: { type: 'input' },
    },
    { colKey: 'log_only_mode', title: text('仅记录模式', 'Observe mode'), width: 150 },
    { colKey: 'risk_level', title: text('风险等级', 'Risk level'), width: 110 },
  ];
}
