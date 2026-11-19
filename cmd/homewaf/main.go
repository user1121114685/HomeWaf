// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package main

import (
	"HomeWaf/cache"
	"HomeWaf/common/gwebsocket"
	"HomeWaf/common/tasklog"
	"HomeWaf/common/wafexec"
	"HomeWaf/common/zlog"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/globalobj"
	"HomeWaf/innerbean"
	"HomeWaf/internal/licensing"
	"HomeWaf/iplocation"
	"HomeWaf/model"
	"HomeWaf/plugin"
	"HomeWaf/public"
	"HomeWaf/service/waf_service"
	"HomeWaf/supervisor"
	"HomeWaf/utils"
	"HomeWaf/wafappengine"
	"HomeWaf/wafcli"
	"HomeWaf/wafconfig"
	"HomeWaf/wafdb"
	"HomeWaf/wafdiag"
	"HomeWaf/wafenginecore"
	"HomeWaf/wafenginecore/wafcaptcha"
	"HomeWaf/wafhostguard"
	"HomeWaf/wafinit"
	"HomeWaf/wafipban"
	"HomeWaf/wafipc"
	"HomeWaf/wafmangeweb"
	"HomeWaf/wafnet"
	"HomeWaf/wafnotify"
	"HomeWaf/wafowasp"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafqueue"
	"HomeWaf/wafreg"
	"HomeWaf/wafsafeclear"
	"HomeWaf/wafsec"
	"HomeWaf/waftask"
	"HomeWaf/waftunnelengine"
	"HomeWaf/wafupdate"
	"context"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	dlp "HomeWaf/internal/thirdparty/godlp"

	"github.com/kardianos/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

//go:embed exedata/ip2region.xdb
var Ip2regionBytes []byte // 当前目录，解析为[]byte类型

//go:embed exedata/ip2region_v6.xdb
var Ip2regionV6Bytes []byte

// GeoLite2-Country.mmdb 自 1.3.24-beta.6 起不再内嵌，
// 把它编进发行的二进制等同于再分发。需要 GeoLite2 的用户可自行到 MaxMind 官网下载 mmdb
// 放进 data/ 目录，程序会照常加载。IPv6 默认改用 ip2region（Apache-2.0，可自由分发）。

//go:embed exedata/ldpconfig.yml
var ldpConfig string //隐私防护ldp

//go:embed exedata/owasp
var owaspAssets embed.FS

//go:embed exedata/spiderbot
var spiderBotAssets embed.FS

//go:embed exedata/captcha
var captchaAssets embed.FS

//go:embed exedata/capjs
var capjs embed.FS

//go:embed exedata/httpauth
var httpauth embed.FS

//go:embed exedata/access
var accessAssets embed.FS

// wafSystenService 实现了 service.Service 接口
type wafSystenService struct {
	startup         wafnet.Readiness
	sslWorkerMu     sync.Mutex
	sslWorkerCancel context.CancelFunc
}

var webmanager *wafmangeweb.WafWebManager // web管理端

// Start 是服务启动时调用的方法
func (m *wafSystenService) Start(s service.Service) error {
	zlog.Info("服务启动形式-----Start")
	go m.run()
	return nil
}

// Stop 是服务停止时调用的方法
func (m *wafSystenService) Stop(s service.Service) error {
	zlog.Info("服务形式的 -----stop")
	m.Graceful()
	return nil
}

// 守护协程
func NeverExit(name string, f func()) {
	defer func() {
		if v := recover(); v != nil {
			zlog.Error(fmt.Sprintf("协程%s崩溃了，准备重启一个。 : %v, Stack Trace: %s", name, v, debug.Stack()))
			if global.GWAF_RELEASE == "false" {
				debug.PrintStack()
			}

			go NeverExit(name, f) // 重启一个同功能协程
		}
	}()
	zlog.Info(name + " start")
	f()
}

// run 是服务的主要逻辑
func (m *wafSystenService) run() {
	defer m.startup.Fail(errors.New("Worker 初始化或事件循环已退出"))

	// 先尝试监听端口，检查是否被占用。
	// 升级接管(takeover)模式下旧 Worker 仍持有管理端口 :26666，新 Worker 预期与其并存，故跳过该探测。
	if !global.GWAF_RUNTIME_IS_TAKEOVER {
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(global.GWAF_LOCAL_SERVER_PORT))
		if err != nil {
			errMsg := fmt.Sprintf("管理界面端口 %d 已被占用，请检查并修改配置(conf/config.yml local_port字段)或关闭占用该端口的程序: %s",
				global.GWAF_LOCAL_SERVER_PORT, err.Error())
			zlog.Error(errMsg)
			panic(errMsg)
		}
		if listener != nil {
			listener.Close()
		}
	}
	// 获取当前执行文件的路径
	executablePath, err := os.Executable()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	zlog.Info("执行位置:", executablePath)
	global.GWAF_RUNTIME_CURRENT_EXEPATH = executablePath
	//初始化步骤[加载ip数据库]
	// 创建 IP Location Manager
	global.GIPLOCATION_MANAGER = iplocation.NewManager()
	// 注册内置数据库，供 manager 在磁盘无文件时兜底加载。
	// IPv4/IPv6 均有离线 ip2region 兜底；用户自备文件仍优先。
	iplocation.SetBuiltinData(Ip2regionBytes, nil, Ip2regionV6Bytes)

	// 加载优先级与热重载共用同一份实现（磁盘文件 > 内置数据 > 同类型其它来源降级）。
	// IP 库属于「有更好、没有也得能跑」的辅助数据：任何加载失败都只告警，
	// 绝不能阻止 WAF 启动——防护能力不依赖地区库。
	if err := global.GIPLOCATION_MANAGER.ReloadFromConfig(
		filepath.Join(utils.GetCurrentDir(), "data"),
		global.CurrentSettings().GCONFIG_IP_V4_SOURCE, global.CurrentSettings().GCONFIG_IP_V6_SOURCE,
		global.CurrentSettings().GCONFIG_IP_V4_FORMAT, global.CurrentSettings().GCONFIG_IP_V6_FORMAT,
	); err != nil {
		zlog.Warn("IP地理位置数据库加载失败，地区相关功能将不可用: ", err.Error())
	}
	if st := global.GIPLOCATION_MANAGER.GetStatus(); st != nil {
		zlog.Info(fmt.Sprintf("IP库加载完成 IPv4:%s(内置:%v,%d字节) IPv6:%s(内置:%v,%d字节)",
			st.IPv4Source, st.IPv4Builtin, st.IPv4FileSize,
			st.IPv6Source, st.IPv6Builtin, st.IPv6FileSize))
		if st.IPv6FileSize == 0 {
			zlog.Warn("IPv6 地区库不可用：IPv6 访客的归属地将显示为未知，且地区类自定义规则对 IPv6 请求不生效。" +
				"可在【IP库管理】里在线下载，或自行下载 ip2region_v6.xdb 放入 data/ 目录")
		}
	}

	global.GWAF_DLP_CONFIG = ldpConfig

	//owasp资源 释放
	err = wafinit.CheckAndReleaseDataset(owaspAssets, utils.GetCurrentDir()+"/data/owasp", "owasp")
	if err != nil {
		zlog.Error("owasp", err.Error())
	}

	// 验证码资源释放
	err = wafinit.CheckAndReleaseDataset(captchaAssets, utils.GetCurrentDir()+"/data/captcha", "captcha")
	if err != nil {
		zlog.Error("captcha", err.Error())
	}
	// 验证码工作量证明资源释放
	err = wafinit.CheckAndReleaseDataset(capjs, utils.GetCurrentDir()+"/data/capjs", "capjs")
	if err != nil {
		zlog.Error("capJs", err.Error())
	}

	// HTTP认证登录页面资源释放
	err = wafinit.CheckAndReleaseDataset(httpauth, utils.GetCurrentDir()+"/data/httpauth", "httpauth")
	if err != nil {
		zlog.Error("httpauth", err.Error())
	}

	// 统一访问认证(Access 模式)登录页资源释放
	err = wafinit.CheckAndReleaseDataset(accessAssets, utils.GetCurrentDir()+"/data/access", "access")
	if err != nil {
		zlog.Error("access", err.Error())
	}
	//TODO 准备释放最新spider bot

	//初始化cache
	{
		cacheStore, err := cache.NewCacheStore(global.GCACHE_TYPE, &cache.RedisCacheConfig{
			Host:        global.GCACHE_REDIS_HOST,
			Port:        global.GCACHE_REDIS_PORT,
			Password:    global.GCACHE_REDIS_PASSWORD,
			DB:          global.GCACHE_REDIS_DB,
			PoolSize:    global.GCACHE_REDIS_POOL_SIZE,
			PoolTimeout: time.Duration(global.GCACHE_REDIS_POOL_TIMEOUT_SEC) * time.Second,
			OpTimeout:   time.Duration(global.GCACHE_REDIS_OP_TIMEOUT_SEC) * time.Second,
		})
		if err != nil {
			zlog.Error("初始化缓存失败，程序退出，请检查conf/config.yml缓存配置是否正确", "error", err)
			os.Exit(1)
		}
		global.GCACHE_WAFCACHE = cacheStore
		// 缓存后端决定了令牌等凭证放在哪里，排障时是第一个要确认的事实，启动就写进日志。
		if d, ok := cacheStore.(cache.BackendDescriber); ok {
			zlog.Info("缓存后端: " + d.Describe())
		} else {
			zlog.Info("缓存后端: memory(进程内)")
		}
	}
	//初始化验证码服务
	wafcaptcha.InitCaptchaService(global.GCACHE_WAFCACHE)
	//初始化锁写不锁度
	global.GWAF_MEASURE_PROCESS_DEQUEENGINE = cache.InitWafOnlyLockWrite()

	// 注入 IP 库在线下载上下文：iplocation 不能反向依赖 global/utils（会成环），
	// 所以升级源、SSRF 安全客户端、通知回调都从这里传进去。
	iplocation.ConfigureUpgrader(iplocation.UpgradeConfig{
		Enabled:          global.GUPDATE_VERSION_URL != "",
		RequireSignature: true,
		TrustedKeysJSON:  licensing.TrustedKeysJSON,
		UpdateVersionURL: global.GUPDATE_VERSION_URL,
		NewClient:        utils.SafeHTTPClient,
		ValidateURL:      utils.IsSafeOutboundURL,
		NotifyFunc: func(success bool, msg string) {
			if global.GQEQUE_MESSAGE_DB == nil {
				return
			}
			successStr := "false"
			if success {
				successStr = "true"
			}
			global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.UpdateResultMessageInfo{
				BaseMessageInfo: innerbean.BaseMessageInfo{
					OperaType: "IP库下载",
					Server:    global.GWAF_CUSTOM_SERVER_NAME,
				},
				Msg:     msg,
				Success: successStr,
			})
		},
	})

	// 创建owasp 管理器（支持热重载）
	global.GWAF_OWASP_MANAGER = wafowasp.NewOwaspManager(utils.GetCurrentDir())
	global.GWAF_OWASP = global.GWAF_OWASP_MANAGER.Current()
	// 注入升级上下文：避免 wafowasp → global 的循环依赖
	wafowasp.ConfigureUpgrader(wafowasp.UpgradeConfig{
		UpdateVersionURL: global.GUPDATE_VERSION_URL,
		ClientQuery:      wafupdate.ClientQuery,
		NotifyFunc: func(success bool, msg string) {
			if global.GQEQUE_MESSAGE_DB == nil {
				return
			}
			successStr := "false"
			if success {
				successStr = "true"
			}
			global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.UpdateResultMessageInfo{
				BaseMessageInfo: innerbean.BaseMessageInfo{
					OperaType: "OWASP 规则升级",
					Server:    global.GWAF_CUSTOM_SERVER_NAME,
				},
				Msg:     msg,
				Success: successStr,
			})
		},
	})

	// 初始化ip ban
	wafipban.InitIPBanManager(global.GCACHE_WAFCACHE)
	// 主机登录失败计数器：与上面共用同一个缓存，但 key 前缀独立。
	// 不共用 keyspace 是因为上面那套计数会被自定义规则的 MF.GetIPFailureCount 读取，
	// 混入 SSH/RDP 的失败会静默改变用户已有 WAF 规则的语义。
	wafipban.InitHostLoginFailureManager(global.GCACHE_WAFCACHE)
	// 把事件落库能力注入主机防护引擎（反向注入避免 service 与 wafhostguard 循环依赖）
	waf_service.WafHostGuardServiceApp.InitEventSink()

	//提前初始化
	global.GDATA_CURRENT_LOG_DB_MAP = map[string]*gorm.DB{}
	rversion := "初始化系统 编译器版本:" + runtime.Version() + " 程序版本号：" + global.GWAF_RELEASE_VERSION_NAME + "(" + global.GWAF_RELEASE_VERSION + ") 前端版本：" + global.GWAF_WEB_VERSION
	if global.GWAF_RELEASE == "false" {
		rversion = rversion + " 调试版本"
	} else {
		rversion = rversion + " 发行版本"
	}
	if runtime.GOOS == "linux" {
		rversion = rversion + " linux"
	} else if runtime.GOOS == "windows" {
		rversion = rversion + " windows"
	} else {
		rversion = rversion + "  " + runtime.GOOS
	}
	rversion = rversion + " plat:" + runtime.GOOS + "-" + runtime.GOARCH
	zlog.Info(rversion)
	zlog.Info("OutIp", global.GWAF_RUNTIME_IP)

	if global.GWAF_RELEASE == "false" {
		// Publisher endpoints are configured explicitly with HOMEWAF_UPDATE_URL.
	}

	//初始化每实例静态数据加密密钥(DEK)：首启生成 .homewaf-keys/data_key(0600)，或用
	//config.yml 的 security.data_key_file 指定自管路径。须在数据库初始化之前就绪，
	//否则落库敏感字段(Access 密钥/CDN 凭证/2FA 密钥)无法按新格式加解密。
	if err := wafconfig.EnsureInitialManagementTLS(utils.GetCurrentDir()); err != nil {
		zlog.Error("HomeWaf management TLS initialization failed", "error", err)
		os.Exit(1)
	}
	if err := wafsec.InitDataKey(utils.GetCurrentDir(), global.CurrentSettings().GCONFIG_DATA_KEY_FILE); err != nil {
		zlog.Error("初始化数据加密密钥失败，程序退出，请检查 .homewaf-keys 目录权限或 security.data_key_file 配置", "error", err)
		os.Exit(1)
	}

	// Initialize the per-instance X25519 key used to negotiate management sessions.
	if err := wafsec.InitCommKey(utils.GetCurrentDir(), global.CurrentSettings().GCONFIG_COMM_KEY_FILE); err != nil {
		zlog.Error("初始化管理台传输密钥失败", "error", err)
		m.startup.Fail(err)
		return
	}

	//初始化本地数据库
	if _, err := wafdb.InitCoreDb(""); err != nil {
		zlog.Error("初始化核心数据库失败，程序退出，请检查conf/config.yml数据库配置是否正确", "error", err)
		os.Exit(1)
	}
	// Never automatically bless plaintext or legacy ciphertext injected into
	// the database. Any offline conversion must be explicitly operator-driven.
	if _, err := wafdb.InitLogDb(""); err != nil {
		zlog.Error("初始化日志数据库失败，程序退出，请检查conf/config.yml数据库配置是否正确", "error", err)
		os.Exit(1)
	}
	if _, err := wafdb.InitStatsDb(""); err != nil {
		zlog.Error("初始化统计数据库失败，程序退出，请检查conf/config.yml数据库配置是否正确", "error", err)
		os.Exit(1)
	}

	// 全新安装引导：账户表为空(新用户判定)时创建默认管理员并生成随机初始口令。
	// 放在启动初始化(核心库就绪后)执行，不再等首次登录才触发，便于新装即时拿到 data/initial_password.txt。
	isFreshInstall := false
	if cnt, _ := waf_service.WafAccountServiceApp.GetAccountCountApi(); cnt == 0 {
		if err := waf_service.WafAccountServiceApp.InitDefaultAccount(); err != nil {
			zlog.Error("初始化默认管理员账户失败", "error", err)
		} else {
			isFreshInstall = true
		}
	}

	//初始化插件系统（从配置文件加载）
	if err := plugin.InitPluginSystem(); err != nil {
		zlog.Error("初始化插件系统失败", "error", err)
		// 插件系统初始化失败不影响主程序启动
	}

	// WebSocket 在线表必须先于下面的消费协程和管理端建好：消息队列一开始消费就可能广播，
	// 管理端一起来就可能有连接注册进来，而这里只是建两个空 map，没有任何外部依赖，放早没有副作用。
	global.GWebSocket = gwebsocket.InitWafWebSocket()

	//初始化队列引擎
	wafqueue.InitDequeEngine()
	//启动队列消费
	go NeverExit("ProcessCoreDequeEngine", wafqueue.ProcessCoreDequeEngine)
	go NeverExit("ProcessMessageDequeEngine", wafqueue.ProcessMessageDequeEngine)
	go NeverExit("ProcessStatDequeEngine", wafqueue.ProcessStatDequeEngine)
	go NeverExit("ProcessLogDequeEngine", wafqueue.ProcessLogDequeEngine)

	//运行诊断：注册队列/缓存/WebSocket 计量器并启动 10s 趋势采样（内存环形缓冲，不落盘）
	wafdiag.RegisterBuiltins()
	wafdiag.StartSampler()

	//初始化一次系统参数信息
	waftask.TaskLoadSetting(true)

	// 校验"上次运行版本"，识别容器重建导致的降级运行(旧程序+新库)，仅记录日志不阻断
	waftask.CheckVersionDowngrade()

	// IP 数据库：配置已从 DB 读出，按持久化的数据源权威重载一次
	// （启动早期 manager 用的是默认源，此处才是真正生效的加载点，解决重启后 ipdb 回退到 ip2region 的问题）
	if global.GIPLOCATION_MANAGER != nil {
		ipDataDir := filepath.Join(utils.GetCurrentDir(), "data")
		if err := global.GIPLOCATION_MANAGER.ReloadFromConfig(
			ipDataDir,
			global.CurrentSettings().GCONFIG_IP_V4_SOURCE, global.CurrentSettings().GCONFIG_IP_V6_SOURCE,
			global.CurrentSettings().GCONFIG_IP_V4_FORMAT, global.CurrentSettings().GCONFIG_IP_V6_FORMAT,
		); err != nil {
			zlog.Warn("IP 数据库按配置重载失败: ", err)
		}
	}

	// 初始化任务日志管理器（需在 TaskLoadSetting 后执行，以获取正确的 retainDays 配置）
	taskLogDir := filepath.Join(utils.GetCurrentDir(), "logs", "task")
	tasklog.InitGlobalTaskLogManager(taskLogDir, int(global.CurrentSettings().GCONFIG_TASK_LOG_RETAIN_DAYS))
	// 将 TaskZapCore 挂到全局 zlog，使任务执行期间的 zlog 输出同步写入任务日志文件
	zlog.AddCore(tasklog.NewTaskZapCore(zapcore.DebugLevel))
	//启动通知相关程序
	global.GNOTIFY_KAKFA_SERVICE = wafnotify.InitNotifyKafkaEngine(global.CurrentSettings().GCONFIG_RECORD_KAFKA_ENABLE, global.CurrentSettings().GCONFIG_RECORD_KAFKA_URL, global.CurrentSettings().GCONFIG_RECORD_KAFKA_TOPIC) //kafka
	// 日志文件写入
	compressFlag := global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_COMPRESS == 1
	global.GNOTIFY_LOG_FILE_WRITER = wafnotify.InitLogFileWriterEngine(
		global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_ENABLE,
		global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_PATH,
		global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_FORMAT,
		global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_CUSTOM_TPL,
		global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_MAX_SIZE,
		int(global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_MAX_BACKUPS),
		int(global.CurrentSettings().GCONFIG_LOG_FILE_WRITE_MAX_DAYS),
		compressFlag,
	)
	// Publish the shared IP sets before any protected listener starts accepting requests.
	waf_service.WafIPGroupServiceApp.RebuildAllGroupMatchers()

	//启动隧道
	globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE = waftunnelengine.NewWafTunnelEngine()
	//启动应用管理引擎
	globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE = wafappengine.NewWafAppEngine()
	// 隧道/应用是独占型单例(独占端口/外部进程)，不能与旧 Worker 并存。
	// takeover 升级模式下延迟到旧 Worker 退出后由 Supervisor 的 ACTIVATE 触发接管（见 activateSingletons）。
	if !global.GWAF_RUNTIME_IS_TAKEOVER {
		globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.StartTunnel()
		globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.StartApps()
	} else {
		zlog.Info("[Worker] takeover 模式：暂不启动隧道/应用，等待旧 Worker 退出后由 Supervisor 的 ACTIVATE 接管")
	}
	wafupdate.SoftwareReleases = wafupdate.NewReleaseMonitor(filepath.Join(utils.GetCurrentDir(), "data"))
	//启动管理界面
	webmanager = &wafmangeweb.WafWebManager{LogName: "WebManager", StartupReady: &m.startup}
	go func() {
		webmanager.StartLocalServer()
	}()

	//定时取规则并更新（考虑后期定时拉取公共规则 待定，可能会影响实际生产）

	// 创建任务调度器

	global.GWAF_LAST_UPDATE_TIME = time.Now()

	//开始 需要添加到任务注册器里面的
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry = waftask.NewTaskRegistry()
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_RUNTIME_QPS_CLEAN, waftask.TaskLogQpsClean)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_SHARE_DB, waftask.TaskShareDbInfo)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_SHARE_DB_CHECK, waftask.TaskShareDbInfo)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_COUNTER, waftask.TaskCounter)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_DELAY_INFO, waftask.TaskDelayInfo)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_LOAD_CONFIG, waftask.TaskLoadSettingCron)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_REFLUSH_WECHAT_ACCESS_TOKEN, waftask.TaskWechatAccessToken)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_DELETE_HISTORY_INFO, waftask.TaskDeleteHistoryInfo)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_DELETE_HISTORY_DOWNLOAD_FILE, waftask.TaskHistoryDownload)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_SSL_ORDER_RENEW, waftask.SSLOrderReload)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_SSL_PATH_LOAD, waftask.SSLReload)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_BATCH, waftask.BatchTask)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_SSL_EXPIRE_CHECK, waftask.SSLExpireCheck)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_MANAGER_CERT_CHECK, waftask.ManagerCertCheck)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_NOTICE, waftask.TaskStatusNotify)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_GC, waftask.TaskGC)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_STATS_PUSH, waftask.TaskStatsPush)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_DB_MONITOR, waftask.TaskDatabaseMonitor)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_FIREWALL_CLEAN_EXPIRED, waftask.TaskFirewallCleanExpired)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_STATS_DATA_CLEANUP, waftask.TaskStatsDataCleanup)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_THREAT_IP_SYNC, waftask.TaskThreatIPSync)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_ACCESS_CLEAN, waftask.TaskAccessClean)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_HTTPAUTH_CLEAN, waftask.TaskHttpAuthClean)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_HOSTGUARD_CLEAN_EXPIRED, waftask.TaskHostGuardCleanExpired)
	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry.RegisterTask(enums.TASK_TRAFFIC_FLUSH, waftask.TaskTrafficFlush)

	// 进程启动重放：把各启用威胁情报渠道的快照重新灌入系统 ipset(内存态重启会丢) 并重建 WAF 并集
	go waf_service.WafThreatIPServiceApp.RestoreAllOnStartup()
	// 启动重放 CDN 回源段中心库(从库内快照恢复内存匹配集，不对外拉取)；
	// 到期自动拉取由定时任务按各厂商节奏 + AutoFetch 开关处理
	go waf_service.WafCDNIPServiceApp.RestoreAllOnStartup()

	go waftask.TaskShareDbInfo()

	globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler = waftask.NewTaskScheduler(globalobj.GWAF_RUNTIME_OBJ_WAF_TaskRegistry)
	if err := globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.ScheduleSoftwareUpdate(wafqueue.NotifySoftwareUpdate); err != nil {
		zlog.Error("注册 HomeWaf 自动更新检查失败", err.Error())
	}
	taskDbList := waftask.InitTaskDb()
	for _, task := range taskDbList {
		globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.ScheduleTask(task.TaskUnit, task.TaskValue, task.TaskAt, task.TaskMethod, task.TaskDaysOfTheWeek)
	}
	//结束 需要添加到任务注册器里面的
	// cron 任务调度同样是单例（避免新旧 Worker 重复执行定时任务）；takeover 模式延迟到 ACTIVATE 再启动。
	if !global.GWAF_RUNTIME_IS_TAKEOVER {
		globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.Start()
		// 日志导出执行器与任务调度同一接管边界启动，蓝绿升级时只有一个 Worker 受理任务。
		logExportWorkerLifecycle.start(waf_service.WafLogExportServiceApp.Start)
		// 主机登录防护的日志采集同样是独占单例：两个进程同时 tail 同一份 auth.log，
		// 同一条失败会被计两次，而 memory 缓存下两边计数器互不可见，阈值等于被腰斩。
		wafhostguard.Start()
		// 独占单例已在本函数内联启动完毕，这里把 activateOnce 消费掉，
		// 使后续任何 ACTIVATE 都成为空操作。Supervisor 重启后"原地认领"存活 Worker 时
		// 会补发一次 ACTIVATE（它无从判断该 Worker 当初是否已接管过单例），
		// 没有这道守卫就会把隧道/应用/cron 重复启动一遍。
		markSingletonsActivated()
	}

	//脱敏处理初始化
	global.GWAF_DLP, _ = dlp.NewEngine("wafDlp")
	err = global.GWAF_DLP.ApplyConfig(ldpConfig)
	if err != nil {
		zlog.Info("ldp init error", err)
	} else {
		// 注册自定义脱敏规则
		global.GWAF_DLP.RegisterMasker("LoginSensitiveInfoMaskRule", func(in string) (string, error) {

			// 分割成键值对
			pairs := strings.Split(in, "&")
			// 遍历每个键值对，对值进行脱敏处理
			for i, pair := range pairs {
				keyValue := strings.SplitN(pair, "=", 2)
				if len(keyValue) != 2 {
					continue
				}
				value := keyValue[1]
				if len(value) > 2 {
					value = value[:1] + strings.Repeat("*", len(value)-2) + value[len(value)-1:]
				}
				pairs[i] = keyValue[0] + "=" + value
			}
			// 将处理后的键值对重新组合成字符串
			return "【已脱敏】" + strings.Join(pairs, "&"), nil
		})

	}
	//加载授权信息
	verifyResult, info, err := wafreg.VerifyServerRegByDefaultFile()
	if verifyResult {
		global.GWAF_REG_INFO = info
		zlog.Debug("授权信息 调试信息", info)
		expiryDay, isExpiry := wafreg.CheckExpiry(info.ExpiryDate)
		if isExpiry {
			global.GWAF_REG_INFO.IsExpiry = true
			zlog.Info("授权信息已经过期")
		} else {
			global.GWAF_REG_INFO.IsExpiry = false
			zlog.Info("授权信息还剩余:" + strconv.Itoa(expiryDay) + "天")
		}
	} else {
		zlog.Info("regInfo", err)
	}

	zlog.Info("HomeWaf has started successfully.")
	if global.GWAF_SECURITY_ENTRY_ENABLE {
		zlog.Info("Use homewaf console-info to view the management entry address.")
	} else {
		for _, address := range utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, global.GWAF_SSL_ENABLE, "") {
			zlog.Info("Console address: " + address)
		}
	}
	// 仅全新安装(本次启动创建了默认管理员)时提示随机初始口令位置；按系统语言环境选择中/英文(默认英文，见 utils.IsChineseEnv 跨平台实现)
	if isFreshInstall {
		if utils.IsChineseEnv() {
			zlog.Info("【全新安装】已生成随机初始管理员口令，请查看当前目录文件 data/initial_password.txt ，并在首次登录后立即修改密码")
		} else {
			zlog.Info("[Fresh Install] A random initial admin password has been generated, please see file: data/initial_password.txt and change it right after first login")
		}
	}
	waf_service.WafPipelineServiceApp.OnChange = wafpipeline.Listeners.Apply
	initializePortPipeline()
	if err := waf_service.WafPipelineServiceApp.Load(); err != nil {
		zlog.Warn("部分端口配置未加载，其他端口继续启动", zap.Error(err))
	}
	if err := waf_service.WafPipelineServiceApp.RefreshSourceCertificates(); err != nil {
		zlog.Warn("端口证书启动同步失败", err.Error())
	}
	if err := wafpipeline.Listeners.StartAll(); err != nil {
		zlog.Warn("部分端口监听失败，其他端口与管理入口保持可用", zap.Error(err))
	}
	sslCtx, sslCancel := context.WithCancel(context.Background())
	m.sslWorkerMu.Lock()
	m.sslWorkerCancel = sslCancel
	m.sslWorkerMu.Unlock()
	defer sslCancel()
	go wafenginecore.RunSSLOrderQueue(sslCtx)
	m.startup.Ready()
	for {
		select {
		case common := <-global.GWAF_CHAN_COMMON_MSG:
			if common.Type == enums.ChanComTypeTunnel {
				//隧道类型
				switch common.OpType {
				case enums.OP_TYPE_NEW:
					tunnelNew := common.Content.(model.Tunnel)
					netRunTimes := globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.LoadTunnel(tunnelNew)
					for _, netRunTime := range netRunTimes {
						globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.StartTunnelServer(netRunTime)
					}
					break
				case enums.OP_TYPE_UPDATE:
					// 获取修改前的隧道信息
					tunnelOld := common.OldContent.(model.Tunnel)
					// 获取修改后的隧道信息
					tunnelNew := common.Content.(model.Tunnel)
					netRunTimes := globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.EditTunnel(tunnelOld, tunnelNew)
					for _, netRunTime := range netRunTimes {
						globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.StartTunnelServer(netRunTime)
					}
					break
				case enums.OP_TYPE_DELETE:
					tunnelDelete := common.OldContent.(model.Tunnel)
					globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.RemoveTunnel(tunnelDelete)
					break
				}
			} else if common.Type == enums.ChanComTypeApp {
				// 应用管理类型
				if globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE != nil {
					switch common.OpType {
					case enums.OP_TYPE_NEW:
						// 新增：如果 AutoStart=1，则启动
						appNew := common.Content.(model.WafApp)
						if appNew.AutoStart == 1 && appNew.StartStatus == 1 {
							if err := globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.StartApp(appNew.Code); err != nil {
								zlog.Error("自动启动应用失败", "code", appNew.Code, "error", err.Error())
							}
						}
					case enums.OP_TYPE_UPDATE:
						appNew := common.Content.(model.WafApp)
						globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.LoadApp(appNew)
					case enums.OP_TYPE_DELETE:
						appDel := common.OldContent.(model.WafApp)
						// 先停止进程（能读到 DB 中的 StopMode/StopCmd/StopTimeout 配置），再删 DB 记录
						globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.RemoveApp(appDel.Code)
						global.GWAF_LOCAL_DB.Where("id = ?", appDel.Id).Delete(&model.WafApp{})
					case enums.OP_TYPE_APP_START:
						code := common.Content.(string)
						if err := globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.StartApp(code); err != nil {
							zlog.Error("启动应用失败", "code", code, "error", err.Error())
						}
					case enums.OP_TYPE_APP_STOP:
						code := common.Content.(string)
						if err := globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.StopApp(code); err != nil {
							zlog.Error("停止应用失败", "code", code, "error", err.Error())
						}
					case enums.OP_TYPE_APP_RESTART:
						code := common.Content.(string)
						if err := globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.RestartApp(code); err != nil {
							zlog.Error("重启应用失败", "code", code, "error", err.Error())
						}
					}
				}
			}
		case update := <-global.GWAF_CHAN_UPDATE:
			if update == 1 {
				// 优雅升级：二进制已被 selfupdate 替换到磁盘。
				// 在 Supervisor 模式下，请求 Supervisor 编排滚动升级（起新 Worker→就绪→本 Worker 排空退出），
				// 本进程不自行退出，等待 Supervisor 下发 DRAIN，从而实现业务零中断。
				if gWorkerCtrl != nil {
					zlog.Info("[Worker] 二进制已就绪，请求 Supervisor 发起滚动升级（业务不中断）")
					if err := gWorkerCtrl.RequestUpgrade(); err != nil {
						global.GWAF_RUNTIME_IS_UPDATETING = false
						wafupdate.GlobalUpdateProgress.Fail(err)
						zlog.Error("请求平滑升级失败: " + err.Error())
					}
					break
				}
				// 兜底：非 Supervisor 托管的独立运行模式，沿用原“停止→拉起新进程→退出”的硬重启。
				global.GWAF_RUNTIME_SERVER_TYPE = !service.Interactive()
				//需要重新启动
				if global.GWAF_RUNTIME_SERVER_TYPE == true {
					zlog.Info("服务形式重启")
					m.stopHomeWaf()

					// 只启动一次新进程
					cmd := exec.Command(global.GWAF_RUNTIME_CURRENT_EXEPATH, "restart")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					wafexec.FixStdin(cmd)
					if err := cmd.Start(); err != nil {
						zlog.Error("启动新进程失败:", err)
						os.Exit(0)
					}
					time.Sleep(2 * time.Second)
					os.Exit(0)
				} else {
					m.stopHomeWaf()
					cmd := exec.Command(global.GWAF_RUNTIME_CURRENT_EXEPATH)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					wafexec.FixStdin(cmd)
					if err := cmd.Start(); err != nil {
						zlog.Error("启动新进程失败:", err)
						os.Exit(0)
					}
					time.Sleep(2 * time.Second)
					os.Exit(0)
				}
			}
			break

		case sslExpireCheck := <-global.GWAF_CHAN_SSL_EXPIRE_CHECK:
			zlog.Debug("ssl证书到期检测", sslExpireCheck)
			waftask.SSLExpireCheck()
			break
		case syncHostToSslExpire := <-global.GWAF_CHAN_SYNC_HOST_TO_SSL_EXPIRE:
			zlog.Debug("同步已存在主机到SSL证书检测任务里", syncHostToSslExpire)
			waftask.SyncHostToSslCheck()
			break
		case taskMethod := <-global.GWAF_CHAN_TASK:
			zlog.Debug("需要执行的方法", taskMethod)
			globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.RunManual(taskMethod)
			break
		case taskReload := <-global.GWAF_CHAN_TASK_RELOAD:
			zlog.Info("任务重新加载", "taskName", taskReload.TaskName, "taskMethod", taskReload.TaskMethod)
			err := globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.RescheduleTask(
				taskReload.TaskUnit,
				taskReload.TaskValue,
				taskReload.TaskAt,
				taskReload.TaskMethod,
				taskReload.TaskDaysOfTheWeek,
			)
			if err != nil {
				zlog.Error("任务重新加载失败", "error", err.Error(), "taskName", taskReload.TaskName)
			} else {
				zlog.Info("任务重新加载成功", "taskName", taskReload.TaskName)
			}
			break

		}

	}
}

// 停止要提前关闭的 是服务的主要逻辑
func (m *wafSystenService) stopHomeWaf() {
	m.sslWorkerMu.Lock()
	if m.sslWorkerCancel != nil {
		m.sslWorkerCancel()
	}
	m.sslWorkerMu.Unlock()
	shutdownPortPipeline()
	zlog.Info("Shutdown HomeWaf Tunnel Engine...")
	if globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE != nil {
		globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.CloseTunnel()
		zlog.Info("Shutdown HomeWaf Tunnel Engine finished")
	} else {
		zlog.Warn("Tunnel Engine is nil, skipping shutdown")
	}

	zlog.Info("Shutdown HomeWaf App Engine...")
	if globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE != nil {
		globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.StopApps()
		zlog.Info("Shutdown HomeWaf App Engine finished")
	} else {
		zlog.Warn("App Engine is nil, skipping shutdown")
	}

	// 站点流量落库：引擎已停、不会再有新字节进来，这里补最后一次，
	// 把内存里没到 30 秒周期的增量写掉（否则每次重启都会丢掉最后不足一个周期的流量）。
	zlog.Info("Flush HomeWaf Traffic Stats...")
	waftask.FlushTrafficStats()
	zlog.Info("Flush HomeWaf Traffic Stats finished")

	zlog.Info("Shutdown HomeWaf Queue Processors...")
	// 关闭信号通道，通知所有队列处理协程退出
	close(global.GWAF_QUEUE_SHUTDOWN_SIGNAL)
	// 等待一段时间，让队列处理协程有时间完成当前工作并退出
	time.Sleep(500 * time.Millisecond)
	zlog.Info("Shutdown HomeWaf Queue Processors finished")

	// 设置任务停止标志
	zlog.Info("Notifying HomeWaf Tasks to shutdown...")
	global.GWAF_SHUTDOWN_SIGNAL = true
	// 给任务一些时间完成当前工作
	time.Sleep(200 * time.Millisecond)
	zlog.Info("HomeWaf Tasks notified")

	// 主机登录防护要早于 cron 与关库停：Stop 里要等事件源交还 fd / journalctl 子进程 /
	// Windows 事件订阅句柄，还要把缓冲里的事件与待同步的封禁集合落地。
	// 停晚了就会出现"旧 Worker 还在读日志、新 Worker 也开始读"的重复计数。
	zlog.Info("Shutdown HomeWaf HostGuard...")
	wafhostguard.Stop()
	waf_service.WafHostGuardServiceApp.FlushEvents()
	zlog.Info("Shutdown HomeWaf HostGuard finished")

	zlog.Info("Shutdown HomeWaf Cron...")
	if globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler != nil {
		globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.Stop()
	}
	zlog.Info("Shutdown HomeWaf Cron finished")

	zlog.Info("Shutdown HomeWaf WebManager...")
	if webmanager != nil {
		webmanager.Shutdown()
	}
	zlog.Info("Shutdown HomeWaf WebManager finished")

	zlog.Info("Shutdown HomeWaf IPDatabase...")
	utils.CloseIPDatabase()
	zlog.Info("Shutdown HomeWaf IPDatabase finished")

	// 关闭插件系统
	zlog.Info("Shutdown HomeWaf Plugin System...")
	if globalobj.GWAF_RUNTIME_OBJ_PLUGIN_MANAGER != nil {
		if err := globalobj.GWAF_RUNTIME_OBJ_PLUGIN_MANAGER.Shutdown(); err != nil {
			zlog.Error("Shutdown Plugin System failed", zap.Error(err))
		} else {
			zlog.Info("Shutdown HomeWaf Plugin System finished")
		}
	} else {
		zlog.Warn("Plugin Manager is nil, skipping shutdown")
	}

}

// supervisorService 是被服务管理器(kardianos)托管的“监护进程”程序。
// 它常驻、永不为升级而退出，负责拉起/监护/滚动升级 Worker 子进程。
type supervisorService struct {
	sup *supervisor.Supervisor
}

func (s *supervisorService) Start(svc service.Service) error {
	zlog.Info("[Supervisor] 服务启动")
	go func() {
		if err := s.sup.Run(); err != nil {
			zlog.Error("[Supervisor] 运行失败: " + err.Error())
		}
	}()
	return nil
}

func (s *supervisorService) Stop(svc service.Service) error {
	zlog.Info("[Supervisor] 服务停止")
	s.sup.Shutdown()
	return nil
}

// gracefulOnce 保证优雅停止只执行一次，无论由服务 Stop()、系统信号，
// 还是后续 Supervisor 的 DRAIN 指令触发。
var gracefulOnce sync.Once

// workerLogExportLifecycle 将启动与停止串行化，停止后不再接受ACTIVATE启动。
type workerLogExportLifecycle struct {
	mu       sync.Mutex
	stopping bool
}

func (l *workerLogExportLifecycle) start(start func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.stopping {
		start()
	}
}

func (l *workerLogExportLifecycle) stop(stop func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopping {
		return
	}
	l.stopping = true
	stop()
}

var logExportWorkerLifecycle workerLogExportLifecycle

// Graceful 优雅停止的统一入口：先排空业务连接(StopAllProxyServer 已支持 Shutdown 排空)，
// 回收各引擎/队列/任务资源，最后关闭数据库连接。供信号、服务 Stop 和升级 DRAIN 复用。
func (m *wafSystenService) Graceful() {
	gracefulOnce.Do(func() {
		m.startup.Fail(errors.New("Worker 正在排空退出"))
		zlog.Info("[Graceful] 开始优雅停止：排空业务连接并回收资源 ...")
		zlog.Info("Shutdown HomeWaf LogExport...")
		logExportWorkerLifecycle.stop(waf_service.WafLogExportServiceApp.Stop)
		zlog.Info("Shutdown HomeWaf LogExport finished")
		m.stopHomeWaf()
		wafsafeclear.SafeClear() // 最后关闭数据库连接
		zlog.Info("[Graceful] 优雅停止完成")
	})
}

// @title           HomeWaf Open API
// @version         1.0
// @description     HomeWaf Web Application Firewall 开放平台 API 文档。需要在请求头携带 X-API-Key 进行鉴权。
// @BasePath        /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in              header
// @name            X-API-Key
func main() {
	if len(os.Args) == 3 && os.Args[1] == "check-install-format" {
		if err := wafinit.CheckInstallFormat(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	applyHomeWafEndpoints()
	if wafcli.IsAutomationCommand(os.Args) {
		wafcli.SetResultOutput(os.Stdout)
		os.Stdout = os.Stderr
		defer func() {
			if recover() != nil {
				os.Exit(wafcli.ReportStartupFailure())
			}
		}()
	}
	global.GWAF_WEB_VERSION = public.WebFrontendVersion()
	fmt.Println(`
==========================================
  HomeWaf Web Application Firewall ` + global.GWAF_RELEASE_VERSION + `
  Version Name: ` + global.GWAF_RELEASE_VERSION_NAME + `
  Web UI Version: ` + global.GWAF_WEB_VERSION + `
==========================================
`)
	if err := wafinit.EnsureInstallFormat(utils.GetCurrentDir()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	wafconfig.LoadAndInitConfig()
	// 多进程日志隔离：Supervisor 与 Worker 是同一二进制的两个进程，若都写同一个 logs/log.log，
	// lumberjack 滚动时 rename 会因文件被另一进程占用而失败(Windows)，导致日志再也滚不动。
	// 故 Supervisor 角色单独写 supervisor.log；Worker 仍写 log.log（主业务日志名不变）。
	// parseWorkerRole 仅扫描 os.Args，不依赖日志/配置，可安全前置调用。
	if isWorker, _, _, _ := parseWorkerRole(); !isWorker {
		zlog.FileName = "supervisor.log"
	}
	// 自升级交接助手是与 Supervisor 并存的第三个短命进程，同理单独写一个日志文件，
	// 免得三个进程抢同一个 supervisor.log 的滚动 rename。
	if len(os.Args) > 1 && os.Args[1] == supervisor.HandoffRestartArg {
		zlog.FileName = "handoff.log"
	}
	if wafcli.IsAutomationCommand(os.Args) {
		zlog.FileName = "cli.log"
	}
	//初始化日志
	zlog.InitZLog(global.GWAF_LOG_DEBUG_ENABLE, global.GWAF_LOG_OUTPUT_FORMAT)
	if v := recover(); v != nil {
		zlog.Error("主流程上被异常了")
	}
	pid := os.Getpid()
	zlog.Debug("HomeWaf Current PID:" + strconv.Itoa(pid))

	// 环境自检：精简版 Windows(如 LTSC 纯净版)可能裁剪/禁用 Null 内核驱动，导致空设备不可用。
	// os/exec 在标准流为 nil 时会去打开该设备，缺失则所有子进程(含 Worker)都起不来。
	// HomeWaf 已通过 wafexec 自动规避，这里只留一条线索：Supervisor 记在 supervisor.log、Worker 记在 log.log。
	if !wafexec.NullDeviceAvailable() {
		zlog.Warn("[环境] 系统空设备 " + os.DevNull + " 不可用（精简版 Windows 常见：Null 内核驱动被裁剪或禁用）。" +
			"HomeWaf 已自动规避，不影响运行；如需彻底恢复，请以管理员执行 `sc config Null start=system` 后重启系统。")
	}

	// 调试标识：响应头标记处理进程，便于验证升级时新旧 Worker 交替。
	// 主开关在 config.yml 的 debug_worker_header（已由上方 LoadAndInitConfig 读入，Supervisor/Worker 均生效，
	// 服务模式下务必用配置项——SCM/systemd 不会传入交互式 shell 的环境变量）。
	// 环境变量 SAMWAF_WORKER_HEADER=1 仅作前台调试的临时叠加开关（只会打开、不会关闭配置已开的开关）。
	global.GWAF_WORKER_TAG = "pid=" + strconv.Itoa(pid) + " born=" + time.Now().Format("15:04:05")
	if os.Getenv("SAMWAF_WORKER_HEADER") == "1" {
		global.GWAF_DEBUG_WORKER_HEADER = true
	}

	// 角色分流：带 --worker 的进程由 Supervisor 拉起，直接运行业务引擎 + 控制通道客户端。
	if workerRole, ctrlAddr, ctrlToken, takeover := parseWorkerRole(); workerRole {
		global.GWAF_RUNTIME_IS_TAKEOVER = takeover
		//获取外网IP
		global.GWAF_RUNTIME_IP = utils.GetExternalIp()
		wp := &wafSystenService{}
		if ctrlAddr != "" {
			startWorkerControl(ctrlAddr, ctrlToken, wp)
		}
		// Worker 自身的信号处理：收到中断/终止(如前台 Ctrl+C、systemd SIGTERM)时优雅排空再退出。
		// 与 Supervisor 下发的 DRAIN/SHUTDOWN 经 gracefulOnce 去重，只执行一次。
		go func() {
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
			s := <-sc
			zlog.Info("[Worker] 收到系统信号优雅停止: " + s.String())
			wp.Graceful()
			os.Exit(0)
		}()
		zlog.Info("以 Worker 角色启动 (takeover=" + strconv.FormatBool(takeover) + ")")
		wp.run() // 阻塞运行引擎事件循环
		return
	}
	// ===== 以下为 Supervisor 角色（默认，被服务管理器托管，常驻不为升级退出）=====

	option := service.KeyValue{}
	//windows
	//OnFailure:"restart",
	if runtime.GOOS == "windows" {
		option["OnFailure"] = "restart"
		option["OnFailureDelayDuration"] = "1s"
		option["OnFailureResetPeriod"] = "10"
	} else {
		option["Restart"] = "always"
	}

	// 创建服务对象
	svcConfig := &service.Config{
		Name:        "HomeWafService",
		DisplayName: "HomeWaf Service",
		Description: "HomeWaf Web Application Firewall",
		Option:      option,
	}
	exePath, _ := os.Executable()
	dataDir := filepath.Join(utils.GetCurrentDir(), "data")
	sup := supervisor.New(supervisor.Options{
		ExePath:      exePath,
		DataDir:      dataDir,
		DrainTimeout: int(global.CurrentSettings().GCONFIG_RECORD_DRAIN_TIMEOUT),
		// 自升级交接(Windows)靠"退出 + 助手把服务重新 start"，前台运行时退出等于停服，
		// 故把"是否由服务管理器托管"传下去，由 Supervisor 决定是否交接。
		ServiceManaged: !service.Interactive(),
	})
	prg := &supervisorService{sup: sup}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	// 设置服务控制信号处理程序：收到中断/终止信号时，令 Supervisor 通知各 Worker 优雅退出再停止。
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		zlog.Info("收到系统信号，Supervisor 开始优雅停止: " + sig.String())
		sup.Shutdown()
		os.Exit(0)
	}()

	if len(os.Args) > 1 {
		os.Exit(wafcli.Execute(wafcli.Deps{
			ServiceControl: func(action string) error { return service.Control(s, action) },
			HandoffRestart: func() { runHandoffRestart(s) },
			DataDir:        dataDir,
		}))
	}

	if service.Interactive() {
		zlog.Info("main server under service manager")
		global.GWAF_RUNTIME_SERVER_TYPE = service.Interactive()
	} else {
		zlog.Info("main server not under service manager")
		global.GWAF_RUNTIME_SERVER_TYPE = service.Interactive()
	}
	// 以常规方式运行
	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}

}

// ===== 优雅升级 Worker 侧控制通道客户端（与 Supervisor 通信） =====

// parseWorkerRole 解析进程角色与控制参数。
// 由 Supervisor 拉起的 Worker 会带 --worker --ctrl-addr=127.0.0.1:port --token=xxx [--takeover]。
func parseWorkerRole() (worker bool, ctrlAddr, token string, takeover bool) {
	for _, a := range os.Args[1:] {
		switch {
		case a == "--worker":
			worker = true
		case a == "--takeover":
			takeover = true
		case strings.HasPrefix(a, "--ctrl-addr="):
			ctrlAddr = strings.TrimPrefix(a, "--ctrl-addr=")
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		}
	}
	return
}

// workerCtrl 是 Worker 侧的控制通道客户端：连接 Supervisor、上报 HELLO/READY/HEARTBEAT、
// 响应 DRAIN/SHUTDOWN/PING；断线自动重连（支撑 Supervisor 自升级后的“再认领”）。
type workerCtrl struct {
	addr  string
	token string
	prg   *wafSystenService

	mu   sync.Mutex
	conn *wafipc.Conn
	down bool
}

// gWorkerCtrl 当前进程的 Worker 控制客户端（仅 worker 角色下非 nil）。
var gWorkerCtrl *workerCtrl

// runHandoffRestart 是 Supervisor 自升级交接的"重启助手"（Windows 隐藏命令）。
//
// Windows 没有 execve，Supervisor 换二进制只能退出再被拉起来。这里不依赖 SCM 的
// 失败恢复动作（那是服务安装时写入的，老版本装的服务可能压根没配），而是由老
// Supervisor 先起本进程、随后自己干净退出，本进程轮询把服务 start 回来。
//
// 期间 Worker 作为孤儿继续转发业务，新 Supervisor 起来后靠 supervisor.state 里的
// ctrl 端口 + token 把它收编回去（同版本原地认领，不重起 Worker）。
func runHandoffRestart(s service.Service) {
	const (
		interval = 1 * time.Second
		timeout  = 60 * time.Second
	)
	zlog.Info("[交接助手] 等待旧 Supervisor 退出后把服务重新拉起...")
	deadline := time.Now().Add(timeout)
	sawStopped := false
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		if st, err := s.Status(); err == nil && st == service.StatusRunning {
			if sawStopped {
				// 服务停过又跑起来了——可能是 SCM 的失败恢复动作先我们一步。目的已达成。
				zlog.Info("[交接助手] 服务已重新运行(由服务管理器先行拉起)，交接完成")
				return
			}
			// 旧 Supervisor 还没退干净，继续等。
			continue
		}
		sawStopped = true
		if err := service.Control(s, "start"); err != nil {
			zlog.Debug("[交接助手] start 暂未成功，稍后重试: " + err.Error())
			continue
		}
		zlog.Info("[交接助手] 服务已以新二进制重新启动，交接完成")
		return
	}
	// 拉不起来不代表业务断了：Worker 仍在转发流量，只是暂时没有监护进程。
	zlog.Error("[交接助手] " + timeout.String() + " 内未能把服务拉起来。Worker 仍在服务，" +
		"但监护进程缺席，请人工执行 HomeWaf64.exe start")
}

// activateOnce 保证独占单例(应用/隧道/cron)只被接管启动一次。
var activateOnce sync.Once

// activateSingletons 启动独占型单例：隧道、应用、cron 任务调度。
// takeover 升级模式下由 Supervisor 在旧 Worker 退出后经 ACTIVATE 触发（此时旧 Worker 已释放这些资源，避免端口/进程冲突）；
// 非 takeover（首次启动/自愈重拉）则在 run() 内联启动，不经过这里。
func activateSingletons() {
	activateOnce.Do(func() {
		zlog.Info("[Worker] 接管独占单例：启动隧道 / 应用 / 任务调度")
		if globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE != nil {
			globalobj.GWAF_RUNTIME_OBJ_TUNNEL_ENGINE.StartTunnel()
		}
		if globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE != nil {
			globalobj.GWAF_RUNTIME_OBJ_APP_ENGINE.StartApps()
		}
		if globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler != nil {
			globalobj.GWAF_RUNTIME_OBJ_WAF_TaskScheduler.Start()
		}
		// 日志导出执行器同属独占单例：与任务调度一同只在活动 Worker 中运行。
		logExportWorkerLifecycle.start(waf_service.WafLogExportServiceApp.Start)
		// 主机登录防护的日志采集也必须等旧 Worker 彻底退出后才启动，
		// 否则两个进程同时读同一份 auth.log，失败次数被重复计入，阈值等于打了对折。
		wafhostguard.Start()
	})
}

// markSingletonsActivated 把 activateOnce 消费掉而不做任何事，
// 供"独占单例已在 run() 内联启动"的非 takeover 路径调用，让之后收到的
// ACTIVATE 变成空操作（幂等），避免隧道/应用/cron 被重复启动。
func markSingletonsActivated() {
	activateOnce.Do(func() {})
}

// startWorkerControl 启动 Worker 控制通道客户端（后台重连循环）。
func startWorkerControl(addr, token string, prg *wafSystenService) {
	gWorkerCtrl = &workerCtrl{addr: addr, token: token, prg: prg}
	go gWorkerCtrl.loop()
}

func (wc *workerCtrl) loop() {
	for {
		if wc.isDown() {
			return
		}
		conn, err := wafipc.Dial(wc.addr)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		wc.setConn(conn)
		wc.send(wafipc.Message{
			Type:     wafipc.MsgHello,
			PID:      os.Getpid(),
			Version:  global.GWAF_RELEASE_VERSION,
			ProtoVer: wafipc.ProtoVersion,
			Token:    wc.token,
		})
		wc.serve(conn) // 阻塞直到连接出错
		conn.Close()
		wc.setConn(nil)
		if wc.isDown() {
			return
		}
		time.Sleep(2 * time.Second) // 重连
	}
}

func (wc *workerCtrl) serve(conn *wafipc.Conn) {
	stop := make(chan struct{})
	defer close(stop)
	ctx, cancelReady := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReady()

	// Publish READY only after initialization and this worker's listeners succeed.
	go func() {
		if err := wc.prg.waitReady(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				zlog.Error("[Worker] 就绪检查失败，保留旧 Worker: " + err.Error())
				wc.send(wafipc.Message{Type: wafipc.MsgReadyFailed, PID: os.Getpid(), Err: err.Error()})
			}
			return
		}
		wc.mu.Lock()
		defer wc.mu.Unlock()
		if wc.conn == conn && !wc.down && ctx.Err() == nil {
			_ = conn.Send(wafipc.Message{Type: wafipc.MsgReady, PID: os.Getpid()})
		}
	}()

	// 心跳：上报当前连接数
	go func() {
		if wc.prg.startup.Wait(ctx) != nil {
			return
		}
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				conns := wafpipeline.Listeners.Count()
				wc.send(wafipc.Message{Type: wafipc.MsgHeartbeat, PID: os.Getpid(), Active: conns})
			}
		}
	}()

	// 命令读取
	for {
		m, err := conn.Recv()
		if err != nil {
			return
		}
		switch m.Type {
		case wafipc.MsgDrain, wafipc.MsgShutdown:
			zlog.Info("[Worker] 收到 " + m.Type + " 指令，开始优雅排空退出")
			go wc.gracefulExit()
			return
		case wafipc.MsgActivate:
			zlog.Info("[Worker] 收到 ACTIVATE 指令，接管独占单例(应用/隧道/任务调度)")
			go activateSingletons()
		case wafipc.MsgUpgradeFailed:
			global.GWAF_RUNTIME_IS_UPDATETING = false
			wafupdate.GlobalUpdateProgress.Fail(errors.New(m.Err))
			zlog.Error("[Worker] 平滑升级失败，旧版本继续服务: " + m.Err)
		case wafipc.MsgPing:
			wc.send(wafipc.Message{Type: wafipc.MsgPong, PID: os.Getpid()})
		}
	}
}

func (wc *workerCtrl) gracefulExit() {
	wc.markDown()
	wc.prg.Graceful()
	wc.send(wafipc.Message{Type: wafipc.MsgGone, PID: os.Getpid()})
	time.Sleep(200 * time.Millisecond)
	os.Exit(0)
}

// RequestUpgrade 通知 Supervisor 发起滚动升级（二进制已被 selfupdate 替换到磁盘）。
func (wc *workerCtrl) RequestUpgrade() error {
	return wc.send(wafipc.Message{Type: wafipc.MsgRequestUpgrade, PID: os.Getpid()})
}

func (wc *workerCtrl) send(m wafipc.Message) error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if wc.conn != nil {
		return wc.conn.Send(m)
	}
	return errors.New("Supervisor 控制连接不可用，请稍后重试")
}

func (m *wafSystenService) waitReady(ctx context.Context) error {
	if err := m.startup.Wait(ctx); err != nil {
		return err
	}
	if webmanager == nil {
		return errors.New("Worker 引擎或管理端未初始化")
	}
	if err := wafpipeline.Listeners.WaitReady(ctx); err != nil {
		return err
	}
	return webmanager.WaitReady(ctx)
}

func (wc *workerCtrl) setConn(c *wafipc.Conn) {
	wc.mu.Lock()
	wc.conn = c
	wc.mu.Unlock()
}

func (wc *workerCtrl) isDown() bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return wc.down
}

func (wc *workerCtrl) markDown() {
	wc.mu.Lock()
	wc.down = true
	wc.mu.Unlock()
}
