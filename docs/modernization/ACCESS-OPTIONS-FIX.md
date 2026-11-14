# 登录导航、可选访问限制与双栈地区库

## 登录后导航

已复现携带 `login_notice` 的登录响应引发 `h is not a function`：原 TDesign 的通知渲染函数需要接收 `h`，Element Plus 将它当作无参插槽调用，导致登录来源提醒渲染报错、后续页面更新中断。刷新时一次性提醒已从 sessionStorage 移除，因此表现为刷新后恢复。

通知适配器现在为旧渲染回调传入 Vue `h`，同时正确渲染提醒底部操作、映射弹出位置并支持关闭。登录历史按钮改用原生 Element Plus 按钮和 Vue 3 事件。

同时修正了三个相关问题：登录完成后等待会话与菜单初始化；RouterView 使用插槽缓存真实页面组件；退出登录移除已不存在的 `router.history.current` 调用，不在登录导航中异步删除和重建静态路由。

## 访问限制

- 新建 NAS、家庭设备默认不要求额外账号。勾选“独立访问账号”后才出现必填账号、密码。
- “仅允许指定地区”从 `/api/v1/pipeline/regions` 读取实际加载的 IP 数据库，支持搜索国家、省份及重庆市。界面链接到 `/sys/IPLocation`，并显示两种地址协议的数据是否加载。
- “仅允许白名单 IP 组”直接引用统一 IP 组，可选择已有组或内联新建。服务表单不再重复维护 IP / 网段；组内支持 IPv4、IPv6、CIDR、通配符和区间。通过组白名单后继续执行后续防护，不创建可信免检规则。
- 地区与白名单 IP 组满足任意一项即可继续；额外账号启用时还需要登录。选择来源限制会设置未命中来源为拒绝，未识别地区不会被错误放行。
- 常用设置和创建引导复用同一组件，高级配置修改后同步显示；不会将“未命中继续”的旧地区放行名单标成“仅允许地区”。
- API 模板的服务令牌要求、复制配置的原有策略、手动拒绝仍优先于可信规则；新增的 IP 组黑名单与地域黑名单也先于 trusted。

## IPv6 文件与打包

依照用户最终选择，IPv6 `.xdb` 与既有 IPv4 一起由 Git 跟踪。新增 IPv6 数据、来源清单和许可证随本轮控制台与访问策略改动一并提交。

- 资源：`cmd/homewaf/exedata/ip2region_v6.xdb`，37,283,761 字节。
- 来源提交：`200f4cc7a6452843ef515a889c72d295281e8da3`。
- Git blob：`d0eb5b3ea98589c8b6ad77e9e7d5fd5afe600971`。
- SHA-256：`1f0be1ffbbc3e4eeef10f354e01355f22f57ec649a496a17fd602539f3d384c1`。
- 清单：`cmd/homewaf/exedata/ip2region_v6.source.json`；原许可证：`LICENSES/ip2region/LICENSE.txt`。

程序通过 `go:embed` 携带两份数据。构建与开发启动进行离线校验，Docker 构建也只校验随源码复制的文件；不依赖构建时或首次运行时的数据库下载。下载恢复仅由显式 `--download` 触发。

IPv6 加载顺序为用户 `data/ip2region_v6.xdb` 优先、内置库兜底；缺少用户自备 GeoLite2 时可回退到内置 IPv6。内置文件采用已知的 `opensource` 格式，避免旧配置误解析。发行归档补齐数据库许可证和来源清单。

## 验证

- 类型检查与完整前后端构建通过，输出 `artifacts/HomeWaf64.exe`。
- `go test ./iplocation ./wafpipeline -count=1` 通过，包括离线 IPv6、用户文件优先、GeoLite2 缺失回退、IPv4/IPv6 重庆限制和未解析地址拒绝。
- 实际内置库查询到 `2001:250:2401:: → 中国 / 重庆市`；IPv4 与 IPv6 目录具有相同的重庆选项。
- 登录浏览器测试覆盖普通登录、登录提醒、来源变化的历史入口、强制改密、退出重登，以及无需刷新切换页面；最终组合用例 6 项通过、页面错误 0。
- 接入浏览器回归 16 组通过、页面错误 0，覆盖无账号地区限制、IPv4/IPv6 网段、未选地区校验、凭据隔离及已有高级操作。
- 浏览器使用隔离接口响应，未修改真实服务配置。数据库与防护判断另由真实 Go 数据和运行时测试验证。

记录位于 `artifacts/login-navigation/combined-final.log`、`artifacts/optional-access-browser.log`、`artifacts/access-regions-go-tests.log` 和 `artifacts/access-fixes-build.log`。桌面和手机截图位于 `artifacts/pipeline-ui/optional-access-*.png`。


## 统一 IP 组与地域黑名单（2026-11-14）

### 配置入口

- 地区从 IPv4 / IPv6 当前启用库的统一目录选择国家或省市，例如“中国 / 全部地区”“中国 / 重庆市”；无需手填网段。
- IP 地址统一在 IP 组维护。可在服务表单中直接选择、创建并选用，或打开 IP 组管理；常见内网地址可一键填入新组。
- IP 黑名单包含三个可组合来源：禁止中国大陆以外 IP、指定禁止地区、黑名单 IP 组。
- 黑白名单同时命中时黑名单优先。大陆以外规则涵盖港澳台，未知公网地址以及数据库不可用时的公网来源均拒绝；私网和回环不参与地域判断，仍受名单、认证等规则约束。
- 取消黑名单会清除其生效配置；旧版单独地址规则继续保留在高级设置，不静默重写或删除。

### 与现有防护的联动

- `allow_groups` / `deny_groups` 保存稳定组引用，运行时读取统一的原子快照。新增、删除、批量导入组内地址无需重新保存每个服务，下一次请求即可使用更新后的组。
- 组管理展示引用服务，并支持跳转到该服务配置。服务仍在引用时，即使 `force=1` 也不能删除组；需显式移除服务引用。保存引用与删除组之间串行保护，避免并发删除导致静默放行。
- 丢失或不可用的组引用返回 503，不把黑名单失效视为放行。API 保存时拒绝不存在的组。
- 请求顺序：访问总开关 → 手动 IP / IP 组 / 地域黑名单 → 现有 trusted 及自动保护逻辑 → 地区或组白名单 → 已开启的 QPS / CC / 登录门 / 攻击规则 → 上游。
- 新组白名单是允许继续检测，不会绕过自动封禁、威胁订阅、服务锁定、蜜罐、登录认证和攻击检测。明确的黑名单拒绝先于高级 trusted 规则。
- 拒绝结果进入原有请求日志、trace、今日拦截计数。界面为新增判定原因提供中文说明，保留原始原因码方便排查。
- 服务卡片和保存确认展示相同的来源策略；常用设置可直接进入“验证来源与防护”。IP 数据库切换沿用现有 generation 机制清除地域缓存。
- 观察模式保留来源限制。模板导出移除本地组引用、保留地域规则；模板应用保留目标服务现有组引用。复制服务默认不复制地址名单与组引用，只有显式勾选才复制。

### API 增补

`policy.ip_group` 增加：

- `allow_groups: string[]`、`deny_groups: string[]`：统一 IP 组短码，合计最多 256 个引用。
- `denied_regions: Region[]`：拒绝的国家 / 省市，与 `regions` 使用相同目录。
- `block_outside_mainland: boolean`：大陆以外及未知公网来源拒绝开关。

`POST /api/v1/wafhost/ipgroup/add` 增加可选 `initial_ips: string[]`。组和初始地址在同一数据库事务保存，复用原 IP 组校验（包括全通配保护），任意地址无效时不留下空组；保留原有只创建空组接口兼容性。`GET /api/v1/wafhost/ipgroup/refs` 增加 `ports`，每项包含 `port`、`name`、`allow`、`block`。

### 本轮验证

- 全量 `go test ./wafpipeline ./iplocation ./service/waf_service ./wafenginecore/ipset -count=1` 通过。
- 新增真实运行时用例覆盖双栈组匹配、组热更新、重叠黑白名单、trusted 优先级、未知归属地、港澳台、数据库不可用、数据库代际更新、登录门与攻击规则联动、自动封禁、日志和指标。
- 新增数据库用例覆盖初始地址事务、IPv4 / IPv6、引用显示、删除保护、无效引用拒绝。
- 前端类型检查、13 项逻辑测试和 27 项应用流程检查通过；OpenAPI 从代码重新生成，新增字段纳入校验。
- 浏览器验证使用隔离加密接口，不修改真实服务配置；检查桌面、1024 / 768 / 390 / 320 宽度、创建失败保留输入、组引用保存与回显、黑名单组合、组管理与服务跳转、删除保护及日志原因展示。
- 本轮浏览器回归 18 组通过，页面错误 0；登录 / 强制改密 / 导航组合回归 6 项通过。完整构建输出 `artifacts/HomeWaf64.exe`，仍有已有的大分块警告。构建与登录测试并行时发生过开发服热更新中断，构建结束后串行重跑登录测试通过。
- 日志：`artifacts/shared-access-go-tests.log`、`artifacts/shared-access-browser.log`、`artifacts/shared-access-openapi.log`。界面截图仍位于 `artifacts/pipeline-ui/optional-access-*.png`。

本轮沿用已有控制台视觉系统，仅修改访问配置和关联入口。没有重新宣称整个控制台通过营销站型 skill gate；历史 `GATE: OPEN 40 (rounds used up)` 与适用范围见 `CONSOLE-REDESIGN-QA.md`。
