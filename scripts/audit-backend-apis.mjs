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
  const result = spawnSync('go', ['test', './internal/management', '-run', '^TestAllBackendRouteAuthenticationBoundaries$', '-count=1'], {
    stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0', HOMEWAF_API_AUDIT_OUTPUT: inventory },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
const routes = JSON.parse(fs.readFileSync(inventory, 'utf8'));
const authentication = Object.fromEntries(
  [...new Set(routes.map((route) => route.auth))].map((auth) => [auth, routes.filter((route) => route.auth === auth).length]),
);
// The JSON inventory is consumed by audit-ui-capabilities.mjs and stays ignored.
console.log(JSON.stringify({
  inventory: path.relative(root, inventory),
  routes: routes.length,
  roleProtected: routes.filter((route) => route.role_guard).length,
  authentication,
}, null, 2));
