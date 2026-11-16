export interface DefensePolicy {
  enabled: boolean;
  mode: string;
  window_seconds: number;
  count: number;
  duration_seconds: number;
  tcp_action: string;
}

export const defenseModes = [
  {
    value: 'quiet',
    label: '无提示拒绝',
    description: '异常达到设定次数后，直接断开被拒绝的连接，不再显示提示。正常访问继续。',
  },
  {
    value: 'restrict',
    label: '收紧 IP 访问',
    description: '异常达到设定次数后，只允许近期成功访问过的网段继续接受检查。',
  },
  { value: 'close', label: '自动关闭访问', description: '异常达到设定次数后，暂时关闭整个服务，所有访客都无法访问。' },
  {
    value: 'honeypot',
    label: '蜜罐',
    description: '第一次安全拒绝就将该请求引入仿真页面，不接触真实业务。正常访问继续。',
  },
];

export const defenseModeNames = Object.fromEntries(defenseModes.map(({ value, label }) => [value, label]));

export function newDefensePolicy(): DefensePolicy {
  return { enabled: true, mode: '', window_seconds: 1800, count: 10, duration_seconds: 10800, tcp_action: 'reset' };
}

export function defenseValidation(policy?: DefensePolicy, required = false): string {
  if (!policy || (required && !policy.enabled)) return '请选择并配置一种异常访问处置方式。';
  if (!defenseModes.some(({ value }) => value === policy.mode)) return '请选择一种异常访问处置方式。';
  if (!Number.isInteger(policy.window_seconds) || policy.window_seconds < 1 || policy.window_seconds > 86400)
    return '请填写有效的异常累计时间，最长为 24 小时。';
  if (!Number.isInteger(policy.count) || policy.count < 1 || policy.count > 100000)
    return '请填写 1–100000 之间的异常次数。';
  if (!Number.isInteger(policy.duration_seconds) || policy.duration_seconds < 1 || policy.duration_seconds > 604800)
    return '请填写有效的自动恢复时间，最长为 7 天。';
  if (!['reset', 'drop'].includes(policy.tcp_action)) return '请选择拒绝连接的方式。';
  return '';
}

function duration(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`;
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`;
  return `${seconds} 秒`;
}

export function defenseSummary(policy?: DefensePolicy): string {
  if (!policy?.mode) return '选择处置方式后，这里会显示完整规则。';
  if (!policy.enabled) return '异常访问处置已暂停，原有访问限制和安全检测继续执行。';
  if (policy.mode === 'honeypot') return '第一次安全拒绝就引入统一蜜罐，无需累计次数；正常访问继续。';
  if (defenseValidation(policy)) return '请补全下方参数，查看完整处置规则。';
  const effect: Record<string, string> = {
    quiet: '对后续被拒绝的连接不再发送提示，正常访问继续',
    restrict: '仅允许近 30 天成功来源所在的网段继续访问；没有记录时关闭服务访问',
    close: '关闭整个服务访问，包括可信来源',
  };
  return `最近 ${duration(policy.window_seconds)}内，累计 ${policy.count} 次安全拒绝或认证失败后，${effect[policy.mode]}。${duration(policy.duration_seconds)}后自动解除本次处置。`;
}
