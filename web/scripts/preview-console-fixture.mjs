// Isolated visual-review fixture. Never proxies requests or touches the running WAF.
// Run after npm run build; open http://127.0.0.1:3118/#/dashboard/start.
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { AesEncrypt } from '../src/utils/legacyCipher.ts';
const webRoot = fileURLToPath(new URL('..', import.meta.url));
const dist = path.join(webRoot, 'dist');
const templates = JSON.parse(
  execFileSync('go', ['run', './scripts/pipeline-fixtures'], {
    cwd: path.dirname(webRoot),
    encoding: 'utf8',
    env: { ...process.env, CGO_ENABLED: '0', GOPROXY: 'off', GOSUMDB: 'off' },
  }),
);
const ports =
  process.env.CONSOLE_FIXTURE_EMPTY === '1'
    ? []
    : [
        { port: 5000, name: '家庭相册', upstream: 'http://192.168.1.20:5000', template: 'family-nas', running: true },
        { port: 8080, name: '个人博客', upstream: 'http://192.168.1.30:8080', template: 'public-site', running: false },
      ].map((item) => {
        const tpl = templates.find((template) => template.id === item.template);
        return {
          config: {
            schema_version: 1,
            revision: 1,
            port: item.port,
            name: item.name,
            protocol: 'http',
            upstream: item.upstream,
            listen_enabled: item.running,
            host_names: [],
            policy: tpl.policy,
            template: { id: tpl.id, version: tpl.version },
          },
          listener: { running: item.running },
          stats: { today_hits: 0, today_denied: 0 },
          ban_count: 0,
          honeypot_count: 0,
          locked: false,
        };
      });
const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.svg': 'image/svg+xml',
  '.json': 'application/json',
  '.png': 'image/png',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
};
const server = http.createServer((request, response) => {
  const url = new URL(request.url, 'http://127.0.0.1:3118');
  const send = (data, code = 0) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(
      JSON.stringify({
        code,
        msg: code ? 'Isolated fixture: unsupported operation' : '',
        data: AesEncrypt(JSON.stringify(data)),
      }),
    );
  };
  if (url.pathname.startsWith('/api/')) {
    const endpoint = url.pathname.split('/api/v1/')[1] || '';
    if (endpoint.includes('handshake') || endpoint === 'public/seckey' || endpoint === 'sysinfo/checkversion')
      return send({}, 1);
    if (request.method !== 'GET' && !endpoint.endsWith('/list')) return send({}, 1);
    if (endpoint === 'pipeline/overview') return send({ ports, issues: {}, punishment: {}, log_queue: {} });
    if (endpoint === 'pipeline/templates') return send(templates);
    if (
      [
        'pipeline/migration/legacy',
        'pipeline/regions',
        'pipeline/gate/users',
        'pipeline/gate/sessions',
        'pipeline/state',
        'pipeline/logs',
        'wafhost/ipgroup/options',
      ].includes(endpoint)
    )
      return send([]);
    if (endpoint === 'pipeline/stats') return send({ port: {}, punishment: {}, log_queue: {} });
    if (endpoint === 'threatip/landed/summary') return send([]);
    if (endpoint.endsWith('/list')) return send({ list: [], total: 0 });
    return send({}, 1);
  }
  const requested = path.resolve(dist, '.' + decodeURIComponent(url.pathname));
  if (requested !== dist && !requested.startsWith(dist + path.sep)) {
    response.writeHead(403);
    response.end();
    return;
  }
  const file = url.pathname === '/' ? path.join(dist, 'index.html') : requested;
  if (!fs.existsSync(file) || !fs.statSync(file).isFile()) {
    response.writeHead(404);
    response.end();
    return;
  }
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  if (file.endsWith('index.html')) {
    const bootstrap = `<script>localStorage.setItem('access_token','isolated-visual-fixture');localStorage.setItem('lang','zh_CN');const appearance=new URLSearchParams(location.search).get('appearance');if(['light','dark','auto'].includes(appearance))localStorage.setItem('homewaf.console.preferences',JSON.stringify({mode:appearance}));if(!location.hash)location.hash='/dashboard/start';</script>`;
    response.end(fs.readFileSync(file, 'utf8').replace('<head>', '<head>' + bootstrap));
  } else fs.createReadStream(file).pipe(response);
});
server.on('upgrade', (_request, socket) => socket.destroy());
server.listen(3118, '127.0.0.1', () =>
  console.log('Isolated console visual fixture: http://127.0.0.1:3118 (read-only mock data, no live WAF connection)'),
);
