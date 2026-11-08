"""Generate the distributable dependency inventory from the resolved graphs.
Run after npm ci and go mod download. Requires go-licenses report in .cache.
"""
from pathlib import Path
import csv
import hashlib
import json
import re
import shutil
import subprocess

ROOT = Path(__file__).resolve().parent.parent

def read(path):
    data = Path(path).read_bytes()
    return data.decode('utf-16' if data[:2] == b'\xff\xfe' else 'utf-8-sig')

def stream(text):
    decoder = json.JSONDecoder()
    while text.strip():
        value, end = decoder.raw_decode(text.lstrip())
        yield value
        text = text.lstrip()[end:]

def licenses(directory, destination):
    copied = []
    for p in directory.iterdir():
        if p.is_file() and re.match(r'^(licen[sc]e|copying|copyright|notice|authors)([.\-_]|$)', p.name, re.I):
            destination.mkdir(parents=True, exist_ok=True)
            target = destination / p.name
            target.write_bytes(p.read_bytes())
            copied.append(target.relative_to(ROOT).as_posix())
    return copied

def main():
    import os
    os.chdir(ROOT)
    modules = {m['Path']: m for m in stream(subprocess.check_output(['go', 'list', '-m', '-json', 'all'], text=True, encoding='utf-8'))}
    used = set(subprocess.check_output(['go', 'list', '-deps', '-f', '{{if .Module}}{{.Module.Path}}{{end}}', './cmd/homewaf'], text=True, encoding='utf-8').split())
    platforms = ROOT/'.cache/platform-modules.json'
    if platforms.exists():
        for names in json.loads(read(platforms)).values(): used.update(names)
    report = list(csv.reader(read(ROOT / '.cache/go-license-report.csv').splitlines()))
    components = []
    lines = ['# HomeWaf third-party notices', '',
             'Generated from the resolved backend runtime graph and frontend lockfile. Third-party rights are not restricted by the HomeWaf license.', '',
             'See NOTICE for upstream project attribution and LICENSES for license texts. Development and optional platform packages are identified separately.', '',
             '## Backend modules', '', '| Module | Version | License |', '|---|---|---|']
    for name in sorted(used - {'HomeWaf'}):
        module = modules[name]
        actual = module.get('Replace', module)
        detected = sorted({r[-1] for r in report if r and any(r[0] == n or r[0].startswith(n + '/') for n in [name, actual['Path']])})
        identifier = ' AND '.join(detected) or 'NOASSERTION'
        version = actual.get('Version', module.get('Version', ''))
        lines.append(f'| {name} | {version} | {identifier} |')
        components.append({'type':'library', 'bom-ref':'go:'+name, 'name':actual['Path'], 'version':version,
                           'purl':f'pkg:golang/{actual["Path"]}@{version}', 'licenses':[{'license':{'name':identifier}}],
                           'properties':[{'name':'homewaf:requested-module','value':name}]})
    lines += ['', '## Frontend and build packages', '', '| Package | Version | Scope | License |', '|---|---|---|---|']
    lock = json.loads(read(ROOT/'web/package-lock.json'))
    missing = []
    for location, package in sorted(lock['packages'].items()):
        if not location or package.get('link'): continue
        name = package.get('name') or location.split('node_modules/')[-1]
        directory = ROOT/'web'/location
        scope = 'development' if package.get('dev') or location.startswith('vendor/') else 'runtime'
        if package.get('optional'): scope += ', optional platform'
        identifier = package.get('license', 'NOASSERTION')
        copied = licenses(directory, ROOT/'LICENSES/npm'/name) if directory.exists() else []
        if directory.exists() and not copied and not (ROOT/'LICENSES/npm'/name).exists(): missing.append(name)
        lines.append(f'| {name} | {package["version"]} | {scope} | {identifier} |')
        components.append({'type':'library','bom-ref':'npm:'+location,'name':name,'version':package['version'],
                           'purl':f'pkg:npm/{name}@{package["version"]}','licenses':[{'license':{'name':identifier}}],
                           'properties':[{'name':'homewaf:scope','value':scope},{'name':'homewaf:integrity','value':package.get('integrity','')}]})
    copied = [
        ('SQLite3MultipleCiphers', '2.6.1', 'MIT AND BSD-2-Clause AND Apache-2.0 AND CC0-1.0', 'internal/sqlite3/sqlite3-binding.c'),
        ('sqlite', '3.54.0', 'public-domain', None),
        ('mattn/go-sqlite3 local adapter', '1.14.53', 'MIT', None),
        ('gorm-sqlite local adapter', '1.6.0', 'MIT', None),
        ('gorm-mysql local adapter', '1.6.0', 'MIT', None),
        ('go-mysql local driver adapter', '1.16.0', 'MIT', None),
        ('GoCaptcha local adapter', '2.0.5', 'Apache-2.0', None),
        ('Grule local adapter', '1.20.4', 'Apache-2.0', None),
        ('GoDLP local adapter', '1.2.15', 'MIT', None),
        ('CAP Go server local copy', '1.0.1', 'Apache-2.0', None),
        ('LXGW WenKai', '1.530', 'OFL-1.1', 'internal/captchaassets/LXGWWenKai-Regular.ttf'),
        ('CAP widget', '0.1.61', 'Apache-2.0', 'cmd/homewaf/exedata/capjs/widget.js'),
        ('CAP wasm', '0.0.8', 'Apache-2.0', 'cmd/homewaf/exedata/capjs/cap_wasm_bg.wasm'),
        ('OWASP CRS', '4.30.0', 'Apache-2.0', None),
        ('ip2region IPv4 XDB', '6a276cc7c4f9', 'Apache-2.0 OR MIT', 'cmd/homewaf/exedata/ip2region.xdb'),
    ]
    for name, version, identifier, file in copied:
        component = {'type':'library','bom-ref':'embedded:'+name,'name':name,'version':version,'licenses':[{'license':{'name':identifier}}]}
        if file: component['hashes']=[{'alg':'SHA-256','content':hashlib.sha256((ROOT/file).read_bytes()).hexdigest()}]
        components.append(component)
    for item in json.loads(read(ROOT/'docs/modernization/cap-wasm-build-dependencies.json')):
        components.append({'type':'library','bom-ref':'wasm-build:'+item['name'],'name':item['name'],'version':item['version'],
                           'purl':f'pkg:cargo/{item["name"]}@{item["version"]}', 'licenses':[{'license':{'name':item['license']}}],
                           'hashes':[{'alg':'SHA-256','content':item['sha256']}],
                           'properties':[{'name':'homewaf:scope','value':'upstream CAP WASM build dependency; not a separately installed application package'}]})
    lines += ['', '## Copied code and embedded resources', '',
              '| Component | License and provenance |', '|---|---|',
              '| SamWaf backend and SamWafWeb UI | Apache-2.0; LICENSES/SamWaf*.txt; docs/modernization/upstream.json |',
              '| Historical TDesign Vue Starter template portions | MIT; LICENSES/TDesign-starter-MIT.txt; runtime framework removed |',
              '| SQLite3 Multiple Ciphers 2.6.1 / SQLite 3.54.0 / mattn go-sqlite3 1.14.53 | MIT, public-domain/BSD/CC0 component notices; internal/sqlite3 and LICENSES/native |',
              '| GORM SQLite 1.6.0 and MySQL 1.6.0 adapters; go-mysql 1.16.0 adapter | MIT; LICENSES/native; preserved upstream notices |',
              '| GoCaptcha 2.0.5 and character data | Apache-2.0; internal/thirdparty/gocaptcha/LICENSE and internal/captchaassets/LICENSE-CHARS |',
              '| Grule 1.20.4 local adapter, Git loader omitted | Apache-2.0; internal/thirdparty/grule/LICENSE*; docs/modernization/grule-provenance.json |',
              '| GoDLP 1.2.15 local adapter, maintained YAML v4 | MIT; LICENSES/copied/godlp; docs/modernization/godlp-provenance.json |',
              '| CAP Go server 1.0.1 local copy | Apache-2.0; LICENSES/copied/capserver; docs/modernization/cap-server-provenance.json |',
              '| LXGW WenKai 1.530, original font | SIL OFL-1.1; LICENSES/fonts; no font is sold by itself |',
              '| CAP widget 0.1.61 / wasm 0.0.8 | Apache-2.0; LICENSES/cap-widget.txt; docs/modernization/embedded-web.json |',
              '| OWASP CRS 4.30.0 | Apache-2.0; LICENSES/crs; docs/modernization/crs-provenance.json |',
              '| ip2region IPv4 XDB | Apache-2.0 OR MIT; LICENSES/ip2region.txt; docs/modernization/ip2region-provenance.json |',
              '| HomeWaf logos, CSS backgrounds and click challenge UI | Original HomeWaf additions; root LICENSE |', '',
              'DOMPurify is used under its Apache-2.0 alternative. Browser/system fonts are referenced by family name and are not redistributed.',
              'The optional user-supplied MaxMind databases and ML models are not included in releases; their operators must obtain the corresponding rights.',
              'Unused upstream crawler IP archives, map data, login imagery, frontend binary bundles and UPX executables are not distributed.', '']
    lines += ['CAP WASM upstream Rust build dependencies are listed in docs/modernization/cap-wasm-build-dependencies.json with crate checksums and LICENSES/wasm notices. The official released WASM is used without claiming its compiler-internal dependencies were independently upgraded.', '']
    (ROOT/'THIRD_PARTY_NOTICES.md').write_text('\n'.join(lines), encoding='utf-8')
    (ROOT/'docs/modernization/sbom.cdx.json').write_text(json.dumps({'bomFormat':'CycloneDX','specVersion':'1.6','version':1,
       'metadata':{'component':{'type':'application','name':'HomeWaf','version':'2.0.0-dev'}},'components':components}, indent=2), encoding='utf-8')
    print(json.dumps({'components':len(components),'installed_npm_missing_license_file':missing}, ensure_ascii=False))

if __name__ == '__main__': main()
