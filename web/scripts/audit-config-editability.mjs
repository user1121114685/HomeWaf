// Structural inventory, not a substitute for browser/persistence regression.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse as parseSFC } from '@vue/compiler-sfc';
import { baseParse } from '@vue/compiler-dom';

const root = fileURLToPath(new URL('../..', import.meta.url));
const domains = ['pipeline','access_account','access_config','account','ai','anticc','application','bLocking_page','batchtask','cache_rule','ca_server','cdnip','data_retention','firewall_ipblock','host','hostguard','http_auth_base','ipallow','ipblock','ipgroup','iplocation','ldpurl','loadbalance','logfilewrite','notify_channel','notify_subscription','onekeymod','oplatform','otp','owasp','path_rule','private_group','private_info','rule','rule_ban','sensitive','sslconfig','ssl_expire','systemconfig','task','threatip','tunnel','urlallow','urlblock','vpconfig'];
const pages = [];
for (const domain of domains) {
  const relative = `web/src/pages/waf/${domain}/index.vue`, file = path.join(root, relative);
  if (!fs.existsSync(file)) continue;
  const sourceFiles = [file];
  function collect(directory) {
    if (!fs.existsSync(directory)) return;
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const name = path.join(directory, entry.name);
      if (entry.isDirectory()) collect(name);
      else if (entry.name.endsWith('.vue')) sourceFiles.push(name);
    }
  }
  collect(path.join(path.dirname(file), 'components'));
  const source = sourceFiles.map(file => fs.readFileSync(file, 'utf8')).join('\n');
  const handlers = new Set(), forms = new Set();
  let modelBindings = 0;
  function walk(node) {
    if (node.type === 1) for (const prop of node.props || []) {
      if (prop.type !== 7) continue;
      if (prop.name === 'model') modelBindings++;
      if (prop.name === 'on' && prop.exp?.content) {
        const expression = prop.exp.content;
        if (/edit|update|save|config|setting|modify|manage|submit|upload|reloadModel|unload/i.test(expression)) handlers.add(expression);
        if (prop.arg?.content === 'submit') forms.add(expression);
      }
    }
    for (const child of node.children || []) walk(child);
  }
  for (const file of sourceFiles) {
    const { descriptor } = parseSFC(fs.readFileSync(file, 'utf8'));
    walk(baseParse(descriptor.template?.content || ''));
  }
  const apiReferences = [...new Set(source.match(/\b\w*(?:Edit|Update|Save)\w*(?:Api|_api)\b|\b(?:edit|update|save)[A-Z]\w*/g) || [])];
  pages.push({ page: domain, file: relative, sourceFiles: sourceFiles.map(file => path.relative(root, file).replaceAll('\\', '/')), modelBindings, editOrSaveHandlers: [...handlers], submitHandlers: [...forms], updateReferences: apiReferences, note: domain === 'host' ? 'Legacy source page; migrated ports are edited in pipeline.' : domain === 'pipeline' ? 'Includes port editor, honeypot, accounts, templates and feed dialogs.' : domain === 'ai' ? 'Models support upload/reload/unload; AI scoring modes are editable in system and port policies.' : '' });
}
const result = { scope: 'Configuration-page entry and update wiring; does not claim every form was written through a real running database.', configurationPages: pages.length, withEditOrSaveHandler: pages.filter(page => page.editOrSaveHandlers.length).length, needsInspection: pages.filter(page => !page.editOrSaveHandlers.length).map(page => page.page), pages };
const output = path.join(root, 'artifacts', 'config-editability');
fs.mkdirSync(output, { recursive: true });
fs.writeFileSync(path.join(output, 'configuration-inventory.json'), JSON.stringify(result, null, 2));
console.log(JSON.stringify({ configurationPages: result.configurationPages, withEditOrSaveHandler: result.withEditOrSaveHandler, needsInspection: result.needsInspection }, null, 2));
