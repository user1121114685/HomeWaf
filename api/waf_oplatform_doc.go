// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package api

import (
	"HomeWaf/global"
	"HomeWaf/model/common/response"
	"HomeWaf/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"sort"
	"strings"
)

// GinEngineRef 保存 gin.Engine 引用，在 localserver 初始化路由后设置
var GinEngineRef *gin.Engine

type WafOPlatformDocApi struct{}

// apiRouteInfo API路由文档信息
type apiRouteInfo struct {
	Method          string `json:"method"`
	Path            string `json:"path"`
	Module          string `json:"module"`
	Description     string `json:"description"`
	ParamExample    string `json:"param_example"`
	ResponseExample string `json:"response_example"`
	CurlExample     string `json:"curl_example"`
}

// routeDetail 静态路由注释信息
type routeDetail struct {
	Description     string
	ParamExample    string
	ResponseExample string
}

// apiDocResponse API文档响应结构
type apiDocResponse struct {
	AuthInfo   authInfo                  `json:"auth_info"`
	BaseURL    string                    `json:"base_url"`
	TotalCount int                       `json:"total_count"`
	Modules    map[string][]apiRouteInfo `json:"modules"`
}

type authInfo struct {
	HeaderName  string `json:"header_name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// routeDetailMap 各接口的静态参数说明注册表，key 格式为 "METHOD /path"
var routeDetailMap = map[string]routeDetail{
	// ======== 日志-攻击日志 ========
	"POST /api/v1/waflog/attack/list": {
		Description:     "获取攻击日志列表（分页）",
		ParamExample:    `{"pageIndex":1,"pageSize":10,"host_code":"","src_ip":"","action":"","unix_add_time_begin":"2026-01-01 00:00:00","unix_add_time_end":"2026-12-31 23:59:59"}`,
		ResponseExample: `{"code":0,"data":{"list":[{"req_uuid":"xxx","src_ip":"1.2.3.4","rule":"SQL注入","action":"block","time_str":"2026-03-06 10:00:00"}],"total":100},"msg":"查询成功"}`,
	},
	"GET /api/v1/waflog/attack/detail": {
		Description:     "获取攻击日志详情",
		ParamExample:    `?req_uuid=xxx-yyy-zzz&current_db_name=local_log`,
		ResponseExample: `{"code":0,"data":{"req_uuid":"xxx","src_ip":"1.2.3.4","method":"POST","url":"/login","body":"..."},"msg":"查询成功"}`,
	},

	// ======== 统计-数据统计 ========
	"GET /api/v1/wafstatsumdayrange": {
		Description:     "按日期范围统计访问量/拦截量",
		ParamExample:    `?start_day=2026-01-01&end_day=2026-03-06`,
		ResponseExample: `{"code":0,"data":{"list":[{"date":"2026-03-06","total":1000,"block":50}]},"msg":"查询成功"}`,
	},
	"GET /api/v1/wafstatsumdaytopiprange": {
		Description:     "统计攻击IP排行",
		ParamExample:    `?start_day=2026-01-01&end_day=2026-03-06`,
		ResponseExample: `{"code":0,"data":{"list":[{"src_ip":"1.2.3.4","count":500}]},"msg":"查询成功"}`,
	},

	// ======== 系统日志 ========
	"GET /api/v1/sys_log/list": {
		Description:     "获取系统操作日志列表（分页）",
		ParamExample:    `?pageIndex=1&pageSize=10&op_type=&op_content=`,
		ResponseExample: `{"code":0,"data":{"list":[{"op_type":"login","op_content":"用户登录","time_str":"2026-03-06 10:00:00"}],"total":50},"msg":"查询成功"}`,
	},

	// ======== 账号管理 ========
	"POST /api/v1/account/list": {
		Description:     "获取账号列表（分页）",
		ParamExample:    `{"pageIndex":1,"pageSize":10}`,
		ResponseExample: `{"code":0,"data":{"list":[{"id":"xxx","user_name":"admin","status":1}],"total":1},"msg":"查询成功"}`,
	},
	"POST /api/v1/account/add": {
		Description:     "新增账号",
		ParamExample:    `{"user_name":"apiuser","password":"Password@123","status":1}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},
	"GET /api/v1/account/del": {
		Description:     "删除账号",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 系统配置 ========
	"POST /api/v1/systemconfig/list": {
		Description:     "获取系统配置项列表",
		ParamExample:    `{"pageIndex":1,"pageSize":50,"config_type":""}`,
		ResponseExample: `{"code":0,"data":{"list":[{"config_type":"ssl","config_key":"sslorder_expire_day","config_value":"30"}],"total":10},"msg":"查询成功"}`,
	},
	"POST /api/v1/systemconfig/editByItem": {
		Description:     "更新单个系统配置项",
		ParamExample:    `{"config_type":"ssl","config_key":"sslorder_expire_day","config_value":"15"}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 系统信息 ========
	"GET /api/v1/sysinfo/runtimeinfo": {
		Description:     "获取系统运行信息（CPU、内存、版本等）",
		ParamExample:    `(无需参数)`,
		ResponseExample: `{"code":0,"data":{"cpu_percent":10.5,"mem_percent":40.2,"version":"1.x.x"},"msg":"查询成功"}`,
	},

	// ======== SSL证书配置 ========
	"POST /api/v1/sslconfig/list": {
		Description:     "获取SSL证书列表（分页）",
		ParamExample:    `{"pageIndex":1,"pageSize":10}`,
		ResponseExample: `{"code":0,"data":{"list":[{"id":"xxx","domain":"example.com","expire_time":"2027-01-01"}],"total":1},"msg":"查询成功"}`,
	},
	"POST /api/v1/sslconfig/add": {
		Description:     "上传新增SSL证书",
		ParamExample:    `{"domain":"example.com","cert_content":"-----BEGIN CERTIFICATE-----...","key_content":"-----BEGIN PRIVATE KEY-----..."}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},
	"GET /api/v1/sslconfig/del": {
		Description:     "删除SSL证书",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 通知渠道 ========
	"POST /api/v1/notify/channel/list": {
		Description:     "获取通知渠道列表",
		ParamExample:    `{"pageIndex":1,"pageSize":10}`,
		ResponseExample: `{"code":0,"data":{"list":[{"id":"xxx","channel_name":"企业微信","channel_type":"wechat_work","status":1}],"total":1},"msg":"查询成功"}`,
	},
	"POST /api/v1/notify/channel/add": {
		Description:     "新增通知渠道",
		ParamExample:    `{"channel_name":"钉钉机器人","channel_type":"dingtalk","webhook_url":"https://oapi.dingtalk.com/robot/send?access_token=xxx","status":1}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},
	"GET /api/v1/notify/channel/del": {
		Description:     "删除通知渠道",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 通知订阅 ========
	"POST /api/v1/notify/subscription/list": {
		Description:     "获取通知订阅列表",
		ParamExample:    `{"pageIndex":1,"pageSize":10}`,
		ResponseExample: `{"code":0,"data":{"list":[],"total":0},"msg":"查询成功"}`,
	},
	"POST /api/v1/notify/subscription/add": {
		Description:     "新增通知订阅（将事件与渠道关联）",
		ParamExample:    `{"channel_id":"xxx","event_type":"attack_block","host_code":""}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 防火墙IP封禁 ========
	"POST /api/v1/firewall/ipblock/list": {
		Description:     "获取防火墙IP封禁规则列表（分页）",
		ParamExample:    `{"pageIndex":1,"pageSize":10}`,
		ResponseExample: `{"code":0,"data":{"list":[],"total":0},"msg":"查询成功"}`,
	},
	"POST /api/v1/firewall/ipblock/add": {
		Description:     "新增防火墙IP封禁规则",
		ParamExample:    `{"ip":"1.2.3.4","remarks":"攻击者","expire_time":""}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},
	"GET /api/v1/firewall/ipblock/del": {
		Description:     "删除防火墙IP封禁规则",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 开放平台-Key管理 ========
	"POST /api/v1/oplatform/key/list": {
		Description:     "获取OpenAPI Key列表（分页）",
		ParamExample:    `{"pageIndex":1,"pageSize":10,"key_name":""}`,
		ResponseExample: `{"code":0,"data":{"list":[{"id":"xxx","key_name":"我的Key","api_key":"sk-xxx","status":1,"call_count":100}],"total":1},"msg":"查询成功"}`,
	},
	"POST /api/v1/oplatform/key/add": {
		Description:     "新增OpenAPI Key（系统自动生成api_key和api_secret）",
		ParamExample:    `{"key_name":"第三方系统Key","remark":"用于对接XXX系统","rate_limit":1000,"ip_whitelist":"192.168.1.0/24","expire_time":""}`,
		ResponseExample: `{"code":0,"data":{"api_key":"sk-xxxxxxxx","api_secret":"sk-secret-xxx"},"msg":"操作成功"}`,
	},
	"POST /api/v1/oplatform/key/edit": {
		Description:     "编辑OpenAPI Key信息",
		ParamExample:    `{"id":"xxx","key_name":"更新名称","status":1,"remark":"","rate_limit":500,"ip_whitelist":"","expire_time":"2027-01-01 00:00:00"}`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},
	"GET /api/v1/oplatform/key/del": {
		Description:     "删除OpenAPI Key",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},
	"GET /api/v1/oplatform/key/detail": {
		Description:     "获取OpenAPI Key详情",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{"id":"xxx","key_name":"我的Key","api_key":"sk-xxx","rate_limit":1000,"call_count":500},"msg":"查询成功"}`,
	},
	"POST /api/v1/oplatform/key/resetSecret": {
		Description:     "重置OpenAPI Key的Secret（旧Secret立即失效）",
		ParamExample:    `{"id":"xxx"}`,
		ResponseExample: `{"code":0,"data":{"api_secret":"sk-new-secret-xxx"},"msg":"操作成功"}`,
	},

	// ======== 开放平台-调用日志 ========
	"POST /api/v1/oplatform/log/list": {
		Description:     "获取OpenAPI调用日志列表（分页）",
		ParamExample:    `{"pageIndex":1,"pageSize":10,"key_name":"","request_path":""}`,
		ResponseExample: `{"code":0,"data":{"list":[{"api_key_id":"xxx","key_name":"我的Key","request_path":"/api/v1/pipeline/ports/index","request_method":"GET","status_code":200,"duration":12,"time_str":"2026-03-06 10:00:00"}],"total":50},"msg":"查询成功"}`,
	},
	"GET /api/v1/oplatform/log/detail": {
		Description:     "获取OpenAPI调用日志详情（含请求体和响应体）",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{"request_body":"{...}","response_body":"{...}","client_ip":"192.168.1.1"},"msg":"查询成功"}`,
	},
	"GET /api/v1/oplatform/log/del": {
		Description:     "删除OpenAPI调用日志记录",
		ParamExample:    `?id=xxx`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== WAF引擎 ========
	"POST /api/v1/pipeline/reload": {
		Description:     "重新加载已保存的服务监听与防护策略",
		ParamExample:    `(无需参数)`,
		ResponseExample: `{"code":0,"data":{},"msg":"操作成功"}`,
	},

	// ======== 系统监控 ========
	"GET /api/v1/monitor/system_info": {
		Description:     "获取实时系统监控数据（CPU、内存、网络流量）",
		ParamExample:    `(无需参数)`,
		ResponseExample: `{"code":0,"data":{"cpu":15.2,"mem":40.5,"net_in":1024,"net_out":2048},"msg":"查询成功"}`,
	},
}

// routeModuleMap 路由路径前缀到模块名的映射
var routeModuleMap = map[string]string{
	"/api/v1/pipeline":            "受保护服务",
	"/api/v1/wafhost/rule":        "网站防护-规则管理",
	"/api/v1/sslconfig":           "SSL-证书配置",
	"/api/v1/wafhost/sslorder":    "SSL-证书申请",
	"/api/v1/wafhost/sslexpire":   "SSL-证书过期",
	"/api/v1/wafhost/otp":         "安全-OTP双因素",
	"/api/v1/waflog/attack":       "日志-攻击日志",
	"/api/v1/wafstat":             "统计-数据统计",
	"/api/v1/pipeline/reload":     "引擎-WAF引擎",
	"/api/v1/account":             "账号管理",
	"/api/v1/account_log":         "账号日志",
	"/api/v1/sys_log":             "系统日志",
	"/api/v1/sysinfo":             "系统信息",
	"/api/v1/systemconfig":        "系统配置",
	"/api/v1/wafcommon":           "公共接口",
	"/api/v1/onekeymod":           "一键模式",
	"/api/v1/license":             "License管理",
	"/api/v1/batchtask":           "批量任务",
	"/api/v1/task":                "任务管理",
	"/api/v1/analysis":            "流量分析",
	"/api/v1/privateinfo":         "隐私信息",
	"/api/v1/privategroup":        "隐私分组",
	"/api/v1/tunnel":              "隧道管理",
	"/api/v1/vpconfig":            "VP配置",
	"/api/v1/file":                "文件管理",
	"/api/v1/monitor":             "系统监控",
	"/api/v1/caserver":            "CA服务器",
	"/api/v1/sqlquery":            "SQL查询",
	"/api/v1/notify/channel":      "通知-渠道",
	"/api/v1/notify/subscription": "通知-订阅",
	"/api/v1/notify/log":          "通知-日志",
	"/api/v1/firewall/ipblock":    "防火墙-IP封禁",
	"/api/v1/plugin":              "插件管理",
	"/api/v1/logfilewrite":        "日志文件写入",
	"/api/v1/iplocation":          "IP地址库",
	"/api/v1/oplatform/key":       "开放平台-Key管理",
	"/api/v1/oplatform/log":       "开放平台-调用日志",
}

// getModuleByPath 根据路径判断所属模块
func getModuleByPath(path string) string {
	type prefixEntry struct {
		prefix string
		module string
	}
	var entries []prefixEntry
	for prefix, module := range routeModuleMap {
		entries = append(entries, prefixEntry{prefix, module})
	}
	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].prefix) > len(entries[j].prefix)
	})
	for _, entry := range entries {
		if strings.HasPrefix(path, entry.prefix) {
			return entry.module
		}
	}
	return "其他"
}

// buildCurlExample 生成 curl 调用示例，优先使用静态参数注册表中的真实参数
func buildCurlExample(method, path string, paramExample string) string {
	baseURL := "http://your-homewaf-host:26666"
	upper := strings.ToUpper(method)
	switch upper {
	case "GET":
		queryStr := ""
		if paramExample != "" && strings.HasPrefix(paramExample, "?") {
			queryStr = paramExample
		}
		return fmt.Sprintf(`curl -X GET "%s%s%s" \
  -H "X-API-Key: your_api_key"`, baseURL, path, queryStr)
	case "POST":
		body := `{"pageIndex":1,"pageSize":10}`
		if paramExample != "" && !strings.HasPrefix(paramExample, "?") && paramExample != "(无需参数)" {
			body = paramExample
		}
		return fmt.Sprintf(`curl -X POST "%s%s" \
  -H "X-API-Key: your_api_key" \
  -H "Content-Type: application/json" \
  -d '%s'`, baseURL, path, body)
	default:
		return fmt.Sprintf(`curl -X %s "%s%s" \
  -H "X-API-Key: your_api_key"`, method, baseURL, path)
	}
}

// shouldExcludeRoute 判断该路由是否应该排除出文档（非业务路由）
func shouldExcludeRoute(path string) bool {
	excludePrefixes := []string{
		"/api/v1/public/login",
		"/api/v1/logout",
		"/api/v1/ws",
		"/api/v1/oplatform/doc",
	}
	for _, prefix := range excludePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// GetDocApi 获取 API 文档（供管理界面调用，需要 Token 或 API Key 鉴权）
func (w *WafOPlatformDocApi) GetDocApi(c *gin.Context) {
	if GinEngineRef == nil {
		response.FailWithMessage("文档服务未就绪", c)
		return
	}
	routes := GinEngineRef.Routes()

	modules := make(map[string][]apiRouteInfo)
	totalCount := 0

	for _, route := range routes {
		if shouldExcludeRoute(route.Path) {
			continue
		}
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}

		module := getModuleByPath(route.Path)

		// 查找静态注释信息，key 格式为 "METHOD /path"
		detailKey := strings.ToUpper(route.Method) + " " + route.Path
		detail, hasDetail := routeDetailMap[detailKey]

		description := ""
		paramExample := ""
		responseExample := ""
		if hasDetail {
			description = detail.Description
			paramExample = detail.ParamExample
			responseExample = detail.ResponseExample
		}

		info := apiRouteInfo{
			Method:          route.Method,
			Path:            route.Path,
			Module:          module,
			Description:     description,
			ParamExample:    paramExample,
			ResponseExample: responseExample,
			CurlExample:     buildCurlExample(route.Method, route.Path, paramExample),
		}
		modules[module] = append(modules[module], info)
		totalCount++
	}

	// 对每个模块内的路由按路径排序，便于阅读
	for mod := range modules {
		sort.Slice(modules[mod], func(i, j int) bool {
			return modules[mod][i].Path < modules[mod][j].Path
		})
	}

	doc := apiDocResponse{
		AuthInfo: authInfo{
			HeaderName:  "X-API-Key",
			Description: "在请求头中传入 API Key 进行鉴权，通过管理端「开放平台 > Key管理」创建后使用",
			Example:     "X-API-Key: sk-xxxxxxxxxxxx",
		},
		BaseURL:    "http://your-homewaf-host:26666",
		TotalCount: totalCount,
		Modules:    modules,
	}

	response.OkWithData(doc, c)
}

// GetDocStatusApi 查询开放平台是否已启用（供管理界面 doc.vue 决定是否展示 Swagger UI）
func (w *WafOPlatformDocApi) GetDocStatusApi(c *gin.Context) {
	enabled := int64(0)
	settings := global.CurrentSettings()
	if settings.OpenPlatformEnabled() {
		enabled = 1
	}
	response.OkWithData(gin.H{"enabled": enabled, "enabled_until": settings.OpenPlatformEnabledUntil, "suggested_source": utils.SuggestedMCPSource(c)}, c)
}
