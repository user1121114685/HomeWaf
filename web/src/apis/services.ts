// HomeWaf modification (2026-11-09): canonical service list for the v3 console.
import { v4 as uuidv4 } from 'uuid';
import request from '@/utils/request';
import { pipelineGet, pipelinePost } from './pipeline';

export function listServices() {
  return request({ url: '/waflog/services', method: 'get' });
}

export async function addServiceIPRule(code: string, type: 'allow' | 'deny', value: string, remark: string) {
  const match = /^pipeline-([1-9]\d{0,4})$/.exec(code);
  if (!match) throw new Error('请选择一个现有的受保护服务');
  const config = await pipelineGet('ports/detail', { port: Number(match[1]) });
  if (config.host_code !== code) throw new Error('服务已变更，请刷新后重试');
  const group = config.policy.ip_group;
  const entries = group.entries || [];
  if (entries.some(entry => entry.type === type && entry.value === value && (!entry.expires_at || entry.expires_at > Date.now() / 1000))) {
    throw new Error('此服务已配置相同的 IP 规则');
  }
  group.entries = [...entries, { id: uuidv4(), type, value, remark, ...(type === 'allow' ? { trust: 'allow_only' } : {}) }];
  const saved = await pipelinePost('ports/save', { config });
  return saved.listen_enabled ? 'IP 规则已应用到当前服务' : 'IP 规则已保存，启用服务后生效';
}
