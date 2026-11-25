export interface Sample {
  id: string;
  category: string;
  kind: 'attack' | 'benign' | 'unknown';
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  truncated: boolean;
  imported?: boolean;
}

// Export metadata, response bodies, private hostnames and credentials are never sent.
export function importAssessmentLog(value: unknown): Sample[] {
  const data = value as any;
  const rows = Array.isArray(data) ? data : (data?.detail_records ?? data?.list_records);
  if (!Array.isArray(rows) || !rows.length || rows.length > 256) throw new Error('请选择包含 1–256 条请求的日志 JSON');
  return rows.map((row: any, index: number) => {
    if (!row || typeof row !== 'object') throw new Error(`第 ${index + 1} 条记录无效`);
    let path = String(row.url || row.path || '/');
    if (/^https?:\/\//i.test(path)) {
      const u = new URL(path);
      path = u.pathname + u.search;
    }
    if (row.raw_query && !path.includes('?')) path += '?' + row.raw_query;
    if (!path.startsWith('/') || path.startsWith('//') || path.length > 8192)
      throw new Error(`第 ${index + 1} 条路径无效`);
    const headers: Record<string, string> = {};
    const add = (key: string, value: unknown) => {
      if (
        /^(host|authorization|proxy-authorization|cookie|x-homewaf-token|content-length|connection|transfer-encoding)$/i.test(
          key,
        )
      )
        return;
      if (!/^[!#$%&'*+.^_`|~\w-]+$/.test(key)) throw new Error(`第 ${index + 1} 条请求头无效`);
      let text = Array.isArray(value) ? value.join(', ') : String(value);
      if (/^(origin|referer)$/i.test(key)) text = 'service-relative';
      if (/[\r\n]/.test(text)) throw new Error(`第 ${index + 1} 条请求头值无效`);
      headers[key] = text;
    };
    const raw = row.header ?? row.headers ?? {};
    if (typeof raw === 'string' && !raw.trim().startsWith('{')) {
      for (const line of raw.split(/\r?\n/)) {
        if (!line.trim()) continue;
        const colon = line.indexOf(':');
        if (colon < 1) throw new Error(`第 ${index + 1} 条请求头无效`);
        add(line.slice(0, colon).trim(), line.slice(colon + 1).trim());
      }
    } else {
      let object;
      try {
        object = typeof raw === 'string' ? JSON.parse(raw || '{}') : raw;
      } catch {
        throw new Error(`第 ${index + 1} 条请求头 JSON 无效`);
      }
      if (!object || typeof object !== 'object' || Array.isArray(object))
        throw new Error(`第 ${index + 1} 条请求头必须是对象`);
      Object.entries(object).forEach(([key, value]) => add(key, value));
    }
    const body = String(row.body || '');
    if (new TextEncoder().encode(body).length > 65536) throw new Error(`第 ${index + 1} 条正文超过 64 KiB`);
    return {
      id: `log-${index + 1}`,
      category: 'imported',
      kind: 'unknown',
      method: String(row.method || 'GET'),
      path,
      headers,
      body,
      truncated: !!row.truncated,
      imported: true,
    };
  });
}

export const outcomeLabels: Record<string, string> = {
  blocked: '已防御',
  challenge: '要求验证',
  observe: '仅观察',
  passed: '放行',
  incomplete: '样本不完整',
  unavailable: '检测不可用',
};

export const categoryLabels: Record<string,string> = {
  source_boundary:'来源访问限制',source_spoof:'伪造来源',normal:'正常访问',sqli:'SQL 注入',xss:'跨站脚本',lfi:'文件读取与路径穿越',rce:'命令注入',ssti:'模板注入',ssrf:'内网资源探测',scanner:'扫描器与敏感路径',bot:'自动化探测',imported:'导入实录',custom:'自定义样本',
};

export function serviceTarget(config: any): string {
  let host =
    config.host_names?.[0] ||
    config.routes?.flatMap((route: any) => route.hosts || [])[0] ||
    (config.policy.network.listen_ip_version === 'ipv6' ? '::1' : '127.0.0.1');
  if (host.startsWith('*.')) host = 'assessment' + host.slice(1);
  if (host.includes(':')) host = `[${host}]`;
  return `${config.protocol}://${host}:${config.port}`;
}
