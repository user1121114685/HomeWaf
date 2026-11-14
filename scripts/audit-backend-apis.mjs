// Inventory actual production registrations without opening a listening socket.
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
process.chdir(root);
const inventory = path.join(root, '.cache', 'backend-api-routes.json');
fs.mkdirSync(path.dirname(inventory), { recursive: true });
if (!process.argv.includes('--use-inventory')) {
  const result = spawnSync('go', ['test', './wafmangeweb', '-run', '^TestAllBackendRouteAuthenticationBoundaries$', '-count=1'], {
    stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0', HOMEWAF_API_AUDIT_OUTPUT: inventory },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
const routes = JSON.parse(fs.readFileSync(inventory, 'utf8'));
const sources = new Map();
for (const file of fs.readdirSync('api').filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))) {
  const source = fs.readFileSync(path.join('api', file), 'utf8');
  const matches = [...source.matchAll(/func\s+\(\w+\s+\*(\w+)\)\s+(\w+)\(c \*gin.Context\)/g)];
  for (const [index, match] of matches.entries()) {
    const body = source.slice(match.index, matches[index + 1]?.index ?? source.length);
    const line = source.slice(0, match.index).split('\n').length;
    const binding = body.includes('ShouldBindJSON') ? 'JSON' : /ShouldBind|BindQuery/.test(body) ? '参数绑定' : '自定义/无请求体';
    sources.set(`${match[1]}.${match[2]}`, { file, line, binding });
  }
}
const authLabels = { public: '公开', 'download-ticket': '短效下载票据', 'session-only': '仅登录令牌', 'session-or-api-key': '登录令牌/API Key' };
const counts = Object.fromEntries(Object.keys(authLabels).map((auth) => [auth, routes.filter((r) => r.auth === auth).length]));
const lines = [
  '# 后端 API 路由核查清单', '',
  '由 `node scripts/audit-backend-apis.mjs` 从生产 `RegisterRoutes` 生成，包含开启调试时的路由及测试应急入口。没有启动监听端口或调用业务写操作。', '',
  `总计 ${routes.length} 条；登录令牌/API Key ${counts['session-or-api-key']} 条，仅登录令牌 ${counts['session-only']} 条，公开 ${counts.public} 条，下载票据 ${counts['download-ticket']} 条。`, '',
  '“鉴权拒绝”表示已实际请求验证缺失令牌、过期令牌、非法角色均被拦截，不表示该接口全部业务分支均已测试。“角色限制”来自真实中间件链；细分角色和 API Key 的权限语义见审计报告。参数绑定列仅为源码索引，不等同完整输入校验。', '',
  '| 方法 | 路径 | 认证 | 角色限制 | 鉴权拒绝 | 参数入口 | 业务处理器 |',
  '| --- | --- | --- | --- | --- | --- | --- |',
];
for (const route of routes) {
  const match = route.handler.match(/\(\*(\w+)\)\.(\w+)-fm$/);
  const name = match ? `${match[1]}.${match[2]}` : route.handler;
  const source = sources.get(name);
  const location = source ? `[${name}](../../api/${source.file}#L${source.line})` : `\`${name}\``;
  const verified = route.auth.startsWith('session-') ? '通过 × 3' : '单独验证';
  lines.push(`| ${route.method} | \`${route.path}\` | ${authLabels[route.auth]} | ${route.role_guard ? '有' : '无'} | ${verified} | ${source?.binding ?? '特殊入口'} | ${location} |`);
}
const output = 'docs/modernization/backend-api-routes.md';
fs.writeFileSync(output, lines.join('\n') + '\n');
console.log(`Generated ${output}: ${routes.length} routes, ${routes.filter((r) => r.role_guard).length} role-protected routes.`);
