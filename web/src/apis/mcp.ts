import request from '@/utils/request';

export interface MCPTool {
  name: string;
  group: string;
  description: string;
  read_only: boolean;
  requires_confirmation: boolean;
}
export interface MCPStatus {
  endpoint: string;
  transport: string;
  enabled: boolean;
  enabled_until?: number;
  suggested_source?: string;
  tools: MCPTool[];
}
export function getMCPStatus() {
  return request({ url: '/oplatform/mcp', method: 'get' });
}

export function mcpConnectionConfig(endpoint: string, token: string) {
  return JSON.stringify({ mcpServers: { homewaf: { type: 'http', url: endpoint, headers: { Authorization: `Bearer ${token || '<MCP_KEY>'}` } } } }, null, 2);
}

// Uses MCP's normal HTTP transport; console session encryption is not part of MCP.
// No credentials are persisted, and testing never calls a configuration tool.
export async function testMCPConnection(endpoint: string, token: string, signal: AbortSignal) {
  const url = new URL(endpoint);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error('请输入不含账号、查询参数和片段的 HTTP(S) MCP 地址。');
  }
  if (!token.trim()) throw new Error('请输入 MCP 专用密钥。');
  const rpc = async (method: string, params: Record<string, unknown>, id: number, protocol?: string) => {
    const headers: Record<string, string> = { Authorization: `Bearer ${token.trim()}`, 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream' };
    if (protocol) headers['MCP-Protocol-Version'] = protocol;
    const response = await fetch(url, { method: 'POST', headers, body: JSON.stringify({ jsonrpc: '2.0', id, method, params }), signal, credentials: 'omit', redirect: 'error' });
    if (!response.ok) throw new Error(response.status === 401 ? '认证失败：检查密钥用途、启用状态、有效期和开放 API 开关。' : response.status === 403 ? '访问被拒绝：检查来源 IP、管理域名和 Origin 限制。' : `连接失败（HTTP ${response.status}），请检查地址和反向代理。`);
    const data = await response.json();
    if (data.error) throw new Error(data.error.message || 'MCP 返回协议错误。');
    if (data.jsonrpc !== '2.0' || data.id !== id || !data.result) throw new Error('此地址没有返回有效的 MCP 响应。');
    return data.result;
  };
  const initialized = await rpc('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'HomeWaf connection check', version: '1.0.0' } }, 1);
  if (!initialized.protocolVersion || !initialized.serverInfo?.name) throw new Error('MCP 初始化响应不完整。');
  const listed = await rpc('tools/list', {}, 2, initialized.protocolVersion);
  if (!Array.isArray(listed.tools)) throw new Error('MCP 工具列表响应不完整。');
  return { name: initialized.serverInfo.name as string, tools: listed.tools.length as number };
}
