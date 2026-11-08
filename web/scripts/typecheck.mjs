// TypeScript 7's native compiler does not expose the legacy compiler API used by
// current vue-tsc/Volar. Compile Vue SFC scripts with the official Vue compiler,
// then run the native TypeScript compiler over the mirrored source tree.
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc';
const root = path.resolve(fileURLToPath(new URL('..', import.meta.url)));
const target = path.join(root, '.cache', 'typecheck');
if (!target.startsWith(root + path.sep + '.cache' + path.sep)) throw new Error('Unsafe typecheck cache path');
fs.rmSync(target, { recursive: true, force: true });
fs.mkdirSync(path.join(target, 'src'), { recursive: true });
let errors = 0;
function visit(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const source = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      visit(source);
      continue;
    }
    const relative = path.relative(path.join(root, 'src'), source);
    const out = path.join(target, 'src', relative);
    fs.mkdirSync(path.dirname(out), { recursive: true });
    if (source.endsWith('.vue')) {
      const text = fs.readFileSync(source, 'utf8');
      const { descriptor, errors: parseErrors } = parse(text, { filename: source });
      const issues = [...parseErrors];
      let code = 'export default {};';
      try {
        if (descriptor.script || descriptor.scriptSetup) code = compileScript(descriptor, { id: relative }).content;
        if (descriptor.template)
          issues.push(
            ...compileTemplate({
              source: descriptor.template.content,
              filename: source,
              id: relative,
              compilerOptions: { expressionPlugins: ['typescript'] },
            }).errors,
          );
      } catch (error) {
        issues.push(error);
      }
      for (const issue of issues) {
        console.error(`${relative}: ${issue.message || issue}`);
        errors++;
      }
      fs.writeFileSync(
        out + ((descriptor.scriptSetup?.lang || descriptor.script?.lang) === 'ts' ? '.ts' : '.js'),
        code,
      );
    } else if (/\.(?:[cm]?[jt]sx?|json)$/.test(source)) {
      fs.copyFileSync(source, out);
    }
  }
}
visit(path.join(root, 'src'));
const config = JSON.parse(fs.readFileSync(path.join(root, 'tsconfig.json'), 'utf8'));
config.include = ['src/**/*.ts', 'src/**/*.tsx', 'src/**/*.d.ts'];
fs.copyFileSync(path.join(root, 'package.json'), path.join(target, 'package.json'));
fs.writeFileSync(path.join(target, 'tsconfig.json'), JSON.stringify(config, null, 2));
if (errors) process.exit(1);
const compiler = path.join(root, 'node_modules', 'typescript', 'bin', 'tsc');
const result = spawnSync(process.execPath, [compiler, '--project', path.join(target, 'tsconfig.json'), '--noEmit'], {
  cwd: root,
  stdio: 'inherit',
});
if (result.error) throw result.error;
process.exit(result.status ?? 1);
