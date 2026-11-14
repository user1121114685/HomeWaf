import type { PolicyTemplate, PortConfig } from './model';

export const serviceScenes = [
  {
    id: 'family-nas',
    name: 'NAS 与家庭相册',
    short: '家庭 NAS',
    icon: 'FolderOpened',
    description: '文件、相册和私有网盘，可选登录或来源限制。',
    detail: '保留大文件与 WebDAV 支持，可按需添加账号、地区或 IP 限制。',
  },
  {
    id: 'public-site',
    name: '公开网站',
    short: '公开网站',
    icon: 'Monitor',
    description: '博客、主页与公开站点，访客直接访问。',
    detail: '使用公开站点模板，识别异常请求与自动化访问。',
  },
  {
    id: 'family-device',
    name: '摄像头与设备',
    short: '家庭设备',
    icon: 'VideoCamera',
    description: '设备的网页管理入口，限制陌生人访问。',
    detail: '使用设备场景的请求频率设置，额外访问限制按需选择。',
  },
  {
    id: 'api-webhook',
    name: 'API 与自动化',
    short: 'API 服务',
    icon: 'Connection',
    description: '程序调用和回调，使用服务令牌认证。',
    detail: '保存后生成服务令牌，再启用并配置调用端。',
  },
];

export function suggestPort(used: number[], start = 5000): number {
  const taken = new Set(used);
  for (let port = start; port <= 65535; port++) if (!taken.has(port)) return port;
  for (let port = 1024; port < start; port++) if (!taken.has(port)) return port;
  throw new Error('没有可推荐的端口，请先释放一个端口。');
}

export function normalizeServiceAddress(input: string): string {
  const value = input.trim();
  if (!value) throw new Error('填写现有服务地址，例如 192.168.1.20:5000。');
  const candidate = /^[a-z][a-z\d+.-]*:\/\//i.test(value) ? value : `http://${value}`;
  let url: URL;
  try {
    url = new URL(candidate);
  } catch {
    throw new Error('地址格式不正确，请填写 IP 或域名及端口。');
  }
  if (!['http:', 'https:'].includes(url.protocol)) throw new Error('服务地址只支持 HTTP 或 HTTPS。');
  if (!url.hostname || url.username || url.password) throw new Error('地址中不能包含账号密码，请只填写服务地址。');
  if (url.hash || url.search || (url.pathname && url.pathname !== '/'))
    throw new Error('请填写服务的根地址，去掉路径、查询参数和 # 后的内容。');
  return url.origin;
}

export function suggestedName(templateID: string, upstream: string): string {
  const scene = serviceScenes.find((item) => item.id === templateID);
  const host = new URL(normalizeServiceAddress(upstream)).hostname;
  return `${scene?.short || '服务'} · ${host}`.slice(0, 100);
}

export function optionalAccessDefaults(config: PortConfig): void {
  // New NAS/device connections offer authentication rather than require it.
  // Copied configurations and explicitly selected custom templates keep their policy.
  if (['family-nas', 'family-device'].includes(config.template?.id || '')) config.policy.gate.enabled = false;
  config.policy.network.listen_ip_version ||= 'both';
}

export function parseAllowedSources(input: string): string[] {
  const values = input
    .split(/[\r\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
  return [
    ...new Set(
      values.map((value) => {
        const parts = value.split('/');
        if (parts.length > 2 || !parts[0]) throw new Error(`IP / 网段格式不正确：${value}`);
        let address = parts[0],
          max = 32;
        if (address.includes(':')) {
          max = 128;
          try {
            const parsed = new URL(`http://[${address}]/`);
            address = parsed.hostname.slice(1, -1);
          } catch {
            throw new Error(`IPv6 地址格式不正确：${value}`);
          }
        } else if (
          !/^\d{1,3}(\.\d{1,3}){3}$/.test(address) ||
          address.split('.').some((part) => Number(part) > 255 || String(Number(part)) !== part)
        ) {
          throw new Error(`IPv4 地址格式不正确：${value}`);
        }
        if (parts.length === 2 && (!/^\d+$/.test(parts[1]) || Number(parts[1]) > max))
          throw new Error(`网段前缀应在 0–${max} 之间：${value}`);
        return address + (parts.length === 2 ? `/${Number(parts[1])}` : '');
      }),
    ),
  ];
}

export function templateDescription(template: PolicyTemplate): string {
  return (
    serviceScenes.find((scene) => scene.id === template.id)?.detail ||
    (template.policy.gate.require_token
      ? '调用方需要服务令牌。'
      : template.policy.gate.enabled
        ? '访客需要使用独立账号登录。'
        : '使用此模板的访问限制与检测规则。')
  );
}

export function serviceStatus(row: {
  config: PortConfig;
  listener?: { running?: boolean; error?: string };
  locked?: boolean;
}) {
  if (row.listener?.error) return { label: '启动异常', tone: 'danger' };
  if (row.locked) return { label: '临时锁定', tone: 'warning' };
  if (!row.config.listen_enabled) return { label: '待启用', tone: 'info' };
  if (!row.listener?.running) return { label: '未监听', tone: 'warning' };
  if (!row.config.policy.access.enabled) return { label: '访问已关闭', tone: 'warning' };
  return { label: '运行中', tone: 'success' };
}
