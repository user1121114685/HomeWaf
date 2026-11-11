// HomeWaf modification (2026-11-11): immutable runtime configuration.
package global

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

// RuntimeSettings contains values only; callers receive private copies.
// Business-port policies remain in their own snapshots; this is legacy/admin configuration.
type RuntimeSettings struct {
	// *****记录参数配置***************
	// 是否将web日志持久化到日志库（默认1 ） 1 持久化入库 0 不入库
	GCONFIG_LOG_PERSIST_ENABLED int64
	// 限制记录最大请求的body长度 record_max_req_body_length
	GCONFIG_RECORD_MAX_BODY_LENGTH int64
	// 限制记录最大响应的body长度 record_max_rep_body_length
	GCONFIG_RECORD_MAX_RES_BODY_LENGTH int64
	// 是否记录响应记录 record_resp
	GCONFIG_RECORD_RESP int64
	// 配置获取IP头信息
	GCONFIG_RECORD_PROXY_HEADER string
	// 管理端获取客户端IP头信息（逗号分隔按优先级），留空则直接取网络层IP；仅当直连来源属可信代理时才采信
	GCONFIG_MANAGE_PROXY_HEADER string
	// 管理端可信代理网段（CIDR/IP，逗号分隔）；仅当直连来源在此网段内才采信上面的代理头，留空=不信任任何代理头
	GCONFIG_MANAGE_TRUSTED_PROXIES string
	// 管理端引用的CDN厂商码（管理端也挂CDN时）；设置后GetManageClientIP会额外信任该厂商中心库最新回源段，自动跟随更新
	GCONFIG_MANAGE_CDN_PROVIDER string
	// SSL证书导出允许的额外目录（绝对路径，逗号分隔）；只从config.yml读、不进DB/API。内置默认data/ssl_export恒允许，其余目录须运营方在此声明，攻击者/OpenAPI Key改不了
	GCONFIG_SSL_EXPORT_ALLOWED_DIRS string
	// 每实例静态数据加密密钥(DEK)文件的自管路径；只从config.yml读、不进DB/API。留空=默认.homewaf-keys/data_key(首启自动生成,0600)
	GCONFIG_DATA_KEY_FILE string
	// 每实例传输密钥(X25519)文件的自管路径；只从config.yml读、不进DB/API。留空=默认data/.keys/comm_key(首启自动生成,0600)
	GCONFIG_COMM_KEY_FILE string
	// 是否保留 legacy 传输通道(旧客户端用内置密钥的CBC报文)。默认开，任何版本不自动翻转；确认无旧独立前端/旧App后可手动关
	GCONFIG_COMM_LEGACY_KEY bool
	// 批量任务本地来源允许的额外目录（绝对路径，逗号分隔）；只从config.yml读、不进DB/API。内置默认data/import恒允许，留空=只允许该默认目录
	GCONFIG_BATCH_IMPORT_ALLOWED_DIRS string
	// 用户可配的对外拉取地址允许清单（主机名/IP/CIDR，逗号分隔）；只从config.yml读、不进DB/API。留空=只允许公网目标；内网镜像源须运营方在此带外声明
	GCONFIG_OUTBOUND_ALLOWED_HOSTS string
	// 是否每天凌晨3点自动加载ssl证书
	GCONFIG_RECORD_AUTO_LOAD_SSL int64
	// kafka 是否激活
	GCONFIG_RECORD_KAFKA_ENABLE int64
	// kafka url地址
	GCONFIG_RECORD_KAFKA_URL string
	// kafka topic
	GCONFIG_RECORD_KAFKA_TOPIC string
	// 80跳转https的方式
	GCONFIG_RECORD_REDIRECT_HTTPS_CODE int64
	// 是否启用HTTPS重定向服务器 0关闭 1开启
	GCONFIG_ENABLE_HTTPS_REDIRECT int64
	// 反向代理最大跳数，超过则判定为环路并拦截；0=关闭环路检测 proxy_loop_max_hop
	GCONFIG_RECORD_PROXY_LOOP_MAX_HOP int64
	// 登录周期里错误最大次数
	GCONFIG_RECORD_LOGIN_MAX_ERROR_TIME int64
	// 登录错误记录周期 单位分钟最小1
	GCONFIG_RECORD_LOGIN_LIMIT_MINTUTES int64
	// 真实IP来源诊断探针：默认关闭。开启后才在内存里采样"最近到达的请求头"供管理端排查，
	// 关闭时业务请求路径上只多一次整型比较；样本只存内存、不落库、不外发。
	// 真实IP来源探针开关 1开启 0关闭
	GCONFIG_IPPROBE_ENABLE int64
	// 统一访问认证(Access 模式)
	// 开启后，被 WAF 代理的站点默认都要先登录才能访问；站点可用 access_json 的三态单独强制开/关。
	// 默认 0（关闭）—— 存量用户升级后行为完全不变，这一点不能改。
	// 统一访问认证总开关 1开启 0关闭
	GCONFIG_ACCESS_ENABLE int64
	// 访问认证审计日志保留天数
	GCONFIG_ACCESS_AUDIT_RETAIN_DAYS int64
	// GCONFIG_ACCESS_FORCE_DISABLE 是自救开关，只能从 conf/config.yml 的 security.access_force_disable
	// 或环境变量 SAMWAF_ACCESS_DISABLE 设置，管理端改不了。
	// 存在的意义：用户把管理端也反代进了 WAF 并开启 Access，一旦配错就会把自己彻底锁在外面，
	// 那时管理端已经进不去，只剩「改配置文件 + 重启」这一条路。
	GCONFIG_ACCESS_FORCE_DISABLE bool
	// 是否进行系统统计数据推送
	// 是否启用系统统计数据推送 1启用 0禁用
	GCONFIG_ENABLE_SYSTEM_STATS_PUSH int64
	// Proxy Protocol 开关（0：关闭；1：开启）
	GCONFIG_ENABLE_PROXY_PROTOCOL int64
	// 指纹认证相关配置
	// 是否启用设备指纹认证 1启用 0禁用
	GCONFIG_ENABLE_DEVICE_FINGERPRINT int64
	// 是否启用严格IP绑定 1启用 0禁用(默认关：启用后令牌绑定登录时真实IP，IP变化需重登；反代后需先配可信代理网段。避免动态IP/多出口LB场景反复掉线)
	GCONFIG_ENABLE_STRICT_IP_BINDING int64
	// 防重放攻击开关 1启用 0禁用
	GCONFIG_ENABLE_REPLAY_PROTECT int64
	// 容器环境是否允许应用内升级。默认 0=拦截：容器里的可执行文件在镜像可写层，
	// 升级只在本次容器生命周期内有效，容器一重建就回退成镜像里的旧版本，
	// 而数据库已被新版本迁移且回不去，会形成"旧程序 + 新库"的静默不一致状态。
	// 确实把二进制挂到卷里的高级用户可置 1 自行放行。
	GCONFIG_ALLOW_CONTAINER_SELFUPDATE int64
	// 启动OWASP数据检测
	GCONFIG_RECORD_ENABLE_OWASP int64
	// OWASP 检测引擎工作模式: On(拦截) / DetectionOnly(观察/仅记录) / Off(关闭)
	GCONFIG_OWASP_MODE string
	// AI智能检测总开关 1启用 0关闭（需先在AI模型管理上传模型包）
	GCONFIG_AI_ENABLE int64
	// AI检测工作模式: observe(仅记录/观察) / block(达到拦截阈值则拦截)
	GCONFIG_AI_MODE string
	// 请求体深度检测模式（D3/S1 新增的 formValue/JSON 逐值 XSS/SQLi/RCE 检测）：
	// observe(默认,命中只在日志标记不拦截) / block(命中即拦) / off(不做请求体深检)。
	// 默认 observe：请求体逐值检测对正常 JSON/表单流量误报较高(libinjection 本为单值设计)，
	// 先观察、用日志确认无误后再切 block；查询串检测不受此开关影响(始终拦截)。
	GCONFIG_BODY_DETECT_MODE string
	// 请求体深检字段排除清单（逗号分隔的字段名，不区分大小写，默认空）。这些字段名下的表单值/
	// JSON 值(匹配直接父键名)跳过 XSS/SQLi/RCE 逐值检测——用于放行已知携带富文本/HTML/代码的
	// 合法字段(如 content,html,markdown)，压 block 模式误报(E)。观察日志里看是哪些字段误报再填。
	GCONFIG_BODY_DETECT_FIELD_EXCLUDE string
	// 自定义规则在检测链里的位置 0:默认(排在CC之后) 1:规则优先(排在黑名单之后、Bot/SQLI/XSS等之前)
	// 规则优先模式下，规则的放行动作(RF.Allow)才能跳过 Bot/SQLI/XSS/扫描/RCE/目录穿越/CC 这些检测
	GCONFIG_RULE_CHAIN_MODE int64
	// OWASP 入站 anomaly score 阈值(官方默认 5,我们宽松到 7)
	GCONFIG_OWASP_BLOCK_THRESHOLD int64
	// 启动80端口服务（为自动申请证书使用 HTTP文件验证类型，DNS验证不需要）
	GCONFIG_RECORD_ENABLE_HTTP_80 int64
	// 提前多少天进行自动申请
	GCONFIG_RECORD_SSLOrder_EXPIRE_DAY int64
	// 获取IP证书时的IP地址
	GCONFIG_RECORD_SSL_IP_CERT_IP string
	// IP证书提前多少天进行自动申请
	GCONFIG_RECORD_SSL_IP_EXPIRE_DAY int64
	// ssl文件验证：本地无挑战文件且后端对该路径返回非404/301/302时是否写告警 1告警 0不告警
	GCONFIG_RECORD_SSLHTTP_CHECK int64
	// 注意：本地有挑战文件时始终优先使用本地文件，与本项无关
	// SSL过期检测任务开始前，是否自动把已配置的SSL主机同步进过期检测列表 1同步(默认) 0不同步
	// 关掉之后过期检测只查用户自己在列表里维护的域名，不会再被主机配置自动塞回来（手动点【同步主机】按钮仍然可用）
	GCONFIG_SSL_EXPIRE_AUTO_SYNC_HOST int64
	// ssl最低版本
	GCONFIG_RECORD_SSLMinVerson string
	// ssl最大版本
	GCONFIG_RECORD_SSLMaxVerson string
	// 连接超时 默认30s
	GCONFIG_RECORD_CONNECT_TIME_OUT int64
	// 保持活动超时 默认30s
	GCONFIG_RECORD_KEEPALIVE_TIME_OUT int64
	// 升级/停止时连接优雅排空超时(秒) 默认30s，超时仍未排空的连接将被强制关闭
	GCONFIG_RECORD_DRAIN_TIMEOUT int64
	// 应用内升级替换二进制后，界面等待服务重新就绪的上限(秒)。容器与低配机器重启更慢，
	// 故放开为可配；由升级进度接口下发给前端，前端不写死。
	GCONFIG_UPDATE_RESTART_TIMEOUT int64
	// 版本/清单检查请求是否附带运行环境标识(os/arch/rt)。默认开：升级源据此判断存量版本
	// 与运行环境分布。关闭后只保留一直都有的 v(版本)与 u(实例码)，即回到旧版本的携带内容。
	GCONFIG_UPDATE_ENV_REPORT bool
	// GCONFIG_RECORD_PATCH_VERSION_CORE  int64 = 20250106 // 核心数据库补丁日期
	// GCONFIG_RECORD_PATCH_VERSION_LOG   int64 = 20250106 // 日志数据库补丁日期
	// 记录原始信息(默认不开启)
	GCONFIG_RECORD_ALL_SRC_BYTE_INFO int64
	// 配置是否启用http3(默认关闭)
	GCONFIG_ENABLE_HTTP3 int64
	// 配置http3是否用BBR(默认NewReno)
	GCONFIG_ENABLE_HTTP3_BBR int64
	// 请求记录是否进行脱敏处理 1开启脱敏 0关闭脱敏
	GCONFIG_RECORD_LOG_DESENSITIZE int64
	// 令牌有效期(分钟)：空闲多久未访问即失效，每次通过鉴权的请求都会滑动续期。
	// 默认由 5 分钟调整为 30 分钟：5 分钟同时充当空闲与绝对超时，配合前端无保活轮询，
	// 表现为"页面开着不动 5 分钟必掉线"（issue #930）。30 分钟落在 OWASP 低风险区间与 NIST 800-63B AAL2 内。
	// 配置值填 0 或负数表示"不管控有效期"，由 waftask.normalizeTokenExpireMinutes 归一化为 1 年封顶，
	// 因此本变量在运行期恒为正数，取用方无需再做防御。
	// 令牌有效期 单位分钟
	GCONFIG_RECORD_TOKEN_EXPIRE_MINTUTES int64
	// 公告有效期 单位小时
	GCONFIG_RECORD_ANNOUNCEMENT_EXPIRE_HOURS int64
	// 口令复杂度策略（国标 GB/T 32917 §7 自身安全：身份鉴别/口令复杂度）
	// 密码最小长度
	GCONFIG_PWD_MIN_LENGTH int64
	// 是否要求大写字母 1要求 0不要求
	GCONFIG_PWD_REQUIRE_UPPER int64
	// 是否要求小写字母 1要求 0不要求
	GCONFIG_PWD_REQUIRE_LOWER int64
	// 是否要求数字 1要求 0不要求
	GCONFIG_PWD_REQUIRE_DIGIT int64
	// 是否要求特殊字符 1要求 0不要求
	GCONFIG_PWD_REQUIRE_SPECIAL int64
	// 密码有效期天数（0=不限，到期登录时提示强制更换）
	GCONFIG_PWD_EXPIRE_DAYS int64
	// 历史密码防重用个数（0=不启用）
	GCONFIG_PWD_HISTORY_COUNT int64
	// 默认密码/被重置后是否强制改密 1强制 0否
	GCONFIG_PWD_FORCE_CHANGE_DEFAULT int64
	// DNS bot有效期 单位小时 默认1天
	GCONFIG_RECORD_DNS_BOT_EXPIRE_HOURS int64
	// DNS 正常有效期 单位小时 默认7天
	GCONFIG_RECORD_DNS_NORMAL_EXPIRE_HOURS int64
	// 爬虫禁止访问开关 默认 0 只检测不阻止访问 1 检测并阻止访问
	GCONFIG_RECORD_SPIDER_DENY int64
	// 伪爬虫进行图形挑战开关 0 放过 1 显示图形验证码
	GCONFIG_RECORD_FAKE_SPIDER_CAPTCHA int64
	// 是否隐藏Server头信息 1隐藏 0不隐藏
	GCONFIG_RECORD_HIDE_SERVER_HEADER int64
	// 是否强制绑定双因素认证(1强制 0不强制)
	GCONFIG_RECORD_FORCE_BIND_2FA int64
	// 调试开关 默认关闭
	GCONFIG_RECORD_DEBUG_ENABLE int64
	// 调试密码 如果未空则不需要密码
	GCONFIG_RECORD_DEBUG_PWD string
	// CORS 跨域来源白名单(逗号分隔,大小写不敏感)，默认空=不允许任何跨域(同源管理界面不受影响)
	GCONFIG_CORS_ALLOW_ORIGINS string
	// GPT远程地址 DeepSeek ChatGpt 以及使用one-api封装好的接口
	GCONFIG_RECORD_GPT_URL string
	// GPT远程授权密钥
	GCONFIG_RECORD_GPT_TOKEN string
	// GPT 模型名称
	GCONFIG_RECORD_GPT_MODEL string
	// IP失败封禁相关配置
	// 失败状态码配置，支持多个用|分隔，也支持正则表达式
	GCONFIG_IP_FAILURE_STATUS_CODES string
	// 是否启用IP失败封禁 1启用 0禁用
	GCONFIG_IP_FAILURE_BAN_ENABLED int64
	// IP失败封禁锁定时间（分钟）默认10分钟
	GCONFIG_IP_FAILURE_BAN_LOCK_TIME int64
	// 规则封禁相关配置（自定义规则 RF.Ban(分钟) 命中后临时封禁客户端IP）
	// 规则封禁执行动作 0返回拦截页 1直接断开连接
	GCONFIG_RULE_BAN_MODE int64
	// 是否检测beta版本更新 1启用 0禁用（默认检测最新版本）
	GCONFIG_CHECK_BETA_VERSION int64
	// ZeroSSL 相关配置
	// zerossl访问key
	GCONFIG_ZEROSSL_ACCESS_KEY string
	// zerossl eab_kid
	GCONFIG_ZEROSSL_EAB_KID string
	// zerossl eab_hmac_key
	GCONFIG_ZEROSSL_EAB_HMAC_KEY string
	// 开放平台配置
	// 开放平台开关，默认关闭 有需要的开启 1启用 0禁用
	GCONFIG_OPEN_PLATFORM_ENABLED int64
	// 任务日志配置
	// 任务日志保留天数（默认 30 天）
	GCONFIG_TASK_LOG_RETAIN_DAYS int64
	// 日志文件写入配置 (额外输出，不影响SQLite存储)
	// 日志文件写入开关 (0关闭 1开启)
	GCONFIG_LOG_FILE_WRITE_ENABLE int64
	// 日志文件路径
	GCONFIG_LOG_FILE_WRITE_PATH string
	// 日志格式: nginx, apache, custom
	GCONFIG_LOG_FILE_WRITE_FORMAT string
	// 自定义格式模板
	GCONFIG_LOG_FILE_WRITE_CUSTOM_TPL string
	// 单个日志文件最大大小 (MB)
	GCONFIG_LOG_FILE_WRITE_MAX_SIZE int64
	// 保留的历史文件数量
	GCONFIG_LOG_FILE_WRITE_MAX_BACKUPS int64
	// 保留天数
	GCONFIG_LOG_FILE_WRITE_MAX_DAYS int64
	// 是否压缩历史文件 (0关闭 1开启)
	GCONFIG_LOG_FILE_WRITE_COMPRESS int64
	// 主机远程登录爆破防护(SSH/RDP)
	// 保护的是 SamWaf 所在这台机器自身，与 WAF 引擎(保护 Web 站点)是两件事。
	// 默认关闭且默认观察模式 —— 这类功能一旦误封就是"自己 SSH 进不去"，不能开箱即封。
	// 主机防爆破总开关 1启用 0禁用
	GCONFIG_HOST_GUARD_ENABLED int64
	// 工作模式 observe(只记录不封禁) / block(达阈值即封)
	GCONFIG_HOST_GUARD_MODE string
	// 失败统计窗口(分钟)
	GCONFIG_HOST_GUARD_FIND_TIME int64
	// 窗口内失败达该次数触发处置
	GCONFIG_HOST_GUARD_MAX_RETRY int64
	// 累犯记忆期(天)，超过则下次封禁从第1级重新开始
	GCONFIG_HOST_GUARD_OFFENDER_RESET_DAY int64
	// 软失败(preauth断连/用户名枚举/PAM行)是否计入阈值 1计入 0不计入
	GCONFIG_HOST_GUARD_COUNT_SOFT_FAIL int64
	// 永不封禁的IP/网段，逗号分隔，支持单IP/CIDR/通配符/区间
	GCONFIG_HOST_GUARD_WHITELIST string
	// 是否自动豁免本机网卡IP、环回与常见内网段 1豁免 0不豁免
	GCONFIG_HOST_GUARD_AUTO_LAN int64
	// 自定义认证日志路径(逗号分隔)，留空自动探测；容器部署的逃生口
	GCONFIG_HOST_GUARD_LOG_PATHS string
	// SSH实际端口(逗号分隔)，留空自动发现
	GCONFIG_HOST_GUARD_SSH_PORTS string
	// RDP实际端口(逗号分隔)，留空自动发现
	GCONFIG_HOST_GUARD_RDP_PORTS string
	// 封禁范围 all(全端口，更安全) / detected(只封SSH/RDP端口)
	GCONFIG_HOST_GUARD_PORT_SCOPE string
	// 执行方式 auto(平台自适应) / ipset(强制集合) / rule(强制逐条规则)
	GCONFIG_HOST_GUARD_EXEC_MODE string
	// Windows 去抖合并窗口(秒)，Linux/macOS 走增量不受影响。
	// 取 5 而不是 30：去抖的目的是把"一阵爆发"合并成一次重建，5 秒已经足够
	// 收拢成百上千条封禁；再往上加只是让"通知都收到了、防火墙里还没这个 IP"
	// 的观感变差，换不来多少重建次数的下降。
	GCONFIG_HOST_GUARD_DEBOUNCE_SEC int64
	// 封禁集合容量上限，超限先淘汰剩余时间最短的临时封禁(永久封不淘汰)
	GCONFIG_HOST_GUARD_MAX_BAN_ENTRIES int64
	// 每分钟最多新增封禁数，防分布式爆破把集合瞬间打满
	GCONFIG_HOST_GUARD_BAN_RATE_LIMIT int64
	// 网段聚合开关，同/24内被封数达阈值则升级封整段。误伤面大，默认关
	GCONFIG_HOST_GUARD_SUBNET_AGGREGATE int64
	// 网段聚合触发阈值
	GCONFIG_HOST_GUARD_SUBNET_THRESHOLD int64
	// 触发封禁时是否发通知(复用「IP封禁」消息类型)
	GCONFIG_HOST_GUARD_NOTIFY int64
	// GCONFIG_HOST_GUARD_FORCE_DISABLE 是自救开关，只能从 conf/config.yml 的
	// security.host_guard_force_disable 或环境变量 SAMWAF_HOSTGUARD_DISABLE 设置，管理端改不了。
	// 存在的意义：白名单配错把自己的管理IP也封进 iptables 后，SSH 和管理端会同时进不去，
	// 那时只剩物理控制台/云厂商VNC，能操作的只有配置文件。
	GCONFIG_HOST_GUARD_FORCE_DISABLE bool
	// 远程连接看板
	// 连接看板开关 1启用 0禁用
	GCONFIG_HOST_CONN_ENABLED int64
	// 连接快照缓存秒数(Linux采集需遍历/proc，建议不低于3)
	GCONFIG_HOST_CONN_CACHE_SEC int64
	// 反向查询DNS的IP
	GWAF_RUNTIME_DNS_SERVER string
	// DNS 查询超时时间 单位毫秒
	GWAF_RUNTIME_DNS_TIMEOUT int64
	// 记录日志形式： 全部(all),非正常(abnormal)
	GWAF_RUNTIME_RECORD_LOG_TYPE string
	// 全局排除记录日志的IP清单(所有站点生效)，语法同站点级 exclude_ip_log
	GCONFIG_EXCLUDE_IP_LOG string
	// IP 数据库配置
	// IPv4 数据来源: ip2region / geolite2（ip2region.xdb 随程序内置）
	GCONFIG_IP_V4_SOURCE string
	// IPv6 数据来源: ip2region / geolite2
	GCONFIG_IP_V6_SOURCE string
	// 注：IPv6 默认自 1.3.24-beta.6 由 geolite2 改为 ip2region —— GeoLite2 受 MaxMind 再分发授权限制，
	// 已不再随程序内置。ip2region_v6.xdb 需在【IP库管理】在线下载或自行放入 data/。
	// 只影响新装；老用户 sys_config 里已有的值不动，由 ReloadFromConfig 做运行时降级。
	// IPv4 xdb 字段格式，所有发行版本统一默认开源版
	GCONFIG_IP_V4_FORMAT string
	// IPv6 xdb 字段格式（仅 ip2region 时有效）
	GCONFIG_IP_V6_FORMAT string
	// 删除180天前的数据
	GDATA_DELETE_INTERVAL int64
	// *****数据库处理参数****
	// 最大批量插入数量
	GDATA_BATCH_INSERT int64
	// 100w 进行分库 100*10000
	GDATA_SHARE_DB_SIZE int64
	// 1024M 进行分库
	GDATA_SHARE_DB_FILE_SIZE int64
	// GDATA_ACCESS_LOG_MODE 访问日志窄行档位：off=只留安全事件，db=全量入库，sample=事件+采样。
	// off 会失去：CC 阈值推荐、AI 训练负样本、异常 IP 的正常行为回溯（全都要正常请求的全量/样本）。
	GDATA_ACCESS_LOG_MODE string
	// GDATA_ACCESS_LOG_RETENTION_DAYS access_log 保留天数。它直接决定 CC 阈值推荐能回看多少天。
	GDATA_ACCESS_LOG_RETENTION_DAYS int64
	// GDATA_ANALYSIS_SCAN_PATH_THRESHOLD 来源分析里「疑似扫目录」的判定线：一天摸过多少个不同路径模板。
	// 只影响界面上的提示与筛选，不落库、不改任何拦截行为。
	GDATA_ANALYSIS_SCAN_PATH_THRESHOLD int64
	// GDATA_ANALYSIS_UA_THRESHOLD 来源分析里「疑似换 UA 试探」的判定线：一天用过多少种 UA 指纹。
	GDATA_ANALYSIS_UA_THRESHOLD int64
	// GDATA_IP_TAG_DB IP Tag 存放位置：0 核心库，1 统计库。
	// 新装默认统计库——ip_tags 是派生索引，写入量跟着攻击量走，和配置共用核心库会拖慢每一次配置读写。
	// 存量用户的取值来自 system_configs 里已有的那一行，不受这个默认值影响；
	// 切换归属时 MergeIPTagsInto 会把另一个库里的历史标签并过来。
	GDATA_IP_TAG_DB int64
	// GCONFIG_ATTACK_TAG_EXCLUDE 风险日志里不算风险的标签（逗号分隔）。"正常"永远排除，不用写这里。
	// 这些标签既不出现在规则筛选列表里，也不计入「阻止数量」，而是算作放行。
	GCONFIG_ATTACK_TAG_EXCLUDE string
}

type runtimeSettingsSnapshot struct {
	values     RuntimeSettings
	bodyFields map[string]struct{}
}

func prepareRuntimeSettings(values RuntimeSettings) *runtimeSettingsSnapshot {
	s := &runtimeSettingsSnapshot{values: values, bodyFields: map[string]struct{}{}}
	for _, name := range strings.Split(values.GCONFIG_BODY_DETECT_FIELD_EXCLUDE, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			s.bodyFields[name] = struct{}{}
		}
		if len(s.bodyFields) >= 200 {
			break
		}
	}
	return s
}

var runtimeSettingsMu sync.Mutex
var runtimeSettings = func() *atomic.Pointer[runtimeSettingsSnapshot] {
	p := new(atomic.Pointer[runtimeSettingsSnapshot])
	p.Store(prepareRuntimeSettings(RuntimeSettings{
		GCONFIG_LOG_PERSIST_ENABLED:              1,
		GCONFIG_RECORD_MAX_BODY_LENGTH:           1024 * 2,
		GCONFIG_RECORD_MAX_RES_BODY_LENGTH:       1024 * 4,
		GCONFIG_RECORD_RESP:                      0,
		GCONFIG_RECORD_PROXY_HEADER:              "",
		GCONFIG_MANAGE_PROXY_HEADER:              "",
		GCONFIG_MANAGE_TRUSTED_PROXIES:           "",
		GCONFIG_MANAGE_CDN_PROVIDER:              "",
		GCONFIG_SSL_EXPORT_ALLOWED_DIRS:          "",
		GCONFIG_DATA_KEY_FILE:                    "",
		GCONFIG_COMM_KEY_FILE:                    "",
		GCONFIG_COMM_LEGACY_KEY:                  true,
		GCONFIG_BATCH_IMPORT_ALLOWED_DIRS:        "",
		GCONFIG_OUTBOUND_ALLOWED_HOSTS:           "",
		GCONFIG_RECORD_AUTO_LOAD_SSL:             1,
		GCONFIG_RECORD_KAFKA_ENABLE:              0,
		GCONFIG_RECORD_KAFKA_URL:                 "127.0.0.1:9092",
		GCONFIG_RECORD_KAFKA_TOPIC:               "samwaf_logs_topic",
		GCONFIG_RECORD_REDIRECT_HTTPS_CODE:       301,
		GCONFIG_ENABLE_HTTPS_REDIRECT:            0,
		GCONFIG_RECORD_PROXY_LOOP_MAX_HOP:        10,
		GCONFIG_RECORD_LOGIN_MAX_ERROR_TIME:      3,
		GCONFIG_RECORD_LOGIN_LIMIT_MINTUTES:      1,
		GCONFIG_IPPROBE_ENABLE:                   0,
		GCONFIG_ACCESS_ENABLE:                    0,
		GCONFIG_ACCESS_AUDIT_RETAIN_DAYS:         90,
		GCONFIG_ACCESS_FORCE_DISABLE:             false,
		GCONFIG_ENABLE_SYSTEM_STATS_PUSH:         1,
		GCONFIG_ENABLE_PROXY_PROTOCOL:            0,
		GCONFIG_ENABLE_DEVICE_FINGERPRINT:        0,
		GCONFIG_ENABLE_STRICT_IP_BINDING:         0,
		GCONFIG_ENABLE_REPLAY_PROTECT:            1,
		GCONFIG_ALLOW_CONTAINER_SELFUPDATE:       0,
		GCONFIG_RECORD_ENABLE_OWASP:              0,
		GCONFIG_OWASP_MODE:                       "DetectionOnly",
		GCONFIG_AI_ENABLE:                        0,
		GCONFIG_AI_MODE:                          "observe",
		GCONFIG_BODY_DETECT_MODE:                 "observe",
		GCONFIG_BODY_DETECT_FIELD_EXCLUDE:        "",
		GCONFIG_RULE_CHAIN_MODE:                  0,
		GCONFIG_OWASP_BLOCK_THRESHOLD:            7,
		GCONFIG_RECORD_ENABLE_HTTP_80:            0,
		GCONFIG_RECORD_SSLOrder_EXPIRE_DAY:       30,
		GCONFIG_RECORD_SSL_IP_CERT_IP:            "",
		GCONFIG_RECORD_SSL_IP_EXPIRE_DAY:         3,
		GCONFIG_RECORD_SSLHTTP_CHECK:             0,
		GCONFIG_SSL_EXPIRE_AUTO_SYNC_HOST:        1,
		GCONFIG_RECORD_SSLMinVerson:              "TLS 1.2",
		GCONFIG_RECORD_SSLMaxVerson:              "TLS 1.3",
		GCONFIG_RECORD_CONNECT_TIME_OUT:          30,
		GCONFIG_RECORD_KEEPALIVE_TIME_OUT:        30,
		GCONFIG_RECORD_DRAIN_TIMEOUT:             30,
		GCONFIG_UPDATE_RESTART_TIMEOUT:           90,
		GCONFIG_UPDATE_ENV_REPORT:                false,
		GCONFIG_RECORD_ALL_SRC_BYTE_INFO:         0,
		GCONFIG_ENABLE_HTTP3:                     0,
		GCONFIG_ENABLE_HTTP3_BBR:                 0,
		GCONFIG_RECORD_LOG_DESENSITIZE:           1,
		GCONFIG_RECORD_TOKEN_EXPIRE_MINTUTES:     30,
		GCONFIG_RECORD_ANNOUNCEMENT_EXPIRE_HOURS: 24,
		GCONFIG_PWD_MIN_LENGTH:                   8,
		GCONFIG_PWD_REQUIRE_UPPER:                0,
		GCONFIG_PWD_REQUIRE_LOWER:                0,
		GCONFIG_PWD_REQUIRE_DIGIT:                1,
		GCONFIG_PWD_REQUIRE_SPECIAL:              0,
		GCONFIG_PWD_EXPIRE_DAYS:                  0,
		GCONFIG_PWD_HISTORY_COUNT:                0,
		GCONFIG_PWD_FORCE_CHANGE_DEFAULT:         1,
		GCONFIG_RECORD_DNS_BOT_EXPIRE_HOURS:      24,
		GCONFIG_RECORD_DNS_NORMAL_EXPIRE_HOURS:   7 * 24,
		GCONFIG_RECORD_SPIDER_DENY:               0,
		GCONFIG_RECORD_FAKE_SPIDER_CAPTCHA:       0,
		GCONFIG_RECORD_HIDE_SERVER_HEADER:        1,
		GCONFIG_RECORD_FORCE_BIND_2FA:            0,
		GCONFIG_RECORD_DEBUG_ENABLE:              0,
		GCONFIG_RECORD_DEBUG_PWD:                 "",
		GCONFIG_CORS_ALLOW_ORIGINS:               "",
		GCONFIG_RECORD_GPT_URL:                   "https://api.deepseek.com",
		GCONFIG_RECORD_GPT_TOKEN:                 "SamWaf提示请输入密钥",
		GCONFIG_RECORD_GPT_MODEL:                 "deepseek-chat",
		GCONFIG_IP_FAILURE_STATUS_CODES:          "401|403|404|444|429|503",
		GCONFIG_IP_FAILURE_BAN_ENABLED:           0,
		GCONFIG_IP_FAILURE_BAN_LOCK_TIME:         10,
		GCONFIG_RULE_BAN_MODE:                    0,
		GCONFIG_CHECK_BETA_VERSION:               1,
		GCONFIG_ZEROSSL_ACCESS_KEY:               "",
		GCONFIG_ZEROSSL_EAB_KID:                  "",
		GCONFIG_ZEROSSL_EAB_HMAC_KEY:             "",
		GCONFIG_OPEN_PLATFORM_ENABLED:            0,
		GCONFIG_TASK_LOG_RETAIN_DAYS:             30,
		GCONFIG_LOG_FILE_WRITE_ENABLE:            0,
		GCONFIG_LOG_FILE_WRITE_PATH:              "logs/access.log",
		GCONFIG_LOG_FILE_WRITE_FORMAT:            "nginx",
		GCONFIG_LOG_FILE_WRITE_CUSTOM_TPL:        "",
		GCONFIG_LOG_FILE_WRITE_MAX_SIZE:          100,
		GCONFIG_LOG_FILE_WRITE_MAX_BACKUPS:       10,
		GCONFIG_LOG_FILE_WRITE_MAX_DAYS:          30,
		GCONFIG_LOG_FILE_WRITE_COMPRESS:          0,
		GCONFIG_HOST_GUARD_ENABLED:               0,
		GCONFIG_HOST_GUARD_MODE:                  "observe",
		GCONFIG_HOST_GUARD_FIND_TIME:             10,
		GCONFIG_HOST_GUARD_MAX_RETRY:             8,
		GCONFIG_HOST_GUARD_OFFENDER_RESET_DAY:    7,
		GCONFIG_HOST_GUARD_COUNT_SOFT_FAIL:       0,
		GCONFIG_HOST_GUARD_WHITELIST:             "",
		GCONFIG_HOST_GUARD_AUTO_LAN:              1,
		GCONFIG_HOST_GUARD_LOG_PATHS:             "",
		GCONFIG_HOST_GUARD_SSH_PORTS:             "",
		GCONFIG_HOST_GUARD_RDP_PORTS:             "",
		GCONFIG_HOST_GUARD_PORT_SCOPE:            "all",
		GCONFIG_HOST_GUARD_EXEC_MODE:             "auto",
		GCONFIG_HOST_GUARD_DEBOUNCE_SEC:          5,
		GCONFIG_HOST_GUARD_MAX_BAN_ENTRIES:       10000,
		GCONFIG_HOST_GUARD_BAN_RATE_LIMIT:        200,
		GCONFIG_HOST_GUARD_SUBNET_AGGREGATE:      0,
		GCONFIG_HOST_GUARD_SUBNET_THRESHOLD:      10,
		GCONFIG_HOST_GUARD_NOTIFY:                1,
		GCONFIG_HOST_GUARD_FORCE_DISABLE:         false,
		GCONFIG_HOST_CONN_ENABLED:                1,
		GCONFIG_HOST_CONN_CACHE_SEC:              3,
		GWAF_RUNTIME_DNS_SERVER:                  "119.29.29.29",
		GWAF_RUNTIME_DNS_TIMEOUT:                 500,
		GWAF_RUNTIME_RECORD_LOG_TYPE:             "all",
		GCONFIG_EXCLUDE_IP_LOG:                   "",
		GCONFIG_IP_V4_SOURCE:                     "ip2region",
		GCONFIG_IP_V6_SOURCE:                     "ip2region",
		GCONFIG_IP_V4_FORMAT:                     "opensource",
		GCONFIG_IP_V6_FORMAT:                     "opensource",
		GDATA_DELETE_INTERVAL:                    180,
		GDATA_BATCH_INSERT:                       100,
		GDATA_SHARE_DB_SIZE:                      100 * 10000,
		GDATA_SHARE_DB_FILE_SIZE:                 1024,
		GDATA_ACCESS_LOG_MODE:                    "db",
		GDATA_ACCESS_LOG_RETENTION_DAYS:          30,
		GDATA_ANALYSIS_SCAN_PATH_THRESHOLD:       20,
		GDATA_ANALYSIS_UA_THRESHOLD:              5,
		GDATA_IP_TAG_DB:                          1,
		GCONFIG_ATTACK_TAG_EXCLUDE:               "ACME证书校验,静态文件访问成功",
	}))
	return p
}()

// CurrentSettings returns one complete published revision.
func CurrentSettings() RuntimeSettings { return runtimeSettings.Load().values }

// UpdateSettings serializes short value-only edits. Never perform IO or call
// UpdateSettings recursively in the callback. Retaining the draft cannot mutate published data.
func UpdateSettings(change func(*RuntimeSettings)) {
	runtimeSettingsMu.Lock()
	defer runtimeSettingsMu.Unlock()
	draft := runtimeSettings.Load().values
	change(&draft)
	runtimeSettings.Store(prepareRuntimeSettings(draft))
}

// CommitSettings merges only fields changed by a prepared configuration reload,
// preserving unrelated updates published while the database was being read.
func CommitSettings(before, after RuntimeSettings) {
	UpdateSettings(func(current *RuntimeSettings) {
		old := reflect.ValueOf(before)
		next := reflect.ValueOf(after)
		dst := reflect.ValueOf(current).Elem()
		for i := 0; i < old.NumField(); i++ {
			if !old.Field(i).Equal(next.Field(i)) {
				dst.Field(i).Set(next.Field(i))
			}
		}
	})
}

type runtimeSettingsContextKey struct{}

// WithSettings pins a configuration revision for the complete request/response.
func WithSettings(ctx context.Context) context.Context {
	if _, ok := ctx.Value(runtimeSettingsContextKey{}).(*runtimeSettingsSnapshot); ok {
		return ctx
	}
	return context.WithValue(ctx, runtimeSettingsContextKey{}, runtimeSettings.Load())
}
func SettingsForContext(ctx context.Context) RuntimeSettings {
	if ctx != nil {
		if s, ok := ctx.Value(runtimeSettingsContextKey{}).(*runtimeSettingsSnapshot); ok {
			return s.values
		}
	}
	return CurrentSettings()
}

func BodyFieldExcluded(ctx context.Context, field string) bool {
	s := runtimeSettings.Load()
	if ctx != nil {
		if pinned, ok := ctx.Value(runtimeSettingsContextKey{}).(*runtimeSettingsSnapshot); ok {
			s = pinned
		}
	}
	_, excluded := s.bodyFields[strings.ToLower(field)]
	return excluded
}
