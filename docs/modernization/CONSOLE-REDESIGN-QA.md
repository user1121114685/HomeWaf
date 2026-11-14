GATE: OPEN 40 (rounds used up)

# 控制台改版检查记录

以下保留最初改版的视觉检查记录。后续登录导航修复、可选账号与来源限制、IPv6 默认内置及 Git 跟踪方式见 [ACCESS-OPTIONS-FIX.md](ACCESS-OPTIONS-FIX.md)。

## 完成内容

- 新增工作台和可搜索的常用 / 全部功能导航。默认登录入口改为工作台，原有页面路径保留。
- 新建服务改为三个步骤。公开网站只需填写一个现有地址；私有服务按场景增加访问账号。自动带入名称、推荐端口和后端场景模板。
- 可选参数逐步展开，按协议显示证书；失败保留输入并定位字段，忙碌时阻止重复提交。切换到不需要认证的场景后，不提交之前隐藏的访问凭据。
- 默认服务卡片，搜索、批量表格、原有迁移与高级功能仍可用。已接入服务先显示常用设置。
- 保存后提供明确后续入口；启用自动预检，观察模式保留访问与认证限制，API 后续操作直接进入令牌面板。
- 手机导航可展开、通过 Escape 关闭并恢复焦点；手机表单主操作固定在滚动区域底部。

## 验证结果

- `npm --prefix web run typecheck`：通过。
- `npm --prefix web run build`：通过，生成 JavaScript 语法校验通过。构建仍有大于 500kB 的分块提示。
- `node --test web/scripts/verify-onboarding.test.mjs web/scripts/verify-pipeline.test.mjs`：10 项通过。
- `npm --prefix web test`：27 项通过。合计 37 项逻辑检查。
- `verify-pipeline-browser.mjs`：15 组检查通过，页面脚本异常 0。覆盖空状态、完整导航搜索、仅一个可见输入的公开网站接入、字段错误、连接失败、保存重试、防重复提交、场景切换后的凭据处理、复制策略保留、API 令牌、观察启动、账号、插件、批量部分失败、模板校验、旧站点迁移。
- 真实 Chrome 检查 1440、1024、820、768、390、320px；新手路径无文档横向溢出。已检查浅色、实际系统深色模式、减少动态效果和键盘导航。高级批量表格有自身横向滚动。
- 所有浏览器写入使用隔离的加密接口夹具，模板来自 Go 内置模板生成器。没有操作真实防护实例。TCP 可达性、实际证书、DNS、路由器映射和真实后端部署不在本次实测范围内。
- 构建包含最终业务逻辑修正；该修正仅限制提交当前场景所需的凭据，不改变最后一次视觉检查的布局。

## Skill 检查与适用范围

Kind：software。配色延续原有蓝色盾牌与可切换明暗主题；抽签首选 green-black-bone 未采用，原因见设计文档。字体沿用可离线使用的中文字体。Voice：technical-dry，描述真实状态、动作与下一步。真实 UI 为视觉主体；没有新增照片、替身素材、客户事实、宣传效果或营销滚动场景。没有待补照片、canonical 或表单服务地址，管理页面保留 noindex。

已运行 skill 的 consistency、layout、perf、shoot 和三轮 gate，没有修改 skill 文件，没有设置豁免，也没有将检查失败改称通过。营销站检查器对 hash 路由、嵌套后台容器、数据指标字号与宣传式整屏章节有不适配项，保留原始结果。

- 最终 gate：40 个开放项、13 个 WARN，三轮结束；最终开放项中没有 text-contrast、focus-visible 或 a11y-name。
- 完整 consistency：覆盖五个尺寸；全量检查发生在最后的深色颜色修正之前，记录为 111 FAIL / 24 WARN，重复宽度计数不可与最终 gate 去重计数直接比较。最终修正后的颜色与焦点由最后 gate 和截图检查。
- layout 检查：因后台没有营销站 `.wrap`，工具返回容器 finding 后在 `r.sections` 遍历处报 TypeError；未将这一工具错误标为通过。五个尺寸另由实际浏览器截图和溢出断言验证。
- perf：桌面与手机合计 4 FAIL / 2 WARN，均涉及 JS/CSS 体积：JS parsed 约 1917KiB、CSS 约 725KiB。两个尺寸的坏请求、LCP、阻塞时间、布局偏移和 WebGL 数量检查均通过。这是仍需专项拆分的打包问题。
- shoot：已生成 1440、1024、820、768、390px 工作台、1440px 服务卡片和 390px 创建引导截图。该工具会主动将 bottom-sticky 改为 relative，因此它的创建引导截图不显示真实固定操作区；另以原生浏览器截图及 `toBeInViewport` 断言验证手机下一步按钮，未改写工具掩盖差异。

## 原始记录

- `artifacts/console-redesign/build.log`
- `artifacts/console-redesign/browser-regression.log` 和 `artifacts/pipeline-ui/report.json`
- `artifacts/console-redesign/logic-tests.log`
- `artifacts/console-redesign/consistency.log`、`layout.log`、`perf.log`、`gate-final.log`
- `artifacts/console-redesign/*.png`：skill 截图；`artifacts/pipeline-ui/*.png`：原生交互截图。所有服务信息均为测试数据。

## 最终开放项（逐项保留）

没有使用 waivers。以下记录 checker 原始定位，并注明为何保留。

### 1. shader-lint

位置：`assets/router-CIu3H3ej.js "enum"`

检查结果：a shader uses "enum" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 2. shader-lint

位置：`assets/router-CIu3H3ej.js "default"`

检查结果：a shader uses "default" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 3. shader-lint

位置：`assets/router-CIu3H3ej.js "this"`

检查结果：a shader uses "this" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 4. shader-lint

位置：`assets/router-CIu3H3ej.js "__name"`

检查结果：a shader uses "__name" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 5. shader-lint

位置：`assets/router-CIu3H3ej.js "inline"`

检查结果：a shader uses "inline" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 6. shader-lint

位置：`assets/router-CIu3H3ej.js "filter"`

检查结果：a shader uses "filter" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 7. shader-lint

位置：`assets/router-CIu3H3ej.js "class"`

检查结果：a shader uses "class" as a name (line 1: ))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u00A0-\uD7FF\uF900-): GLSL ES reserves it, so a strict GPU rejects the whole shader and the picture stays empty

处理说明：检查器将包含校验正则与语言关键字的压缩 JavaScript bundle 当作 GLSL；该工作台未创建 WebGL 上下文。未改动依赖或检查器来消除误报。

### 8. no-visual

位置：`section.el-container > section.el-container > main.el-main > section.el-container.tdesign-starter-layout "网站与家庭服务防护"`

检查结果：no picture, effect or mock in this section; a feature section needs a visual

处理说明：这是可以操作的实际管理 UI，检查器要求额外图片、视觉 mock 或宣传效果。

### 9. button-styles

位置：`(one screen)`

检查结果：5 button styles on one screen: soft, fill #191f29 ("工作台"); soft, outline #4c4d4f ("监控大屏"); pill, fill #141414, outline #4c4d4f ("CPU 0% 内存 0%"); soft, fill #141414, outline #4c4d4f ("刷新状态"); soft, fill #4582e6, outline #4582e6 ("继续启用服务"); the odd ones out: "CPU 0% 内存 0%", "刷新状态", "继续启用服务"

处理说明：后台的导航选中态、监控触发器和动作按钮被合并计入营销按钮样式。主要任务按钮已统一，既有监控工具仍保留。

### 10. button-styles

位置：`(the page)`

检查结果：5 button styles on the page: soft, fill #191f29 x1 ("工作台"); soft, outline #4c4d4f x1 ("监控大屏"); pill, fill #141414, outline #4c4d4f x1 ("CPU 0% 内存 0%"); soft, fill #141414, outline #4c4d4f x1 ("刷新状态"); soft, fill #4582e6, outline #4582e6 x1 ("继续启用服务")

处理说明：后台的导航选中态、监控触发器和动作按钮被合并计入营销按钮样式。主要任务按钮已统一，既有监控工具仍保留。

### 11. button-styles

位置：`div.tdesign-starter-header-menu-fixed.tdesign-starter-header-menu-fixed-side > div.hw-menu-operations > div.operations-container > a.screen-entry "监控大屏"`

检查结果：a button pair at two heights: "监控大屏" 32px beside "CPU 0% 内存 0%" 40px

处理说明：后台的导航选中态、监控触发器和动作按钮被合并计入营销按钮样式。主要任务按钮已统一，既有监控工具仍保留。

### 12. copy-headline

位置：`main#workspace-main > header.workbench-heading > div > h1 "网站与家庭服务防护"`

检查结果："网站与家庭服务防护" does not name what is sold (homewaf, from flags)

处理说明：控制台标题说明实际用途，品牌已经显示在导航和前导文字；该检查要求出售品类包含命令行传入的 HomeWaf 一词。

### 13. heading-scale

位置：`main#workspace-main > section.welcome-panel > div.welcome-copy > h2 "还有服务等待启用"`

检查结果：h2 headings in 2 sizes, 18px ("我的服务") to 26px (x1.44)

处理说明：工作台的引导标题、区块标题和数据指标使用不同视觉层级；该检查按营销页统一 h2 和指标上限判断。

### 14. heading-scale

位置：`main#workspace-main > div.workspace-metrics > div:nth-of-type(1) > strong "2个"`

检查结果：the figure "2个" is set at 28px, larger than the section headings (h2 18px); 3 more like it

处理说明：工作台的引导标题、区块标题和数据指标使用不同视觉层级；该检查按营销页统一 h2 和指标上限判断。

### 15. links

位置：`div.tdesign-starter-side-nav.hw-menu > ul.el-menu.el-menu--vertical > div.workspace-nav > a.workspace-link.nav-help "使用指南"`

检查结果："使用指南" goes to #/help, which is not on this page

处理说明：这是有效的 Vue hash 路由，检查器按同页锚点判断。对应路由由应用路由表提供，核心跳转已做浏览器回归。

### 16. links

位置：`section.welcome-panel > ol.start-checklist > li:nth-of-type(3) > a "查看记录"`

检查结果："查看记录" goes to #/waf-pipeline/ports?inspect=5000, which is not on this page

处理说明：这是有效的 Vue hash 路由，检查器按同页锚点判断。对应路由由应用路由表提供，核心跳转已做浏览器回归。

### 17. links

位置：`main#workspace-main > section.workspace-services > div.workspace-service-list > a.workspace-service:nth-of-type(1) "家庭相册 http://192.168.1.20:5000 HTTP :5000"`

检查结果："家庭相册 http://192.168.1.20:5000 HTTP :5000" goes to #/waf-pipeline/ports?inspect=5000, which is not on this page

处理说明：这是有效的 Vue hash 路由，检查器按同页锚点判断。对应路由由应用路由表提供，核心跳转已做浏览器回归。

### 18. links

位置：`main#workspace-main > section.workspace-services > div.workspace-service-list > a.workspace-service:nth-of-type(2) "个人博客 http://192.168.1.30:8080 HTTP :8080"`

检查结果："个人博客 http://192.168.1.30:8080 HTTP :8080" goes to #/waf-pipeline/ports?inspect=8080, which is not on this page

处理说明：这是有效的 Vue hash 路由，检查器按同页锚点判断。对应路由由应用路由表提供，核心跳转已做浏览器回归。

### 19. links

位置：`main#workspace-main > section.workspace-resources > div.resource-links > a:nth-of-type(3) "阅读使用指南 了解接入方式和配置说明。"`

检查结果："阅读使用指南 了解接入方式和配置说明。" goes to #/help, which is not on this page

处理说明：这是有效的 Vue hash 路由，检查器按同页锚点判断。对应路由由应用路由表提供，核心跳转已做浏览器回归。

### 20. focus-ring

位置：`.el-date-table td:focus`

检查结果：".el-date-table td:focus" in style-pCqV6OtP.css removes the focus outline and sets nothing in its place

处理说明：命中 Element Plus 自带日期/月/季度/年选择器样式中的静态声明，并非本次新手表单使用的控件。本次按钮和链接的实际可见焦点检查已通过；未为清除静态告警修改依赖包。

### 21. focus-ring

位置：`.el-month-table td:focus-visible`

检查结果：".el-month-table td:focus-visible" in style-pCqV6OtP.css removes the focus outline and sets nothing in its place

处理说明：命中 Element Plus 自带日期/月/季度/年选择器样式中的静态声明，并非本次新手表单使用的控件。本次按钮和链接的实际可见焦点检查已通过；未为清除静态告警修改依赖包。

### 22. focus-ring

位置：`.el-quarter-table td:focus-visible`

检查结果：".el-quarter-table td:focus-visible" in style-pCqV6OtP.css removes the focus outline and sets nothing in its place

处理说明：命中 Element Plus 自带日期/月/季度/年选择器样式中的静态声明，并非本次新手表单使用的控件。本次按钮和链接的实际可见焦点检查已通过；未为清除静态告警修改依赖包。

### 23. focus-ring

位置：`.el-year-table td:focus-visible`

检查结果：".el-year-table td:focus-visible" in style-pCqV6OtP.css removes the focus outline and sets nothing in its place

处理说明：命中 Element Plus 自带日期/月/季度/年选择器样式中的静态声明，并非本次新手表单使用的控件。本次按钮和链接的实际可见焦点检查已通过；未为清除静态告警修改依赖包。

### 24. moment-none

位置：`(the page)`

检查结果：a software page (--type) deals no moment: every kind carries one, two on a long page (moments.md)

处理说明：没有为管理后台添加营销滚动场景，以保留短路径与表单可操作性。

### 25. hero-screen

位置：`section.el-container > section.el-container > main.el-main > section.el-container.tdesign-starter-layout "HomeWaf 控制台网站与家庭服务防护从接入一个服务开始，在这里确认状态和下一"`

检查结果：the hero's content is 335px taller than the hero (scrollHeight 1171, clientHeight 836): it spills out or scrolls inside it (SR-1)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 26. hero-screen

位置：`main#workspace-main > section.workspace-services > div.workspace-service-list > a.workspace-service:nth-of-type(2) "个人博客http://192.168.1.30:8080HTTP :8080待启"`

检查结果："个人博客http://192.168.1.30:8080HTTP" ends at 900px, below the hero's content line at 876px (the 900px screen less 24px) (SR-1)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 27. hero-screen

位置：`div.workspace-service-list > a.workspace-service:nth-of-type(2) > div.service-identity > h3 "个人博客"`

检查结果："个人博客" ends at 900px, below the hero's content line at 876px (the 900px screen less 24px) (SR-1)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 28. section-height

位置：`section.el-container > section.el-container > main.el-main > section.el-container.tdesign-starter-layout "HomeWaf 控制台网站与家庭服务防护从接入一个服务开始，在这里确认状态和下一"`

检查结果：the section starts at 64px, 64px below the screen grid (0 x 900px): sections start on whole screens (SR-2)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 29. one-section

位置：`main#workspace-main > section.welcome-panel "接入进度还有服务等待启用已保存 2 个服务，其中 1 个尚未启用。确认配置后，继"`

检查结果：a band: a box 95% of the section wide on a ground of its own that holds the heading or the intro: the section's own ground is the band (SR-4)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 30. one-heading

位置：`main#workspace-main > section.welcome-panel > div.welcome-copy > h2 "还有服务等待启用"`

检查结果：the hero holds an h2 (3): its heading is the h1, and an h2 opens the next section (SR-5)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 31. container-edges

位置：`div.console-shell.dark > section.el-container > section.el-container > header.el-header "监控大屏CPU0%内存0%"`

检查结果：the nav's content runs 268px from the left and 32px from the right (read from its outermost blocks: no .wrap): the page's container starts at 232px; 268 and 32px are not one margin; the margin is 64-120px at 1440px (one container, equal margins, 2px either way) (SR-6)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 32. container-edges

位置：`section.el-container > section.el-container > main.el-main > section.el-container.tdesign-starter-layout "HomeWaf 控制台网站与家庭服务防护从接入一个服务开始，在这里确认状态和下一"`

检查结果：the content of section 1 runs 232px from the left and 0px from the right (read from its outermost blocks: no .wrap): 232 and 0px are not one margin; the margin is 64-120px at 1440px (one container, equal margins, 2px either way) (SR-6)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 33. container-edges

位置：`div#app "跳到主要内容HomeWaf日常管理工作台服务接入访问记录安全事件流量概览策略模板"`

检查结果：a <div> at the top of the page, outside header.nav, main, footer.closing and nav.actionbar (SR-6)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 34. container-edges

位置：`main#workspace-main > section.welcome-panel > div.welcome-copy > h2 "还有服务等待启用"`

检查结果：3 text blocks start on a left line at x 301, 69px from the container's left: text starts on the container's left, or on one split line (x 745, 848, 951) (also "已保存 2 个服务，其中 1 个尚未启用。确认配置后，继续完成接", "配置保存后，需要单独启用。") (SR-6)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 35. container-edges

位置：`main#workspace-main > section.welcome-panel "接入进度还有服务等待启用已保存 2 个服务，其中 1 个尚未启用。确认配置后，继"`

检查结果：a box over half the container wide runs 264-1408px, not on the container's lines at 232-1440px (SR-6)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 36. container-edges

位置：`main#workspace-main > section.workspace-services > div.workspace-service-list "家庭相册http://192.168.1.20:5000HTTP :5000运行"`

检查结果：a box over half the container wide runs 264-1408px, not on the container's lines at 232-1440px (SR-6)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 37. row-lines

位置：`main#workspace-main > div.workspace-metrics "已配置服务2个正在监听1个等待启用1个今日拦截请求0次"`

检查结果：a row of 4 items: its first item starts at 264px, the container's left line at 232px; its last item ends at 1408px, the container's right line at 1440px (2px either way) (SR-7)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 38. split-pair

位置：`main#workspace-main > section.welcome-panel "接入进度还有服务等待启用已保存 2 个服务，其中 1 个尚未启用。确认配置后，继"`

检查结果：the two columns of the split start 38px apart at the top (centred on each other): a split's columns are top-aligned, 8px either way (SR-8)

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

### 39. menu-toggle

位置：`div.tdesign-starter-header-menu-fixed.tdesign-starter-header-menu-fixed-side:nth-of-type(4) > div.hw-menu-logo:nth-of-type(1) > div.header-operate-left > button.el-button.el-button--info "展开侧栏"`

检查结果：the menu button keeps aria-expanded="false" after a click: its state never changes (system.js flips it on .nav__toggle)

处理说明：完整 consistency 的 menu-toggle 通过；快速 gate 仍报告该项。真实浏览器独立断言 aria-expanded true/false、导航可见和 Escape 关闭均通过，保留检查器差异。

### 40. container

位置：`page`

检查结果：no .wrap

处理说明：该项按营销站的整屏 hero、统一 wrap 与章节网格计算；后台保留侧栏、自然内容高度和独立滚动容器。实际尺寸和交互另行验证。

## 保留的 WARN

- `copy-headline`：main#workspace-main > header.workbench-heading > div > h1 "网站与家庭服务防护" — the h1 has 0 words (2 to 8 read at a glance)。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `copy-buttons`：div.tdesign-starter-header-menu-fixed.tdesign-starter-header-menu-fixed-side > div.hw-menu-operations > div.operations-container > div.system-monitor-trigger.el-tooltip__trigger "CPU 0% 内存 0%" — "CPU 0% 内存 0%" does not start with a verb (book, buy, try, start, get, see...)。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `heading-scale`：ol.start-checklist > li.done:nth-of-type(1) > div > h3 "填写服务地址" — the h3s (14px) are only 1.29x smaller than the h2s (18px): one level a clear step (1.3x or more) below the other。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `nav-family`：div.console-shell.dark > section.el-container > section.el-container > header.el-header — the nav's links and actions use 2 type families (system-ui, Arial)。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `label-centre`：ul.el-menu.el-menu--vertical > div.workspace-nav > nav > a.router-link-active.router-link-exact-active "工作台" — its label sits 85px off-centre (12px of room at the left, 97px at the right)。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `stranded-action`：main#workspace-main > header.workbench-heading > button.el-button "刷新状态" — stands 724px across from the nearest text of its block ("网站与家庭服务防护")。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `button-contrast`：ul.el-menu.el-menu--vertical > div.workspace-nav > nav > a.router-link-active.router-link-exact-active "工作台" — the button "工作台" (#191f29) stands only 1.11:1 off the ground around it: it reads as part of the ground, or as switched off。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `action-count`：main#workspace-main > section.welcome-panel > div.welcome-copy > button.el-button.el-button--primary — the main action "接入服务" is on 5 buttons and links: the nav, the hero and the closing call carry it; elsewhere a section has its own action。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `skip-link`：div#app > div > div.console-shell.dark:nth-of-type(1) > a.workspace-skip "跳到主要内容" — the first Tab stop is "跳到主要内容", not a skip link: a keyboard user tabs through the whole nav on every visit。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `share-meta`：head — the head lacks og:title, og:description, og:image, twitter:card, theme-color: a shared link shows no picture or title of its own。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `tap-size`：div.console-shell.dark > div:nth-of-type(2) > aside.hw-sticky-tool > button.el-button.hw-button — a 46x32px tap target: under the 44px a phone needs。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `heading-scale`：main#workspace-main > section.welcome-panel > div.welcome-copy > h2 "还有服务等待启用" — h2 headings in 2 sizes, 18px ("我的服务") to 23px (x1.28)。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
- `copy-stat-strip`：copy x1: "0% / 0% / 2 / 1 / 1 / 0" — stat strip x1。保留原因：沿用后台导航与原生控件语义，不改造成营销站骨架。
