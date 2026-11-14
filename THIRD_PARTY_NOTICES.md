# HomeWaf third-party notices

Component inventory for the backend runtime graph, frontend lockfile and embedded resources. Third-party rights are not restricted by the HomeWaf license.

See [NOTICE](NOTICE) for upstream attribution and [LICENSES/THIRD_PARTY.txt](LICENSES/THIRD_PARTY.txt) for complete license texts. Development and optional platform packages are identified separately.

The license bundle groups only byte-identical texts and retains every original source label and copyright notice. The labels inside the bundle are an index, not separate repository files. Licenses beside copied source files remain in place.

When dependencies or embedded assets change, update this inventory, the corresponding texts in the license bundle and [provenance.json](docs/modernization/provenance.json). Keep package versions in `go.mod` / `go.sum` and `web/package-lock.json`; store temporary audit reports and SBOM exports under ignored `artifacts/` or `.cache/` directories.

## Backend modules

| Module | Version | License |
|---|---|---|
| filippo.io/edwards25519 | v1.2.0 | BSD-3-Clause |
| github.com/alibabacloud-go/alibabacloud-gateway-spi | v0.0.5 | Apache-2.0 |
| github.com/alibabacloud-go/darabonba-openapi/v2 | v2.2.5 | Apache-2.0 |
| github.com/alibabacloud-go/debug | v1.0.1 | Apache-2.0 |
| github.com/alibabacloud-go/tea | v1.5.4 | Apache-2.0 |
| github.com/alibabacloud-go/tea-utils/v2 | v2.0.9 | Apache-2.0 |
| github.com/aliyun/credentials-go | v1.4.14 | Apache-2.0 |
| github.com/andybalholm/brotli | v1.2.8 | MIT |
| github.com/antlr4-go/antlr/v4 | v4.13.1 | BSD-3-Clause |
| github.com/baidubce/bce-sdk-go | v0.9.277 | Apache-2.0 |
| github.com/bmatcuk/doublestar/v4 | v4.10.4 | MIT |
| github.com/boombuler/barcode | v1.1.0 | MIT |
| github.com/cenkalti/backoff/v5 | v5.0.3 | MIT |
| github.com/cespare/xxhash/v2 | v2.3.0 | MIT |
| github.com/clbanning/mxj/v2 | v2.7.0 | MIT |
| github.com/cloudflare/ahocorasick | v0.0.0-20240916140611-054963ec9396 | BSD-3-Clause |
| github.com/coder/websocket | v1.8.15 | ISC |
| github.com/corazawaf/coraza/v3 | v3.8.1 | Apache-2.0 |
| github.com/corazawaf/libinjection-go | v0.3.5 | BSD-3-Clause |
| github.com/coreos/go-semver | v0.3.1 | Apache-2.0 |
| github.com/denisbrodbeck/machineid | v1.0.1 | MIT |
| github.com/dmitryikh/leaves | v0.0.0-20230708180554-25d19a787328 | MIT |
| github.com/dsnet/compress | v0.0.1 | BSD-3-Clause |
| github.com/dustin/go-humanize | v1.1.0 | MIT |
| github.com/ebitengine/purego | v0.11.3 | Apache-2.0 |
| github.com/fsnotify/fsnotify | v1.10.1 | BSD-3-Clause |
| github.com/gabriel-vasile/mimetype | v1.4.16 | MIT |
| github.com/gin-contrib/gzip | v1.2.10 | MIT |
| github.com/gin-contrib/pprof | v1.5.8 | MIT |
| github.com/gin-contrib/sse | v1.2.1 | MIT |
| github.com/gin-gonic/gin | v1.12.0 | MIT |
| github.com/go-acme/alidns-20150109/v4 | v4.7.0 | Apache-2.0 |
| github.com/go-acme/lego/v4 | v4.35.2 | MIT |
| github.com/go-acme/tencentclouddnspod | v1.3.131 | Apache-2.0 |
| github.com/go-co-op/gocron/v2 | v2.23.0 | MIT |
| github.com/go-jose/go-jose/v4 | v4.1.9 | Apache-2.0 AND BSD-3-Clause |
| github.com/go-mysql-org/go-mysql | v1.16.0 | MIT |
| github.com/go-ole/go-ole | v1.3.0 | MIT |
| github.com/go-playground/locales | v0.14.2 | MIT |
| github.com/go-playground/universal-translator | v0.18.2 | MIT |
| github.com/go-playground/validator/v10 | v10.30.6 | MIT |
| github.com/go-viper/mapstructure/v2 | v2.5.0 | MIT |
| github.com/goccy/go-yaml | v1.19.2 | MIT |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause |
| github.com/gorilla/websocket | v1.5.3 | BSD-2-Clause |
| github.com/huaweicloud/huaweicloud-sdk-go-v3 | v0.1.222 | BSD-2-Clause AND BSD-3-Clause |
| github.com/inconshreveable/mousetrap | v1.1.0 | Apache-2.0 |
| github.com/ipipdotnet/ipdb-go | v1.3.3 | Apache-2.0 |
| github.com/jackc/pgpassfile | v1.0.0 | MIT |
| github.com/jackc/pgservicefile | v0.0.0-20240606120523-5a60cdf6a761 | MIT |
| github.com/jackc/pgx/v5 | v5.12.0 | MIT |
| github.com/jackc/puddle/v2 | v2.2.3 | MIT |
| github.com/jinzhu/inflection | v1.0.0 | MIT |
| github.com/jinzhu/now | v1.1.5 | MIT |
| github.com/jonboulle/clockwork | v0.5.0 | Apache-2.0 |
| github.com/json-iterator/go | v1.1.13-0.20220915233716-71ac16282d12 | MIT |
| github.com/kaptinlin/jsonpointer | v0.4.28 | MIT |
| github.com/kaptinlin/jsonschema | v0.9.10 | MIT |
| github.com/kardianos/service | v1.3.0 | Zlib |
| github.com/klauspost/compress | v1.20.3 | Apache-2.0 AND BSD-3-Clause AND MIT |
| github.com/leodido/go-urn | v1.5.0 | MIT |
| github.com/lionsoul2014/ip2region/binding/golang | v0.0.0-20261028042204-6a276cc7c4f9 | Apache-2.0 AND MIT |
| github.com/mattn/go-colorable | v0.1.16 | MIT |
| github.com/mattn/go-isatty | v0.0.24 | MIT |
| github.com/miekg/dns | v1.1.73 | BSD-3-Clause |
| github.com/modern-go/concurrent | v0.0.0-20180306012644-bacd9c7ef1dd | Apache-2.0 |
| github.com/modern-go/reflect2 | v1.0.3-0.20250322232337-35a7c28c31ee | Apache-2.0 |
| github.com/ncruces/go-strftime | v1.1.0 | MIT |
| github.com/oschwald/geoip2-golang/v2 | v2.5.0 | ISC |
| github.com/oschwald/maxminddb-golang/v2 | v2.9.0 | ISC |
| github.com/pelletier/go-toml/v2 | v2.4.3 | MIT |
| github.com/petar-dambovaliev/aho-corasick | v0.0.0-20250424160509-463d218d4745 | MIT |
| github.com/pierrec/lz4/v4 | v4.1.33 | BSD-3-Clause |
| github.com/pingcap/errors | v0.11.5-0.20260310054046-9c8b3586e4b2 | BSD-2-Clause |
| github.com/pingcap/log | v1.1.1-0.20260227082333-572e590d08f1 | Apache-2.0 |
| github.com/pingcap/tidb/pkg/parser | v0.0.0-20261107225221-b98174386ec5 | Apache-2.0 |
| github.com/pires/go-proxyproto | v0.16.0 | Apache-2.0 |
| github.com/pquerna/otp | v1.5.0 | Apache-2.0 |
| github.com/quic-go/qpack | v0.6.0 | MIT |
| github.com/quic-go/quic-go | v0.0.0-20261108163454-291d013e1594 | MIT |
| github.com/redis/go-redis/v9 | v9.23.0 | BSD-2-Clause |
| github.com/remyoudompheng/bigfft | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause |
| github.com/richardlehane/mscfb | v1.0.9 | Apache-2.0 |
| github.com/richardlehane/msoleps | v1.0.6 | Apache-2.0 |
| github.com/robfig/cron/v3 | v3.0.1 | MIT |
| github.com/rs/zerolog | v1.35.1 | MIT |
| github.com/sagikazarmark/locafero | v0.12.0 | MIT |
| github.com/shirou/gopsutil/v4 | v4.26.10 | BSD-3-Clause |
| github.com/sirupsen/logrus | v1.10.2 | MIT |
| github.com/spf13/afero | v1.15.0 | Apache-2.0 |
| github.com/spf13/cast | v1.10.0 | MIT |
| github.com/spf13/cobra | v1.10.2 | Apache-2.0 |
| github.com/spf13/pflag | v1.1.0 | BSD-3-Clause |
| github.com/spf13/viper | v1.21.0 | MIT |
| github.com/subosito/gotenv | v1.6.0 | MIT |
| github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common | v1.3.210 | Apache-2.0 |
| github.com/tidwall/gjson | v1.20.2 | MIT |
| github.com/tidwall/match | v1.2.0 | MIT |
| github.com/tidwall/pretty | v1.3.0 | MIT |
| github.com/tiendc/go-deepcopy | v1.7.2 | MIT |
| github.com/tjfoc/gmsm | v1.4.1 | Apache-2.0 |
| github.com/tklauser/go-sysconf | v0.4.0 | BSD-3-Clause |
| github.com/tklauser/numcpus | v0.12.0 | Apache-2.0 |
| github.com/twmb/franz-go | v1.22.2 | BSD-3-Clause |
| github.com/twmb/franz-go/pkg/kmsg | v1.15.0 | BSD-3-Clause |
| github.com/ugorji/go/codec | v1.3.2 | MIT |
| github.com/valllabh/ocsf-schema-golang | v1.0.3 | Apache-2.0 |
| github.com/xuri/efp | v0.0.3-0.20260924042702-217b81952dec | BSD-3-Clause |
| github.com/xuri/excelize/v2 | v2.11.1 | BSD-3-Clause |
| github.com/xuri/nfp | v0.0.2-0.20250530014748-2ddeb826f9a9 | BSD-3-Clause |
| github.com/yosida95/uritemplate/v3 | v3.0.2 | BSD-3-Clause |
| github.com/yusufpapurcu/wmi | v1.2.4 | MIT |
| go.mongodb.org/mongo-driver | v1.17.10 | Apache-2.0 |
| go.mongodb.org/mongo-driver/v2 | v2.10.0 | Apache-2.0 |
| go.uber.org/atomic | v1.12.0 | MIT |
| go.uber.org/multierr | v1.11.0 | MIT |
| go.uber.org/zap | v1.28.0 | MIT |
| go.yaml.in/yaml/v3 | v3.0.6 | MIT |
| go.yaml.in/yaml/v4 | v4.0.0 | Apache-2.0 |
| golang.org/x/crypto | v0.58.1-0.20261009181718-2f1f834330b2 | BSD-3-Clause |
| golang.org/x/exp | v0.0.0-20261106192419-47ce03d25ef0 | BSD-3-Clause |
| golang.org/x/image | v0.47.0 | BSD-3-Clause |
| golang.org/x/mod | v0.42.0 | BSD-3-Clause |
| golang.org/x/net | v0.62.0 | BSD-3-Clause |
| golang.org/x/sync | v0.24.0 | BSD-3-Clause |
| golang.org/x/sys | v0.49.0 | BSD-3-Clause |
| golang.org/x/text | v0.43.0 | BSD-3-Clause |
| golang.org/x/time | v0.16.0 | BSD-3-Clause |
| google.golang.org/genproto/googleapis/rpc | v0.0.0-20261104224025-d1e00e9f33ab | Apache-2.0 |
| google.golang.org/grpc | v1.85.0 | Apache-2.0 |
| google.golang.org/protobuf | v1.36.12 | BSD-3-Clause |
| gopkg.in/ini.v1 | v1.67.3 | Apache-2.0 |
| gopkg.in/natefinch/lumberjack.v2 | v2.2.1 | MIT |
| gorm.io/driver/postgres | v1.6.3 | MIT |
| gorm.io/gorm | v1.31.2 | MIT |
| modernc.org/libc | v1.80.1 | BSD-3-Clause; bundled third-party notices |
| modernc.org/mathutil | v1.7.1 | BSD-3-Clause |
| modernc.org/memory | v1.12.1 | BSD-3-Clause; bundled third-party notices |
| modernc.org/sqlite | v1.62.0 | BSD-3-Clause; SQLite public domain; bundled third-party notices |
| rsc.io/binaryregexp | v0.2.0 | BSD-3-Clause |

## Frontend and build packages

| Package | Version | Scope | License |
|---|---|---|---|
| @babel/helper-string-parser | 8.0.8 | runtime | MIT |
| @babel/helper-validator-identifier | 8.0.6 | runtime | MIT |
| @babel/parser | 8.1.0 | runtime | MIT |
| @babel/types | 8.1.0 | runtime | MIT |
| @codemirror/autocomplete | 6.20.4 | runtime | MIT |
| @codemirror/commands | 6.11.1 | runtime | MIT |
| @codemirror/lang-javascript | 6.2.5 | runtime | MIT |
| @codemirror/lang-json | 6.0.2 | runtime | MIT |
| @codemirror/lang-sql | 6.10.0 | runtime | MIT |
| @codemirror/language | 6.13.1 | runtime | MIT |
| @codemirror/lint | 6.9.7 | runtime | MIT |
| @codemirror/search | 6.7.2 | runtime | MIT |
| @codemirror/state | 6.7.6 | runtime | MIT |
| @codemirror/streamparser | 6.0.0 | runtime | MIT |
| @codemirror/view | 6.43.15 | runtime | MIT |
| @ctrl/tinycolor | 4.3.0 | runtime | MIT |
| @element-plus/icons-vue | 2.3.2 | runtime | MIT |
| @emnapi/core | 2.0.0-beta.2 | runtime, optional platform | MIT |
| @emnapi/runtime | 2.0.0-beta.2 | runtime, optional platform | MIT |
| @emnapi/wasi-threads | 2.2.0 | runtime, optional platform | MIT |
| @floating-ui/core | 1.8.0 | runtime | MIT |
| @floating-ui/dom | 1.8.0 | runtime | MIT |
| @floating-ui/utils | 0.2.12 | runtime | MIT |
| @intlify/core-base | 11.4.18 | runtime | MIT |
| @intlify/devtools-types | 11.4.18 | runtime | MIT |
| @intlify/message-compiler | 11.4.18 | runtime | MIT |
| @intlify/shared | 11.4.18 | runtime | MIT |
| @jridgewell/gen-mapping | 0.3.13 | runtime | MIT |
| @jridgewell/remapping | 2.3.5 | runtime | MIT |
| @jridgewell/resolve-uri | 3.1.2 | runtime | MIT |
| @jridgewell/sourcemap-codec | 1.6.0 | runtime | MIT |
| @jridgewell/trace-mapping | 0.3.31 | runtime | MIT |
| @lezer/common | 1.5.5 | runtime | MIT |
| @lezer/highlight | 1.2.5 | runtime | MIT |
| @lezer/javascript | 1.5.6 | runtime | MIT |
| @lezer/json | 1.0.3 | runtime | MIT |
| @lezer/lr | 1.4.12 | runtime | MIT |
| @marijn/find-cluster-break | 1.0.4 | runtime | MIT |
| @napi-rs/wasm-runtime | 1.2.7 | runtime, optional platform | MIT |
| @noble/ciphers | 2.4.0 | runtime | MIT |
| @noble/curves | 2.4.0 | runtime | MIT |
| @noble/hashes | 2.4.0 | runtime | MIT |
| @oxc-project/types | 0.157.0 | runtime | MIT |
| @playwright/test | 1.65.0 | development | Apache-2.0 |
| @sxzz/popperjs-es | 2.11.8 | runtime | MIT |
| @rolldown/binding-android-arm-eabi | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-android-arm64 | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-darwin-arm64 | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-darwin-x64 | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-freebsd-x64 | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-arm-gnueabihf | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-arm64-gnu | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-arm64-musl | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-ppc64-gnu | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-s390x-gnu | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-x64-gnu | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-linux-x64-musl | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-openharmony-arm64 | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-wasm32-wasi | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-win32-arm64-msvc | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/binding-win32-x64-msvc | 1.3.0 | runtime, optional platform | MIT |
| @rolldown/pluginutils | 1.0.1 | runtime | MIT |
| @tybys/wasm-util | 0.10.4 | runtime, optional platform | MIT |
| @types/estree | 1.0.10 | runtime | MIT |
| @types/lodash | 4.17.27 | runtime | MIT |
| @types/lodash-es | 4.17.12 | runtime | MIT |
| @types/trusted-types | 2.0.7 | runtime, optional platform | MIT |
| @types/web-bluetooth | 0.0.22 | runtime | MIT |
| @typescript/typescript-aix-ppc64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-darwin-arm64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-darwin-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-freebsd-arm64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-freebsd-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-arm | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-arm64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-loong64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-mips64el | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-ppc64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-riscv64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-s390x | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-linux-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-netbsd-arm64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-netbsd-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-openbsd-arm64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-openbsd-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-sunos-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-win32-arm64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @typescript/typescript-win32-x64 | 7.0.2 | runtime, optional platform | Apache-2.0 |
| @unhead/bundler | 3.4.3 | runtime | MIT |
| @unhead/vue | 3.4.3 | runtime | MIT |
| @vitejs/plugin-vue | 6.0.10 | development | MIT |
| @vue-macros/common | 3.1.4 | runtime | MIT |
| @vue/compiler-core | 3.5.45 | runtime | MIT |
| @vue/compiler-dom | 3.5.45 | runtime | MIT |
| @vue/compiler-sfc | 3.5.45 | runtime | MIT |
| @vue/compiler-ssr | 3.5.45 | runtime | MIT |
| @vue/devtools-api | 8.2.1 | runtime | MIT |
| @vue/devtools-kit | 8.2.1 | runtime | MIT |
| @vue/devtools-shared | 8.2.1 | runtime | MIT |
| @vue/reactivity | 3.5.45 | runtime | MIT |
| @vue/runtime-core | 3.5.45 | runtime | MIT |
| @vue/runtime-dom | 3.5.45 | runtime | MIT |
| @vue/server-renderer | 3.5.45 | runtime | MIT |
| @vue/shared | 3.5.45 | runtime | MIT |
| @vueuse/core | 15.0.0 | runtime | MIT |
| @vueuse/metadata | 15.0.0 | runtime | MIT |
| @vueuse/shared | 15.0.0 | runtime | MIT |
| acorn | 8.19.0 | runtime | MIT |
| agent-base | 9.0.0 | runtime | MIT |
| ast-kit | 3.0.1 | runtime | MIT |
| ast-walker-scope | 0.10.0 | runtime | MIT |
| async-validator | 4.2.5 | runtime | MIT |
| asynckit | 0.5.0 | runtime | MIT |
| axios | 1.21.0 | runtime | MIT |
| birpc | 4.2.0 | runtime | MIT |
| boolbase | 2.0.0 | development | ISC |
| call-bind-apply-helpers | 1.0.2 | runtime | MIT |
| chokidar | 6.0.0 | runtime | MIT |
| codemirror | 6.0.2 | runtime | MIT |
| combined-stream | 1.0.8 | runtime | MIT |
| commander | 15.1.0 | development | MIT |
| confbox | 0.3.2 | runtime | MIT |
| copy-anything | 4.1.5 | runtime | MIT |
| crelt | 1.0.7 | runtime | MIT |
| css-select | 7.0.0 | development | BSD-2-Clause |
| css-tree | 3.2.1 | development | MIT |
| css-what | 8.0.0 | development | BSD-2-Clause |
| csso | 5.0.5 | development | MIT |
| csstype | 3.2.3 | runtime | MIT |
| dayjs | 1.11.23 | runtime | MIT |
| debug | 4.4.3 | runtime | MIT |
| delayed-stream | 1.0.0 | runtime | MIT |
| detect-libc | 2.1.2 | runtime | Apache-2.0 |
| dom-serializer | 3.1.1 | development | MIT |
| domelementtype | 3.0.0 | development | BSD-2-Clause |
| domhandler | 6.0.1 | development | BSD-2-Clause |
| dompurify | 3.4.17 | runtime | (MPL-2.0 OR Apache-2.0) |
| domutils | 4.0.2 | development | BSD-2-Clause |
| dunder-proto | 1.0.1 | runtime | MIT |
| echarts | 6.1.0 | runtime | Apache-2.0 |
| element-plus | 2.14.8 | runtime | MIT |
| lodash-es | 4.18.1 | runtime | MIT |
| entities | 9.0.0 | runtime | BSD-2-Clause |
| errno | 1.0.0 | runtime, optional platform | MIT |
| es-define-property | 1.0.1 | runtime | MIT |
| es-errors | 1.3.0 | runtime | MIT |
| es-object-atoms | 1.1.2 | runtime | MIT |
| es-set-tostringtag | 2.1.0 | runtime | MIT |
| estree-walker | 3.0.3 | runtime | MIT |
| exsolve | 1.2.0 | runtime | MIT |
| fdir | 6.5.0 | runtime | MIT |
| follow-redirects | 1.16.3 | runtime | MIT |
| form-data | 4.0.6 | runtime | MIT |
| fsevents | 2.3.3 | runtime, optional platform | MIT |
| function-bind | 1.1.2 | runtime | MIT |
| get-intrinsic | 1.3.0 | runtime | MIT |
| get-proto | 1.0.1 | runtime | MIT |
| gopd | 1.2.0 | runtime | MIT |
| graceful-fs | 4.2.11 | runtime, optional platform | ISC |
| has-symbols | 1.1.0 | runtime | MIT |
| has-tostringtag | 1.0.2 | runtime | MIT |
| hasown | 2.0.4 | runtime | MIT |
| hookable | 6.1.2 | runtime | MIT |
| https-proxy-agent | 9.1.0 | runtime | MIT |
| iconv-lite | 0.7.3 | runtime, optional platform | MIT |
| less | 4.9.1 | runtime | Apache-2.0 |
| lightningcss | 1.33.0 | runtime | MPL-2.0 |
| lightningcss-android-arm64 | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-darwin-arm64 | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-darwin-x64 | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-freebsd-x64 | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-linux-arm-gnueabihf | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-linux-arm64-gnu | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-linux-arm64-musl | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-linux-x64-gnu | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-linux-x64-musl | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-win32-arm64-msvc | 1.33.0 | runtime, optional platform | MPL-2.0 |
| lightningcss-win32-x64-msvc | 1.33.0 | runtime, optional platform | MPL-2.0 |
| local-pkg | 2.0.0 | runtime | MIT |
| lodash | 4.18.1 | runtime | MIT |
| lodash-es | 4.18.1 | runtime | MIT |
| lodash.merge | 4.6.2 | runtime, optional platform | MIT |
| magic-string | 1.4.5 | runtime | MIT |
| magic-string-ast | 2.1.1 | runtime | MIT |
| make-dir | 5.1.0 | runtime, optional platform | MIT |
| marked | 18.1.3 | runtime | MIT |
| math-intrinsics | 1.1.0 | runtime | MIT |
| mdn-data | 2.37.2 | development | CC0-1.0 |
| memoize-one | 6.0.0 | runtime | MIT |
| mime | 4.1.2 | runtime, optional platform | MIT |
| mime-db | 1.54.0 | runtime | MIT |
| mime-types | 3.0.2 | runtime | MIT |
| mlly | 1.8.2 | runtime | MIT |
| ms | 2.1.3 | runtime | MIT |
| muggle-string | 1.0.4 | runtime | MIT |
| nanoid | 6.0.2 | runtime | MIT |
| needle | 3.5.0 | runtime, optional platform | MIT |
| normalize-wheel-es | 1.2.0 | runtime | BSD-3-Clause |
| nostics | 1.3.0 | runtime | MIT |
| nth-check | 3.0.1 | development | BSD-2-Clause |
| oxc-walker | 1.1.1 | runtime | MIT |
| parse-node-version | 2.0.0 | runtime | MIT |
| pathe | 2.0.3 | runtime | MIT |
| perfect-debounce | 2.1.0 | runtime | MIT |
| picocolors | 1.1.1 | runtime | ISC |
| picomatch | 4.0.7 | runtime | MIT |
| pinia | 4.1.0 | runtime | MIT |
| pkg-types | 2.4.0 | runtime | MIT |
| playwright | 1.65.0 | development | Apache-2.0 |
| playwright-core | 1.65.0 | development | Apache-2.0 |
| postcss | 8.5.32 | runtime | MIT |
| prettier | 3.9.10 | development | MIT |
| probe-image-size | 7.5.0 | runtime, optional platform | MIT |
| proxy-from-env | 2.1.0 | runtime | MIT |
| prr | 1.0.1 | runtime, optional platform | MIT |
| qrcode.vue | 3.11.0 | runtime | MIT |
| quansync | 1.0.0 | runtime | MIT |
| readdirp | 5.1.1 | runtime | MIT |
| rolldown | 1.3.0 | runtime | MIT |
| safer-buffer | 2.1.2 | runtime, optional platform | MIT |
| sax | 1.6.1 | runtime | BlueOak-1.0.0 |
| scule | 1.3.0 | runtime | MIT |
| source-map-js | 1.2.2 | runtime, optional platform | BSD-3-Clause |
| source-map-js | 1.2.2 | runtime | BSD-3-Clause |
| style-mod | 4.1.4 | runtime | MIT |
| svgo | 4.1.0 | development | MIT |
| tinyglobby | 0.2.17 | runtime | MIT |
| tslib | 2.9.1 | runtime | 0BSD |
| typescript | 7.0.2 | runtime | Apache-2.0 |
| ufo | 1.6.4 | runtime | MIT |
| unhead | 3.4.3 | runtime | MIT |
| unplugin | 3.7.0 | runtime | MIT |
| unplugin-utils | 0.3.2 | runtime | MIT |
| uuid | 14.0.3 | runtime | MIT |
| vite | 8.3.6 | runtime | MIT |
| vite-svg-loader | 5.1.3 | development | MIT |
| vue | 3.5.45 | runtime | MIT |
| vue-component-type-helpers | 3.3.14 | runtime | MIT |
| vue-i18n | 11.4.18 | runtime | MIT |
| vue-router | 5.4.0 | runtime | MIT |
| w3c-keyname | 2.2.8 | runtime | MIT |
| webpack-virtual-modules | 0.6.2 | runtime | MIT |
| zrender | 6.1.0 | runtime | BSD-3-Clause |
| @homewaf/build-proxy-policy | 1.0.0 | development | LicenseRef-HomeWaf-NC-1.0 |

### Map renderer dependencies

| Package | Version | Usage | License |
|---|---|---|---|
| @mapbox/jsonlint-lines-primitives | 2.0.3 | runtime | MIT |
| @mapbox/point-geometry | 1.1.0 | runtime | ISC |
| @mapbox/tiny-sdf | 2.3.0 | runtime | BSD-2-Clause |
| @mapbox/unitbezier | 1.0.0 | runtime | BSD-2-Clause |
| @mapbox/vector-tile | 3.0.0 | runtime | BSD-3-Clause |
| @maplibre/geojson-vt | 6.3.0 | runtime | ISC |
| @maplibre/maplibre-gl-style-spec | 26.5.1 | runtime | ISC |
| @maplibre/mlt | 1.3.0 | runtime | (MIT OR Apache-2.0) |
| @maplibre/vt-pbf | 4.3.3 | runtime | MIT |
| @types/geojson | 7946.0.16 | runtime | MIT |
| bidi-js | 1.1.0 | runtime | MIT |
| earcut | 3.2.4 | runtime | ISC |
| gl-matrix | 3.4.4 | runtime | MIT |
| json-stringify-pretty-compact | 4.0.0 | runtime | MIT |
| kdbush | 4.1.0 | runtime | ISC |
| maplibre-gl | 6.17.0 | runtime | BSD-3-Clause |
| minimist | 1.2.8 | runtime | MIT |
| murmurhash-js | 1.0.0 | runtime | MIT |
| pbf | 5.1.2 | runtime | BSD-3-Clause |
| potpack | 2.1.0 | runtime | ISC |
| protocol-buffers-schema | 3.6.1 | runtime | MIT |
| quickselect | 3.0.0 | runtime | ISC |
| require-from-string | 2.0.2 | runtime | MIT |
| resolve-protobuf-schema | 2.1.0 | runtime | MIT |
| tinyqueue | 3.0.0 | runtime | ISC |

MapLibre GL JS and its added dependency license texts are retained in [LICENSES/MapLibre-GL-JS.txt](LICENSES/MapLibre-GL-JS.txt).

## Copied code and embedded resources

| Component | License and provenance |
|---|---|
| SamWaf backend and SamWafWeb UI | Apache-2.0; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`upstream`) |
| Historical TDesign Vue Starter template portions | MIT; LICENSES/THIRD_PARTY.txt; runtime framework removed |
| SQLite 3.54.1 through modernc.org/sqlite 1.62.0 (pure Go) | BSD-3-Clause and public-domain/component notices; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`sqlite-provenance`) |
| GORM SQLite 1.6.0 and MySQL 1.6.0 adapters; go-mysql 1.16.0 adapter | MIT; LICENSES/THIRD_PARTY.txt; preserved upstream notices |
| GoCaptcha 2.0.5 and character data | Apache-2.0; internal/thirdparty/gocaptcha/LICENSE and internal/captchaassets/LICENSE-CHARS |
| Grule 1.20.4 local adapter, Git loader omitted | Apache-2.0; internal/thirdparty/grule/LICENSE*; docs/modernization/provenance.json (`grule-provenance`) |
| GoDLP 1.2.15 local adapter, maintained YAML v4 | MIT; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`godlp-provenance`) |
| CAP Go server 1.0.1 local copy | Apache-2.0; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`cap-server-provenance`) |
| LXGW WenKai 1.530, original font | SIL OFL-1.1; LICENSES/THIRD_PARTY.txt; no font is sold by itself |
| CAP widget 0.1.61 / wasm 0.0.8 | Apache-2.0; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`embedded-web`) |
| OWASP CRS 4.30.0 | Apache-2.0; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`crs-provenance`) |
| ip2region IPv4 XDB | Apache-2.0 OR MIT; LICENSES/THIRD_PARTY.txt; docs/modernization/provenance.json (`ip2region-provenance`) |
| ip2region IPv6 XDB | Apache-2.0 OR MIT; LICENSES/ip2region/LICENSE.txt; pinned source, size and hashes in cmd/homewaf/exedata/ip2region_v6.source.json |
| HomeWaf logos, CSS backgrounds and click challenge UI | Original HomeWaf additions; root LICENSE |
| Short weak-password rejection list in wafenginecore/accessgate/password.go | Project-authored list of common trivial passwords; no external password corpus imported; root LICENSE. Shared by account creation and login/honeypot classification. Attempted passwords are not retained in plaintext. |
| Monitoring screen country polygons, borders, country and city coordinates | Derived from Natural Earth 1:110m countries and 1:10m populated places, public domain; web/src/pages/dashboard/screen/README.md documents source and generation |

DOMPurify is used under its Apache-2.0 alternative. Browser/system fonts are referenced by family name and are not redistributed.
The optional user-supplied MaxMind databases and ML models are not included in releases; their operators must obtain the corresponding rights.
Unused upstream crawler IP archives, map data, login imagery, frontend binary bundles and UPX executables are not distributed.

CAP WASM upstream Rust build dependencies are listed in docs/modernization/provenance.json (`cap-wasm-build-dependencies`) with crate checksums and LICENSES/THIRD_PARTY.txt notices. The official released WASM is used without claiming its compiler-internal dependencies were independently upgraded.
