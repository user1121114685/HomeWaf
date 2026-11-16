// Conservative source reachability inventory. Reports candidates; never deletes code.
// Run from any directory: node scripts/audit-unused-frontend.mjs [--check]
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
process.chdir(root);
const require = createRequire(path.join(root, 'web/package.json'));
const { parse } = require('@babel/parser');
const { parse: parseSfc } = require('@vue/compiler-sfc');
const tracked = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], {
  encoding: 'utf8',
})
  .split('\0')
  .filter((file) => file && fs.existsSync(file));
const files = new Set(tracked);
const codeFiles = tracked.filter(
  (file) => /\.(?:vue|[cm]?[jt]sx?|css|less|html)$/.test(file) && /^(?:web\/|scripts\/|\.github\/scripts\/)/.test(file),
);
const candidates = codeFiles.filter(
  (file) => file.startsWith('web/src/') && /\.(?:vue|[cm]?[jt]sx?)$/.test(file) && !file.endsWith('.d.ts'),
);
// HTML, build/configuration scripts and tests are independent entry points.
// Type declarations are compiler entries even when they have no explicit import.
const roots = new Set(
  codeFiles.filter((file) => file.endsWith('.d.ts') || !/^web\/(?:src|public|vendor)\//.test(file)),
);
roots.add('web/src/main.js');
const records = new Map();
const namespaceTargets = new Set();
const dynamicReferences = [];
const errors = [];

function resolve(from, value) {
  if (typeof value !== 'string' || !value) return null;
  value = value.split(/[?#]/)[0];
  const bases = value.startsWith('@/')
    ? ['web/src/' + value.slice(2)]
    : value.startsWith('~/')
      ? ['web/' + value.slice(2)]
      : value.startsWith('.')
        ? [
            path.posix.normalize(path.posix.join(path.posix.dirname(from), value)),
            path.posix.normalize(value),
            path.posix.normalize('web/' + value),
          ]
        : [value, 'web/' + value];
  for (const base of bases) {
    for (const file of [
      base,
      ...['.ts', '.js', '.mjs', '.tsx', '.vue', '/index.ts', '/index.js'].map((ext) => base + ext),
    ]) {
      if (files.has(file)) return file;
    }
  }
  return null;
}

function walk(node, visit) {
  if (!node || typeof node !== 'object') return;
  visit(node);
  for (const [key, value] of Object.entries(node)) {
    if (['loc', 'comments', 'tokens', 'leadingComments', 'trailingComments', 'innerComments'].includes(key)) continue;
    if (Array.isArray(value)) value.forEach((child) => walk(child, visit));
    else if (value && typeof value === 'object') walk(value, visit);
  }
}

for (const file of codeFiles) {
  const source = fs.readFileSync(file, 'utf8');
  const references = new Set();
  const exports = [];
  const add = (value) => {
    const target = resolve(file, value);
    if (target) references.add(target);
    return target;
  };
  let code = source;
  if (file.endsWith('.vue')) {
    const { descriptor, errors: parseErrors } = parseSfc(source, { filename: file });
    errors.push(...parseErrors.map((error) => ({ file, message: String(error) })));
    code = (descriptor.script?.content || '') + '\n' + (descriptor.scriptSetup?.content || '');
    for (const block of [descriptor.script, descriptor.scriptSetup, descriptor.template, ...descriptor.styles]) {
      if (block?.src) add(block.src);
    }
  }
  if (/\.(?:vue|[cm]?[jt]sx?)$/.test(file)) {
    try {
      const ast = parse(code, {
        sourceType: 'unambiguous',
        plugins: ['typescript', ...(/\.[jt]sx$/.test(file) ? ['jsx'] : []), 'decorators-legacy'],
      });
      walk(ast, (node) => {
        // Includes static imports, import(), re-exports, require(), new URL(),
        // test fixture reads and explicit component registration modules.
        if (node.type === 'StringLiteral') add(node.value);
        if (node.type === 'TemplateLiteral' && !node.expressions.length) add(node.quasis[0]?.value.cooked);
        if (
          node.type === 'ImportDeclaration' &&
          node.specifiers.some((spec) => spec.type === 'ImportNamespaceSpecifier')
        ) {
          const target = add(node.source.value);
          if (target) namespaceTargets.add(target);
        }
        const dynamicImport = node.type === 'ImportExpression' && node.source?.type !== 'StringLiteral';
        const glob =
          node.type === 'CallExpression' &&
          node.callee?.type === 'MemberExpression' &&
          ['glob', 'globEager', 'context'].includes(node.callee.property?.name);
        if (dynamicImport || glob)
          dynamicReferences.push({ file, line: node.loc.start.line, expression: code.slice(node.start, node.end) });
      });
      if (!file.endsWith('.vue')) {
        for (const node of ast.program.body) {
          if (node.type !== 'ExportNamedDeclaration' || !node.declaration) continue;
          const declaration = node.declaration;
          const names = declaration.id
            ? [declaration.id.name]
            : (declaration.declarations || []).map((item) => item.id?.name).filter(Boolean);
          if (names.length) exports.push({ names, line: node.loc.start.line, start: node.start, end: node.end });
        }
      }
    } catch (error) {
      errors.push({ file, message: error.message });
    }
  }
  // SFC styles/templates and CSS imports may not appear in the script AST.
  for (const match of source.matchAll(/(?:from\s*|@import\s*|url\(\s*|(?:src|href)\s*=\s*)['"]([^'"]+)['"]/g))
    add(match[1]);
  records.set(file, { source, references: [...references], exports });
}

const reachable = new Set();
function visit(file) {
  if (reachable.has(file)) return;
  reachable.add(file);
  for (const reference of records.get(file)?.references || []) visit(reference);
}
for (const file of roots) visit(file);

// Unresolved production dynamic imports/globs must be reviewed before any file
// is called unused. Never silently classify their targets as dead files.
const unresolvedDynamic = dynamicReferences.filter(({ file }) => file.startsWith('web/src/') && reachable.has(file));
const reviewRequired = errors.length > 0 || unresolvedDynamic.length > 0;
const unreachableFiles = candidates.filter((file) => !reachable.has(file));
const unusedExportCandidates = [];
if (!reviewRequired) {
  for (const file of candidates.filter((candidate) => reachable.has(candidate) && !namespaceTargets.has(candidate))) {
    const record = records.get(file);
    for (const item of record?.exports || []) {
      const outside = record.source.slice(0, item.start) + record.source.slice(item.end);
      const used = item.names.some((name) => {
        const pattern = new RegExp('\\b' + name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\b');
        return (
          pattern.test(outside) ||
          [...records].some(([other, value]) => other !== file && reachable.has(other) && pattern.test(value.source))
        );
      });
      if (!used) unusedExportCandidates.push({ file, names: item.names, line: item.line });
    }
  }
}
const report = {
  sourceFiles: candidates.length,
  entryPoints: [...roots].sort(),
  reachableFiles: [...reachable].sort(),
  reviewRequired,
  unreachableFileCandidates: reviewRequired ? [] : unreachableFiles,
  unresolvedReachability: reviewRequired ? unreachableFiles : [],
  unusedExportCandidates,
  dynamicReferences,
  errors,
};
fs.mkdirSync('artifacts/unused-code', { recursive: true });
fs.writeFileSync('artifacts/unused-code/frontend.json', JSON.stringify(report, null, 2) + '\n');
console.log(JSON.stringify({ ...report, entryPoints: roots.size, reachableFiles: reachable.size }, null, 2));
if (process.argv.includes('--check') && (reviewRequired || unreachableFiles.length || unusedExportCandidates.length))
  process.exitCode = 1;
