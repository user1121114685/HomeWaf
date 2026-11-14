export type RuleMode = 'off' | 'observe' | 'block';
export type Policy = Record<string, any>;
export interface UpstreamRoute { hosts: string[]; path_prefix?: string; upstream: string; dial_ip?: string; match_type?: string; priority?: number; strip_prefix?: boolean; target_type?: string; redirect_url?: string; redirect_code?: number; redirect_port?: number; static?: Record<string, any>; header_profile?: string; balance?: string; backends?: Array<{upstream:string;dial_ip?:string;weight:number}> }
export interface LegacySource { code: string; name: string; enabled: boolean }
export interface LegacyPortSummary { port: number; sources: LegacySource[]; imported: boolean; state: string }
export interface LegacyPortPreview extends LegacyPortSummary { config: PortConfig; policy_host: string; preview_token: string; account_names: string[]; requires_totp?: boolean; warnings: string[]; blockers: string[] }
export interface PortConfig {
  schema_version: number; port: number; name: string; protocol: string; listen_enabled: boolean;
  upstream: string; upstream_dial_ip?: string; header_profile?: string; routes?: UpstreamRoute[]; script_groups?: Record<string, string[]>; host_names: string[]; host_code?: string; policy: Policy;
  template: { id?: string; version?: number; applied_at?: number }; revision: number;
}
export interface PolicyTemplate { schema_version: number; id: string; name: string; version: number; builtin: boolean; policy: Policy }
export interface Field { path: string; label: string; help: string; kind?: 'bool' | 'number' | 'lines' | 'json' | 'json-array' | 'select' | 'text'; nullable?: boolean; options?: Array<[string, string]>; min?: number; max?: number }
export const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value));
export function newTOTPSecret(): string { const alphabet='ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'; let bits=0, value=0, output=''; for(const byte of crypto.getRandomValues(new Uint8Array(20))){value=(value<<8)|byte;bits+=8;while(bits>=5){bits-=5;output+=alphabet[(value>>>bits)&31]}value&=(1<<bits)-1}return output; }
export const modes: Array<[RuleMode, string]> = [['off', '关闭'], ['observe', '仅观察'], ['block', '拦截']];
export const sections = ['access', 'ip_group', 'ruleset', 'gate', 'hit_policy', 'honeypot', 'network'];
export const sectionNames: Record<string, string> = { access: '1 · 访问开关', ip_group: '2 · IP 组', ruleset: '3 · 规则集', gate: '3 · 登录门', hit_policy: '4 · 命中限制', honeypot: '4 · 蜜罐', network: '5 · 业务代理' };
export const ruleNames: Record<string, string> = { malformed: '畸形 HTTP', scanner: '扫描器', sqli: 'SQL 注入', xss: '跨站脚本 XSS', upload: '文件上传', lfi: '文件包含 / 路径穿越', rce: '命令注入', java_inject: 'Java 代码注入', java_deser: 'Java 反序列化', php_deser: 'PHP 反序列化', php_inject: 'PHP 代码注入', asp_inject: 'ASP 代码注入', ssti: '模板注入', csrf: 'CSRF 同源校验', ssrf: 'SSRF URL 检测' };
export const ruleHelp: Record<string, string> = {
  malformed: '协议歧义与非法编码；常用 WebDAV 方法默认允许。HTTP 解析器提前拒绝只观察，不归罪前置代理。',
  scanner: '识别明确扫描器特征。探测路径只能填本站确认不存在的路径。',
  sqli: '强 libinjection 命中默认拦截；自然语言和业务参数误报可按参数名排除。',
  xss: '富文本、Markdown 与代码示例可能命中，默认观察；先排除正文参数再收紧。',
  upload: 'NAS 上传 .sh/.exe/.php 也可能是正常业务。默认观察，可限定上传路径和字段。',
  lfi: '敏感文件遍历默认拦截；正常文件选择参数可按路径/参数设置例外。',
  rce: '组合识别命令语境；技术文档与代码提交易误报，默认观察。',
  java_inject: '检测 JNDI、OGNL、SpEL 等高风险 Java 表达式；默认拦截。',
  java_deser: '识别 Java 序列化魔数与已知危险类名，默认拦截。',
  php_deser: '识别 PHP 序列化结构；合法序列化业务先观察。',
  php_inject: 'PHP 标签、危险包装器与调用语境；默认观察。',
  asp_inject: 'ASP 标签及脚本对象组合；默认观察。',
  ssti: '模板表达式可能属于正常文案或开发代码，默认观察。',
  csrf: '校验带 Cookie 的状态变更请求是否同源，不是 CSRF 令牌防护；Webhook/API 需例外。',
  ssrf: '检测参数中的内网/回环/元数据 URL，不发 DNS 请求，也无法防御 DNS 重绑定。',
};
export function newPort(template: PolicyTemplate, port = 5000): PortConfig {
  return { schema_version: 1, port, name: '', protocol: 'http', listen_enabled: false, upstream: '', host_names: [], policy: clone(template.policy), template: { id: template.id, version: template.version }, revision: 0 };
}
export function observationConfig(config: PortConfig): PortConfig {
  const next = clone(config); next.policy.hit_policy.enabled = false; next.policy.hit_policy.lockdown.enabled = false;
  next.policy.honeypot.enabled = false; next.policy.ruleset.qps.mode = 'observe'; next.policy.ruleset.cc.mode = 'observe'; next.policy.ruleset.bot = 'observe';
  if (next.policy.ruleset.owasp !== 'off') next.policy.ruleset.owasp = 'observe';
  if (next.policy.ruleset.ai && next.policy.ruleset.ai !== 'off') next.policy.ruleset.ai = 'observe';
  for (const option of [next.policy.ruleset.anti_leech, next.policy.ruleset.challenge, next.policy.ruleset.sensitive, next.policy.ruleset.plugins]) if (option?.mode && option.mode !== 'off') option.mode = 'observe';
  if(next.policy.ruleset.callback?.mode && next.policy.ruleset.callback.mode!=='off')next.policy.ruleset.callback.mode='observe';
  for (const rule of [...(next.policy.ruleset.scripts || []), ...(next.policy.ruleset.urls || [])]) if (rule.mode !== 'off') rule.mode = 'observe';
  for (const rule of next.policy.ruleset.rules || []) if (rule.mode !== 'off') rule.mode = 'observe';
  for (const rule of next.policy.network.response?.tamper || []) if (rule.mode !== 'off') rule.mode = 'observe';
  if(next.policy.network.response?.privacy?.mode && next.policy.network.response.privacy.mode!=='off')next.policy.network.response.privacy.mode='observe';
  return next;
}
export function publicTemplate(config: PortConfig, name: string): PolicyTemplate {
  const policy = clone(config.policy); policy.ip_group.entries = []; policy.network.trusted_proxies = [];
  policy.ip_group.allow_groups = []; policy.ip_group.deny_groups = [];
  policy.gate.exempt_ips=[];
  if(policy.ruleset.callback)policy.ruleset.callback.networks=[];
  for (const rule of policy.network.response?.tamper || []) rule.learned_at=0;
  return { schema_version: 1, id: 'user-' + config.port, name, version: 1, builtin: false, policy };
}
export function parseLines(value: string): string[] { return value.split(/[\r\n,]+/).map(v => v.trim()).filter(Boolean); }
export function valueAt(object: any, path: string): any { return path.split('.').reduce((v, key) => v?.[key], object); }
export function setAt(object: any, path: string, value: any): void {
  const keys = path.split('.'); let parent = object;
  for (const key of keys.slice(0, -1)) { if (!parent[key]) parent[key] = {}; parent = parent[key]; }
  parent[keys[keys.length - 1]] = value;
}
export async function mapBounded<T, R>(items: T[], work: (item: T) => Promise<R>, concurrency = 4): Promise<R[]> {
  const out: R[] = new Array(items.length); let index = 0;
  await Promise.all(Array.from({ length: Math.min(concurrency, items.length) }, async () => { while (index < items.length) { const at = index++; out[at] = await work(items[at]); } }));
  return out;
}
export const fields: Record<string, Field[]> = {
  access: [
    { path: 'enabled', label: '允许业务访问', kind: 'bool', help: '关闭后拒绝所有业务请求，包括 trusted；管理端口不受影响。保存前会再次确认。' },
    { path: 'disabled_action', label: '关闭时响应', kind: 'select', options: [['deny', '503 页面'], ['drop', '直接断开']], help: '仅影响该端口。' },
    { path: 'drain_seconds', label: '已有连接排空时间（秒）', kind: 'number', min: 0, max: 300, help: '关闭访问后，WebSocket/下载等已有连接最多保留此时长。' },
  ],
  ip_group: [
    { path: 'unlisted_action', label: '未命中名单的来源', kind: 'select', options: [['continue', '继续检测'], ['deny', '拒绝（仅白名单可访问）'], ['honeypot', '导入蜜罐']], help: '手动 deny 优先；trusted 可免检测；区域白名单只允许继续检测。' },
    { path: 'feed_enabled', label: '使用签名威胁订阅', kind: 'bool', help: '必须先配置 HTTPS 签名源及有效授权；默认不联网。trusted/受保护地址不因订阅命中自动封禁。' },
  ],
  ruleset: [
    { path: 'ai', label: '本地AI异常评分', kind: 'select', options: [['', '关闭'], ['off', '关闭'], ['observe', '仅观察'], ['block', '拦截']], help: '复用本地模型，无联网请求；登录与常规攻击检测后执行。模型未加载或预算不足时放行并记录。' },
    { path: 'ai_scope', label: '本地AI适用范围', kind: 'json', help: 'hosts、paths、methods及exclude_paths；用于排除易误报业务。正文只检查前32KiB。' },
    { path: 'qps.mode', label: 'QPS 硬上限', kind: 'select', options: modes, help: '控制每 IP / 每端口 / 路径的突发请求；观察模式只记录。' },
    { path: 'qps.per_ip', label: '每 IP 每秒请求', kind: 'number', min: 1, max: 100000, help: '普通页面会并发加载多份资源，家庭 NAS 建议先使用模板默认值。' },
    { path: 'qps.per_port', label: '整个端口每秒请求', kind: 'number', min: 1, max: 1000000, help: '端口间分别计数。' },
    { path: 'qps.burst', label: '允许突发请求数', kind: 'number', min: 1, max: 100000, help: '给页面同时加载静态资源保留余量。' },
    { path: 'qps.static_multiplier', label: '静态资源倍率', kind: 'number', min: 1, max: 100, help: '静态资源可使用更宽松额度。' },
    { path: 'qps.paths', label: '路径限速映射（JSON）', kind: 'json', help: '例如 {"/api":10}；路径单独设每秒上限。留空对象表示无额外限制。' },
    { path: 'cc.mode', label: 'CC 异常频率识别', kind: 'select', options: modes, help: '与 QPS 硬上限分开；初次启用建议观察。' },
    { path: 'cc.window_seconds', label: 'CC 窗口（秒）', kind: 'number', min: 1, max: 3600, help: '滑动窗口内累计请求。' },
    { path: 'cc.threshold', label: 'CC 阈值', kind: 'number', min: 1, max: 1000000, help: '达到阈值后执行下方动作。' },
    { path: 'cc.action', label: 'CC 动作', kind: 'select', options: [['challenge', '轻量挑战'], ['deny', '拒绝 429']], help: 'Cookie 往返挑战仅减轻简单脚本，不是强人机识别。' },
    { path: 'cc.by_path', label: '按路径区分 CC', kind: 'bool', help: '同 IP 访问不同业务路径分开计数。' },
    { path: 'cc.exclude_static', label: 'CC 忽略静态资源', kind: 'bool', help: '减少页面资源突发造成的误报。' },
    { path: 'cc.challenge_seconds', label: '挑战有效期（秒）', kind: 'number', min: 1, max: 3600, help: '通过挑战后仍须经过 QPS、登录门与攻击检测。' },
    { path: 'bot', label: 'BOT 检测', kind: 'select', options: modes, help: '伪造好 BOT 的 UA 不会自动成为可信来源。' },
    { path: 'bot_verify_dns', label: '允许 BOT DNS 验证', kind: 'bool', help: '显式开启才发 DNS 请求；异步验证不会阻塞请求。' },
    { path: 'allowed_methods', label: '允许的方法（每行一项）', kind: 'lines', help: 'WebDAV 请保留 PROPFIND/MKCOL/MOVE/COPY/LOCK/UNLOCK。' },
    { path: 'budget_ms', label: '检测 CPU 预算（毫秒）', kind: 'number', min: 1, max: 1000, help: '检测器异常或预算耗尽时放行并记录，避免自身故障影响家人。正文读取另有超时。' },
    { path: 'view.body_bytes', label: '正文检测上限（字节）', kind: 'number', min: 1024, max: 16777216, help: '只检查有限前缀；原始尾部仍完整转发，NAS 大上传不被截断。' },
    { path: 'view.expansion_ratio', label: '最大解压膨胀比', kind: 'number', min: 1, max: 100, help: '限制 gzip/deflate/br/zstd 解压资源。' },
    { path: 'view.max_fields', label: '最多检测字段', kind: 'number', min: 1, max: 10000, help: '超过配额标记视图不完整，原始请求仍保持完整。' },
    { path: 'view.file_prefix_bytes', label: '上传文件头采样（字节）', kind: 'number', min: 1, max: 1048576, help: '检查 multipart 的文件名、类型与有限内容前缀。' },
    { path: 'csrf_origins', label: 'CSRF 允许来源（每行一项）', kind: 'lines', help: '仅填写明确可信的源，例如 https://panel.example.test；路径例外在 CSRF 规则范围中设置。' },
    { path: 'csrf_missing_origin', label: '缺少 Origin / Referer', kind: 'select', options: [['observe', '仅观察'], ['allow', '允许'], ['block', '按规则模式拒绝']], help: '原生 App/Webhook 常无来源头，建议观察或配置路径例外。' },
    { path: 'csrf_options', label: '迁入CSRF选项', kind: 'json', nullable: true, help: '可选旧策略：is_enable、protect_methods、allowed_origins、allow_empty_ref、exclude_paths；保留方法和例外，即使无Cookie也按旧策略检查。清空为null使用原生默认策略。' },
    { path: 'upload_options', label: '上传检测选项', kind: 'json', nullable: true, help: '可选：is_enable、check_ext、ext_blacklist、check_content、check_magic、check_size、max_size_kb、over_limit_action、include_paths、exclude_paths。检查副本仍受正文预算限制；超限按block/pass处理，不截断转发。' },
    { path: 'anti_leech', label: '防盗链策略', kind: 'json', nullable: true, help: 'mode为off/observe/block；settings含is_enable_anti_leech、file_types、valid_referers、action、redirect_url。引用来源支持none、blocked、server_names、通配域名和~RE2表达式。' },
    { path: 'callback', label: '明确的回调攻击检测例外', kind: 'json', nullable: true, help: 'mode=block表示启用例外，observe只记录，off关闭；networks填写核对过的IP/CIDR，scope.paths必须指定回调路径。不会绕过黑名单或登录门，业务仍须校验签名；旧源码IP表默认关闭，不代表当前服务身份。' },
    { path: 'challenge', label: '访问挑战 / 紧急模式', kind: 'json', help: 'mode为off/observe/block，until为Unix到期秒（0不限），scope设置路径例外；使用端口独立签名Cookie挑战，未通过不回源。' },
    { path: 'sensitive', label: '敏感词策略', kind: 'json', nullable: true, help: 'mode为off/observe/block；replacement是替换文本；rules包含content、check_direction(in/out/all)、action(deny/replace)。每端口5000条/256KiB，deny优先；观察不改写，超限及流式响应保持原流并记录跳过原因。' },
    { path: 'owasp', label: 'OWASP CRS', kind: 'select', options: modes, help: '复用本地安装的 CRS；比原生轻量规则更耗资源，默认关闭。范围可在高级 JSON 中设置。' },
  ],
  gate: [
    { path: 'enabled', label: '启用前置登录门', kind: 'bool', help: '未认证请求不会回源。启用前至少要有一个有效账号。' },
    { path: 'require_token', label: '必须使用服务令牌', kind: 'bool', help: 'API/Webhook可只用令牌，不启用账号登录门。先关闭上面的账号登录门，配置专用头并生成令牌；无令牌不能开始监听。' },
    { path: 'mode', label: '认证方式', kind: 'select', options: [['form', '表单登录'], ['basic', 'Basic 浏览器弹窗']], help: 'Basic 会占用 Authorization，与业务自身 Basic 冲突；WAF 凭据会在回源前剥离。' },
    { path: 'title', label: '登录页标题', help: '页面离线可用，适配手机和深色模式；不显示上游信息。' },
    { path: 'hint', label: '登录提示', help: '不要放置内部地址或敏感信息。' },
    { path: 'exempt_ips', label: '只免登录的IP/CIDR', kind: 'lines', help: '只跳过登录门，不跳过IP黑名单与攻击检测；从旧SSO/IP组迁入后独立保存。' },
    { path: 'protect', label: '仅对指定范围要求登录', kind: 'json', help: '留空对象表示全端口；可指定hosts、paths或methods。范围外免登录，其他防护仍执行。' },
    { path: 'prefix', label: '登录保留路径', help: '默认 /__homewaf/gate；该前缀由 WAF 独占。' },
    { path: 'bypass_trusted', label: 'trusted 免登录', kind: 'bool', help: '关闭后 trusted 仍跳过攻击规则，但必须登录。' },
    { path: 'idle_seconds', label: '空闲超时（秒）', kind: 'number', min: 0, max: 2592000, help: '默认 7200；0 表示不设空闲超时。' },
    { path: 'absolute_seconds', label: '绝对会话时长（秒）', kind: 'number', min: 60, max: 2592000, help: '默认 86400；到期需要重新登录。' },
    { path: 'remember_days', label: '记住我最长天数', kind: 'number', min: 0, max: 30, help: '0 为禁用；最长 30 天。' },
    { path: 'bind_ip', label: '会话绑定 IP', kind: 'bool', help: '移动网络可能更换出口地址，家庭默认关闭。' },
    { path: 'max_sessions', label: '单账号并发会话', kind: 'number', min: 0, max: 1000, help: '0 不单独限制，仍受系统总额度保护。' },
    { path: 'login_per_minute', label: '每 IP 每分钟登录尝试', kind: 'number', min: 1, max: 100, help: '独立于 QPS / CC，默认 10 次；账号只采用软延迟，不永久硬锁。' },
    { path: 'allow_options', label: 'OPTIONS 预检免登录', kind: 'bool', help: '仅在业务需要跨域预检时开启。' },
    { path: 'exempt.paths', label: '免登录路径', kind: 'lines', help: '每行一个路径前缀。适用于公开分享、Webhook、App API；仍进行后续攻击检测。' },
    { path: 'exempt.methods', label: '免登录方法范围', kind: 'lines', help: '留空不限制方法；与免登录路径一起匹配。' },
    { path: 'token_header', label: '预共享令牌头', help: '推荐X-HomeWaf-Token；可迁入旧SSO专用X-头，但不能使用转发地址或内部控制头。每端口独立，令牌只显示一次，回源前剥离。' },
  ],
  hit_policy: [
    { path: 'enabled', label: '自动处罚', kind: 'bool', help: '只影响后续请求；observe、trusted、私网、回环、本机和管理员来源不处罚。' },
    { path: 'ban_scope', label: '自动封禁作用域', kind: 'select', options: [['global', '共享至所有端口'], ['port', '仅当前端口']], help: '其他端口的 trusted 不受共享封禁影响。手动黑名单始终只属于本端口。' },
    { path: 'ipv6_prefix', label: 'IPv6 聚合前缀长度', kind: 'number', min: 32, max: 128, help: '默认 /64，减少同一来源轮换 IPv6 地址绕过计数。' },
    { path: 'max_keys', label: '端口计数键额度', kind: 'number', min: 100, max: 1000, help: '有效上限 1000，总 2 万；满时只淘汰本端口的旧计数。' },
    { path: 'response_signals', label: '启用业务响应信号', kind: 'bool', help: '首次无凭据401不计；全站5xx高发只观察，不归罪个人。业务登录须在高级 JSON 中明确配置字段和成功判定。' },
    { path: 'ignore_paths', label: '响应信号忽略路径', kind: 'lines', help: '每行一个前缀；默认忽略 favicon、robots，另自动忽略 .map。' },
    { path: 'lockdown.enabled', label: '多来源自保锁定', kind: 'bool', help: '攻击者可故意触发，让非trusted正常用户暂时不能访问；仅锁本端口，管理端与trusted仍可用。' },
    { path: 'lockdown.window_seconds', label: '锁定统计窗口（秒）', kind: 'number', min: 1, max: 3600, help: '只统计非观察、high以上的不同来源。' },
    { path: 'lockdown.distinct_ips', label: '触发锁定的不同 IP 数', kind: 'number', min: 2, max: 1000, help: '默认 60 秒内至少 5 个来源；IPv6 按设置的前缀聚合。' },
    { path: 'lockdown.seconds', label: '锁定时长（秒）', kind: 'number', min: 1, max: 86400, help: '默认 600；最后一次触发后无新触发才自动解除。' },
    { path: 'lockdown.cooldown_seconds', label: '解除后冷却（秒）', kind: 'number', min: 0, max: 86400, help: '避免管理员解除后立即再次锁定。' },
    { path: 'lockdown.action', label: '锁定响应', kind: 'select', options: [['deny', '503 页面'], ['drop', '直接断开']], help: '所有其他端口不受影响。' },
  ],
  honeypot: [
    { path: 'enabled', label: '启用内置蜜罐', kind: 'bool', help: '仿真登录页不会触达真实上游；合法用户误入可在处罚列表一键释放。' },
    { path: 'ttl_seconds', label: '蜜罐持续时间（秒）', kind: 'number', min: 1, max: 86400, help: '默认 3600；到期恢复。蜜罐内提交登录或触碰陷阱会升级封禁。' },
    { path: 'auth_fail_threshold', label: '5分钟登录失败阈值', kind: 'number', min: 1, max: 10000, help: '默认 5 次导入蜜罐；持续失败另由登录封禁阈值处理。' },
    { path: 'weak_password_threshold', label: '5分钟弱口令尝试阈值', kind: 'number', min: 1, max: 10000, help: '默认 3 次；仅记录命中与长度/HMAC，不记录明文密码。' },
    { path: 'max_concurrent', label: '蜜罐并发', kind: 'number', min: 1, max: 32, help: '总额度64、单端口有效最多32，避免延迟耗尽工作资源。' },
    { path: 'max_delay_ms', label: '随机延迟上限（毫秒）', kind: 'number', min: 0, max: 2000, help: '0为不延迟；延迟不阻塞其他端口。' },
    { path: 'trap_paths', label: '确认不存在的陷阱路径', kind: 'lines', help: '仅填本站绝不存在的路径；真实 /wp-login.php 等业务路径不可填入。' },
  ],
  network: [
    { path: 'listen_ip_version', label: '监听IP版本', kind: 'select', options: [['', '双栈（默认）'], ['both', 'IPv4 + IPv6'], ['ipv4', '仅IPv4'], ['ipv6', '仅IPv6']], help: '变更前须停止监听；迁入保留原有IP监听范围。' },
    { path: 'trusted_proxies', label: '可信前置代理 CIDR', kind: 'lines', help: '不配置就只使用 TCP 对端；只信任明确的代理网段，不能填全网。' },
    { path: 'real_ip_header', label: '真实 IP 头', help: '默认 X-Forwarded-For，从右到左取第一个不可信地址；只有可信代理能提供。' },
    { path: 'preserve_host', label: '保留客户端 Host 回源', kind: 'bool', help: '默认使用上游主机名；业务依赖客户端 Host 时开启。' },
    { path: 'max_proxy_hops', label: '代理最大跳数', kind: 'number', min: 0, max: 64, help: '默认10；超过返回508并记录观察事件，不处罚访问者。0关闭；回源后递增hop标记，终止配置错误造成的代理循环。' },
    { path: 'disable_http2', label: '关闭对外HTTP/2', kind: 'bool', help: '兼容仅支持HTTP/1.1的设备；更改前先停止该端口。证书更新可直接热生效，不需要停监听。' },
    { path: 'response', label: '响应Cookie与压缩', kind: 'json', help: 'cookie沿用is_enable、http_only、secure、same_site、exclude_cookies；compression沿用is_enable、prefer、min_length及类型/路径范围；default_encoding指定旧页面字符集。内容变换仅处理完整且不超过1MiB的响应，流式/Range不缓冲。' },
    { path: 'max_header_bytes', label: '最大请求头（字节）', kind: 'number', min: 4096, max: 1048576, help: '默认 32768；会影响较大 Cookie 与认证头。' },
    { path: 'read_header_seconds', label: '读取请求头超时（秒）', kind: 'number', min: 1, max: 60, help: '限制慢连接；不作为整段上传/下载的总超时。' },
    { path: 'connect_seconds', label: '上游连接超时（秒）', kind: 'number', min: 0, max: 120, help: '0使用默认5秒。' },
    { path: 'response_header_seconds', label: '上游响应头超时（秒）', kind: 'number', min: 0, max: 300, help: '0使用默认30秒；正文保持流式。' },
    { path: 'max_request_bytes', label: '请求总大小上限（字节）', kind: 'number', min: 0, help: '0为不限；与检测前缀上限不同。NAS大文件通常保持0。' },
    { path: 'max_in_flight', label: '端口同时处理请求', kind: 'number', min: 0, max: 128, help: '0使用默认32；WS/SSE和长下载也占用额度，总额度256。' },
  ],
};
