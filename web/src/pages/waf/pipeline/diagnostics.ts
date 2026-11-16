import { accessReason } from './accessPolicy.ts';

export interface DiagnosticEvent {
  id?: string;
  req_id?: string;
  time_ms: number;
  client_ip?: string;
  kind: string;
  stage?: number;
  verdict?: string;
  observe?: boolean;
  reason?: string;
  rule_id?: string;
  category?: string;
  elapsed_us?: number;
  detail_json?: string;
}

export const actionLabels: Record<string, string> = {
  allow: '放行',
  bypass: '豁免放行',
  continue: '继续检查',
  deny: '拒绝访问',
  drop: '断开连接',
  challenge: '要求验证',
  honeypot: '进入蜜罐',
  ban: '封禁',
  lockdown: '端口锁定',
  honeypot_lockdown: '蜜罐锁定',
};
const eventLabels: Record<string, string> = {
  created: '处罚已创建',
  updated: '处罚已更新',
  released: '处罚已释放',
  expired: '处罚已到期',
  banned: '来源已封禁',
  made_permanent: '已转为永久封禁',
  lockdown_started: '端口锁定开始',
  lockdown_released: '端口锁定解除',
  honeypot_started: '来源已进入蜜罐',
  honeypot_lockdown_started: '蜜罐锁定开始',
  honeypot_lockdown_released: '蜜罐锁定解除',
};
export function diagnosticReason(row: DiagnosticEvent): string {
  return (row.kind === 'state' && eventLabels[row.reason || '']) || accessReason(row.reason || '') || '未记录原因';
}
export function diagnosticAction(row: DiagnosticEvent): string {
  if (row.kind === 'state') return eventLabels[row.reason || ''] || '处罚状态变化';
  const action = actionLabels[row.verdict || ''] || row.verdict || '未记录动作';
  return row.observe ? `${action} · 观察命中` : action;
}
export function diagnosticDetail(row: DiagnosticEvent): Record<string, any> | null {
  try {
    const data = JSON.parse(row.detail_json || '');
    return data && typeof data === 'object' && !Array.isArray(data) ? data : null;
  } catch {
    return null;
  }
}
export function filterDiagnosticEvents(rows: DiagnosticEvent[], query: string, kind: string) {
  const needle = query.trim().toLowerCase();
  return rows.filter(
    (row) =>
      (!kind || row.kind === kind) &&
      [row.client_ip, row.req_id, row.rule_id, row.reason, diagnosticReason(row), diagnosticAction(row)].some((value) =>
        String(value || '')
          .toLowerCase()
          .includes(needle),
      ),
  );
}
export function diagnosticNumber(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString('zh-CN') : '未提供';
}
export function diagnosticModes(policy: Record<string, any>) {
  const rules = policy.ruleset || {},
    response = policy.network?.response || {};
  const modes = [
    rules.bot,
    rules.owasp,
    rules.ai,
    rules.qps?.mode,
    rules.cc?.mode,
    ...['anti_leech', 'challenge', 'sensitive', 'plugins', 'callback'].map((key) => rules[key]?.mode),
    ...['rules', 'scripts', 'urls'].flatMap((key) => (rules[key] || []).map((rule: any) => rule.mode)),
    ...(response.tamper || []).map((rule: any) => rule.mode),
    response.privacy?.mode,
  ];
  return {
    block: modes.filter((mode) => mode === 'block').length,
    observe: modes.filter((mode) => mode === 'observe').length,
  };
}
export function diagnosticTime(value?: number): string {
  return value && Number.isFinite(value) ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '未记录时间';
}

// Only in-console service destinations may be carried through an editing detour.
export function serviceReturnPath(value: unknown, diagnostics = false): string {
  if (typeof value !== 'string' || value.includes('#') || value.includes('\\')) return '/waf-pipeline/ports';
  const path = value.split('?')[0];
  if (path === '/waf-pipeline/ports' || (diagnostics && /^\/waf-pipeline\/ports\/\d+\/diagnostics$/.test(path)))
    return value;
  return '/waf-pipeline/ports';
}

export const diagnosticMetricGroups = [
  {
    title: '本服务 · 请求处理',
    scope: 'service',
    description: '今日计数按服务器本地日期重置；累计计数随运行时重建而重置。',
    fields: [
      ['port.requests', '累计请求'],
      ['port.denied', '累计拦截'],
      ['port.hits', '累计检测命中'],
      ['port.business_cache_hits', '缓存命中'],
      ['port.business_cache_misses', '缓存未命中'],
    ],
  },
  {
    title: '本服务 · 连接与上游',
    scope: 'service',
    description: '上游请求与失败统计最近 60 秒，其余错误为运行期累计；未启动或未上报时显示“未提供”。',
    fields: [
      ['protocol.connections', '当前连接'],
      ['upstream.requests_60s', '上游请求 / 60 秒'],
      ['upstream.failures_60s', '上游失败 / 60 秒'],
      ['upstream.transport_errors', '累计上游传输错误'],
      ['protocol.tls_errors', '累计 TLS 握手错误'],
      ['protocol.server_errors', '累计监听服务错误'],
      ['protocol.pre_handler_closed', '处理前关闭的连接'],
      ['protocol.malformed_http1_responses', '异常 HTTP/1 响应'],
    ],
  },
  {
    title: '实例共享 · 队列与资源',
    scope: 'instance',
    description: '以下数据由多个服务共享。丢弃、拒绝与错误是累计计数，不能据此判定此服务正在遭受攻击。',
    fields: [
      ['punishment.queued', '待处理处罚事件'],
      ['punishment.capacity', '处罚队列容量'],
      ['punishment.quota_dropped', '处罚配额丢弃'],
      ['punishment.persist_errors', '处罚持久化错误'],
      ['punishment.dropped_low', '处罚队列丢弃 / 低优先级'],
      ['punishment.dropped_medium', '处罚队列丢弃 / 中优先级'],
      ['punishment.dropped_high', '处罚队列丢弃 / 高优先级'],
      ['log_queue.queued', '待写入日志'],
      ['log_queue.capacity', '日志队列容量'],
      ['log_queue.dropped_low', '日志丢弃 / 低优先级'],
      ['log_queue.dropped_medium', '日志丢弃 / 中优先级'],
      ['log_queue.dropped_high', '日志丢弃 / 高优先级'],
      ['admission.in_flight', '正在处理的请求'],
      ['admission.capacity', '请求并发容量'],
      ['admission.rejected', '并发配额拒绝'],
      ['daily_persist_errors', '每日计数持久化错误'],
      ['protocol.global_connections', '实例当前连接'],
      ['protocol.global_connection_rejections', '实例连接配额拒绝'],
    ],
  },
] as const;
export function metricValue(stats: Record<string, any> | null, path: string): unknown {
  return path.split('.').reduce((value, key) => value?.[key], stats as any);
}
