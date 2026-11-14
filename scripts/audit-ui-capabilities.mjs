// Static navigation/API inventory. References are review clues, not proof of a complete user flow.
// Run scripts/audit-backend-apis.mjs first; results stay in ignored artifacts.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire(path.resolve('web/package.json'));
const { parse: parseSfc } = require('@vue/compiler-sfc');
const { parse } = require('@babel/parser');
const routes = JSON.parse(fs.readFileSync('.cache/backend-api-routes.json', 'utf8'));
const roots = ['web/src/main.js', 'web/src/router/index.js'];
const outputDir = 'artifacts/ui-capabilities';
fs.mkdirSync(outputDir, { recursive: true });
const { parse: parseTemplate } = require('@vue/compiler-dom');
const files = new Map(),
  refs = [];
function resolve(from, target) {
  let file = target.startsWith('@/')
    ? 'web/src/' + target.slice(2)
    : target.startsWith('.')
      ? path.posix.normalize(path.posix.join(path.posix.dirname(from), target))
      : null;
  if (!file) return null;
  for (const candidate of [file, file + '.ts', file + '.js', file + '/index.ts', file + '/index.js', file + '.vue'])
    if (fs.existsSync(candidate) && fs.statSync(candidate).isFile() && /\.(vue|ts|js)$/.test(candidate))
      return candidate;
  return null;
}
const normalize = (value) =>
  '/api/v1/' +
  value
    .replace(/^\/api\/v1\//, '')
    .replace(/^\//, '')
    .split('?')[0];
function scan(file, parent) {
  if (files.has(file)) {
    files.get(file).parents.add(parent);
    return;
  }
  const source = fs.readFileSync(file, 'utf8');
  let code = source;
  if (file.endsWith('.vue')) {
    const d = parseSfc(source).descriptor;
    code = (d.script?.content || '') + '\n' + (d.scriptSetup?.content || '');
  }
  let ast;
  try {
    ast = parse(code, { sourceType: 'unambiguous', plugins: ['typescript'] });
  } catch (error) {
    throw new Error(file + ': ' + error.message);
  }
  const record = { source: code, parents: new Set([parent]), imports: [] };
  files.set(file, record);
  const constants = new Map();
  function values(n) {
    if (!n) return [];
    if (n.type === 'StringLiteral') return [n.value];
    if (n.type === 'Identifier') return constants.get(n.name) || [];
    if (n.type === 'ConditionalExpression') return [...values(n.consequent), ...values(n.alternate)];
    if (n.type === 'BinaryExpression' && n.operator === '+') {
      const a = values(n.left),
        b = values(n.right);
      return a.flatMap((x) => (b.length ? b : [':dynamic']).map((y) => x + y));
    }
    if (n.type === 'TemplateLiteral') {
      let out = [''];
      for (let i = 0; i < n.quasis.length; i++) {
        out = out.map((x) => x + n.quasis[i].value.cooked);
        if (i < n.expressions.length) {
          const part = values(n.expressions[i]);
          out = out.flatMap((x) => (part.length ? part : [':dynamic']).map((y) => x + y));
        }
      }
      return out;
    }
    return [];
  }
  function constantsWalk(n) {
    if (!n || typeof n !== 'object') return;
    if (n.type === 'VariableDeclarator' && n.id?.type === 'Identifier') {
      const v = values(n.init);
      if (v.length) constants.set(n.id.name, v);
    }
    for (const [k, v] of Object.entries(n)) {
      if (['loc', 'comments', 'tokens'].includes(k)) continue;
      if (Array.isArray(v)) v.forEach(constantsWalk);
      else if (v && typeof v === 'object') constantsWalk(v);
    }
  }
  constantsWalk(ast);
  function walk(node, fn = '') {
    if (node?.type === 'ImportExpression' || (node?.type === 'CallExpression' && node.callee?.type === 'Import')) {
      const target = resolve(file, node.source?.value || node.arguments?.[0]?.value || '');
      if (target) {
        record.imports.push({ file: target, specifiers: [] });
        scan(target, file);
      }
    }
    if (!node || typeof node !== 'object') return;
    if (node.type === 'FunctionDeclaration') fn = node.id?.name || fn;
    if (
      node.type === 'VariableDeclarator' &&
      ['ArrowFunctionExpression', 'FunctionExpression'].includes(node.init?.type)
    )
      fn = node.id?.name || fn;
    if (node.type === 'ImportDeclaration') {
      const target = resolve(file, node.source.value);
      if (target) {
        record.imports.push({
          file: target,
          specifiers: node.specifiers.map((s) => ({
            local: s.local?.name,
            imported: s.imported?.name || 'default',
          })),
        });
        scan(target, file);
      }
    }
    if (node.type === 'ObjectExpression') {
      const property = (name) =>
        node.properties.find((p) => p.type === 'ObjectProperty' && (p.key.name || p.key.value) === name)?.value;
      const url = property('url'),
        method = property('method');
      for (const value of values(url))
        refs.push({
          file,
          fn,
          path: normalize(value),
          method: method?.value?.toUpperCase() || 'GET',
          kind: 'request',
        });
    }
    if (
      node.type === 'CallExpression' &&
      node.callee.type === 'MemberExpression' &&
      ['get', 'post', 'put', 'patch', 'delete'].includes(node.callee.property.name)
    ) {
      for (const value of values(node.arguments[0]))
        if (value.startsWith('/') || value.includes('/'))
          refs.push({
            file,
            fn,
            path: normalize(value),
            method: node.callee.property.name.toUpperCase(),
            kind: 'member-request',
          });
    }
    if (
      node.type === 'CallExpression' &&
      ['pipelineGet', 'pipelinePost'].includes(node.callee.name) &&
      node.arguments[0]?.type === 'StringLiteral'
    )
      refs.push({
        file,
        fn,
        path: normalize('pipeline/' + node.arguments[0].value),
        method: node.callee.name === 'pipelinePost' ? 'POST' : 'GET',
        kind: 'pipeline',
      });
    if (node.type === 'CallExpression' && node.callee.name === 'buildUploadConfig')
      for (const value of values(node.arguments[0]))
        refs.push({
          file,
          fn,
          path: normalize(value),
          method: 'POST',
          kind: 'upload',
        });
    for (const [key, value] of Object.entries(node)) {
      if (['loc', 'start', 'end', 'comments', 'tokens'].includes(key)) continue;
      if (Array.isArray(value)) for (const child of value) walk(child, fn);
      else if (value && typeof value === 'object') walk(value, fn);
    }
  }
  walk(ast);
}
for (const root of roots) scan(root, 'route');
const matches = (left, right) =>
  left === right ||
  (left.split('/').length === right.split('/').length &&
    left
      .split('/')
      .every(
        (part, i) => part === right.split('/')[i] || (part === ':dynamic' && right.split('/')[i].startsWith(':')),
      ));
const output = routes.map((route) => {
  const hits = refs
    .filter((r) => r.method === route.method && matches(r.path, route.path))
    .map((hit) => {
      const consumers = [];
      if (hit.fn)
        for (const [file, record] of files) {
          if (file === hit.file) continue;
          for (const imp of record.imports.filter((i) => i.file === hit.file))
            for (const spec of imp.specifiers.filter((s) => s.imported === hit.fn)) {
              const re = new RegExp('\\b' + spec.local + '\\s*\\(');
              consumers.push({ file, directCall: re.test(record.source) });
            }
        }
      return { ...hit, consumers };
    });
  const direct = hits.some(
    (h) => h.kind !== 'request' || !h.file.startsWith('web/src/apis/') || h.consumers.some((c) => c.directCall),
  );
  return {
    ...route,
    refs: hits,
    status: direct ? 'reachable-reference' : hits.length ? 'client-import-review' : 'no-static-reference',
  };
});
fs.writeFileSync(
  path.join(outputDir, 'backend-references.json'),
  JSON.stringify({ routes: output, reachableFiles: [...files.keys()], references: refs }, null, 2),
);
const counts = Object.fromEntries(
  ['reachable-reference', 'client-import-review', 'no-static-reference'].map((s) => [
    s,
    output.filter((r) => r.status === s).length,
  ]),
);
console.log(
  JSON.stringify({
    registered: routes.length,
    reachableFiles: files.size,
    ...counts,
  }),
);
for (const route of output.filter(
  (r) => r.status !== 'reachable-reference' && r.path.startsWith('/api/v1/') && !r.path.includes('/debug/'),
))
  console.log(
    route.status +
      ' | ' +
      route.method +
      ' ' +
      route.path +
      ' | ' +
      route.refs.map((r) => r.file + ':' + r.fn).join(','),
  );

const components = [];
for (const [file, record] of files) {
  if (!file.endsWith('.vue')) continue;
  const template = parseSfc(fs.readFileSync(file, 'utf8')).descriptor.template;
  if (!template) continue;
  const component = {
    file,
    parents: [...record.parents],
    dialogs: [],
    tabs: [],
    forms: [],
  };
  function walkTemplate(node) {
    const tag = node.tag?.toLowerCase() || '';
    const item = {
      line: node.loc.start.line + template.loc.start.line - 1,
      attributes: (node.props || []).map((p) => p.loc.source),
    };
    if (/^(hw-|el)?(dialog|drawer)$/.test(tag)) component.dialogs.push(item);
    if (/^(hw-tab-panel|eltabpane)$/.test(tag)) component.tabs.push(item);
    if (/^(hw-form|elform)$/.test(tag)) component.forms.push(item);
    for (const child of node.children || []) walkTemplate(child);
  }
  walkTemplate(parseTemplate(template.content));
  components.push(component);
}
fs.writeFileSync(path.join(outputDir, 'components.json'), JSON.stringify(components, null, 2));
console.log(
  JSON.stringify({
    vueComponents: components.length,
    dialogs: components.reduce((n, c) => n + c.dialogs.length, 0),
    tabPanes: components.reduce((n, c) => n + c.tabs.length, 0),
    forms: components.reduce((n, c) => n + c.forms.length, 0),
  }),
);
