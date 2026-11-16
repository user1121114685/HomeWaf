import request from '@/utils/request';
export type CertificateRecord = Record<string, any>;
export const certificateListApi = (data: Record<string, any>) =>
  request({ url: '/certificates/list', method: 'post', data });
export const certificateDetailApi = (id: string) =>
  request({ url: '/certificates/detail', method: 'get', params: { id } });
export const certificatePolicyApi = (data: Record<string, any>) =>
  request({ url: '/certificates/policy', method: 'post', data });
export const certificateRenewApi = (id: string, request_id: string) =>
  request({ url: '/certificates/renew', method: 'post', data: { id, request_id } });
export const certificateSettingsApi = () => request({ url: '/certificates/settings', method: 'get' });
export const certificateNotificationsApi = (id: string) =>
  request({ url: '/certificates/notifications', method: 'get', params: { id } });
export const certificateNotificationRetryApi = (id: string) =>
  request({ url: '/certificates/notifications/retry', method: 'post', data: { id } });
export const certificateRoot = '/waf-host/wafsslconfiglist';
export const renewalStates: Record<string, string> = {
  idle: '等待续期窗口',
  queued: '等待执行',
  running: '续期中',
  deploying: '正在更新服务',
  retry_wait: '失败，等待重试',
  blocked: '需要处理',
  unresolved: '来源待确认',
  manual: '手动维护已接管',
};
export const validityStates: Record<string, string> = {
  valid: '有效',
  expiring: '即将到期',
  expired: '已过期',
  invalid: '有效期未知',
  not_yet_valid: '尚未生效',
};
export const updateMethods: Record<string, string> = {
  managed: 'HomeWaf 自动申请',
  manual: '手动维护 / 未关联来源',
  file: '外部文件同步',
};
export function showTime(value?: string) {
  return value && !value.startsWith('0001') && !value.startsWith('0000') ? value : '—';
}
export function renewalLabel(record: CertificateRecord) {
  const policy = record.renewal;
  if (!policy) return record.update_method === 'file' ? '由外部程序更新' : '未配置自动申请';
  if (['blocked', 'unresolved', 'manual', 'running', 'queued', 'retry_wait', 'deploying'].includes(policy.status))
    return renewalStates[policy.status];
  return policy.enabled ? '已开启，等待续期窗口' : '自动续期已关闭';
}
export function safeCertificateReturn(value: unknown, fallback = certificateRoot) {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//') && !value.includes('://')
    ? value
    : fallback;
}

export const certificateBindingTargetsApi = () => request({ url: '/certificates/bind-targets', method: 'get' });
export const certificateBindApi = (data: Record<string, any>) =>
  request({ url: '/certificates/bind', method: 'post', data });
