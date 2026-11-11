// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/common/uuid"
	"HomeWaf/common/zlog"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/service/waf_service"
	"HomeWaf/utils"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafenginecore/ipset"
	"HomeWaf/wafenginecore/loadbalance"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafproxy"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	goahocorasick "HomeWaf/internal/multimatch"
	"go.uber.org/zap"
)

// 加载全部host
func (waf *WafEngine) LoadAllHost() {
	//重新查询
	var hosts []model.Hosts
	global.GWAF_LOCAL_DB.Find(&hosts)
	for i := 0; i < len(hosts); i++ {
		waf.LoadHost(hosts[i])
	}
}

// 加载指定host
func (waf *WafEngine) LoadHost(inHost model.Hosts) []innerbean.ServerRunTime {

	// 清理该 host 旧的 Transport 缓存，确保重载(尤其后端 IP/端口变化)时不复用过期连接池。
	// 原先由重载路径中的 RemoveHost 负责；现重载改为 LoadHost 原子替换、不再先 RemoveHost，故在此清理。
	waf.purgeTransportForHost(inHost)

	//检测https
	if inHost.Ssl == 1 {
		// 为主域名加载证书
		waf.AllCertificate.LoadSSL(inHost.Host, inHost.Certfile, inHost.Keyfile)

		// 为绑定的多个域名也加载相同的证书
		if inHost.BindMoreHost != "" {
			lines := strings.Split(inHost.BindMoreHost, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					waf.AllCertificate.LoadSSL(line, inHost.Certfile, inHost.Keyfile)
				}
			}
		}
	}
	if inHost.GLOBAL_HOST == 1 {
		global.GWAF_GLOBAL_HOST_CODE = inHost.Code
	}
	// 端口监听统一走 ResolveHostListens 唯一真源（issue #955）：显式 port_listens_json 优先，
	// 空值按老规则派生；主端口/副端口/AutoJumpHTTPS 隐式 80 不再各写一套判定。
	listens := utils.ResolveHostListens(inHost)
	//定义一个port int数组（需要注册域名路由的副端口，不含主端口与隐式80）
	var ports = []int{}
	for _, listen := range listens {
		if !listen.IsMain && !listen.Implied {
			ports = append(ports, listen.Port)
		}
		if wafpipeline.OwnsLegacyPort(uint16(listen.Port)) {
			continue
		}
		onlineServer, ok := waf.ServerOnline.Get(listen.Port)
		if !ok {
			if inHost.START_STATUS == 0 {
				waf.ServerOnline.Set(listen.Port, innerbean.ServerRunTime{
					ServerType: listen.Protocol,
					IPVersion:  listen.IPVersion,
					Port:       listen.Port,
					Status:     1,
				})
			}
		} else if inHost.START_STATUS == 0 {
			if onlineServer.ServerType != listen.Protocol {
				// 端口是全机共享资源：先加载者定协议，后到者协议不一致时保持先到先得，但必须发声
				waf.alertPortProtocolConflict(inHost, listen.Port, listen.Protocol, onlineServer.ServerType)
			} else if normalizeIPV(onlineServer.IPVersion) != normalizeIPV(listen.IPVersion) {
				// 已有监听不会按新 IP 版本自动重建，静默会让用户以为已生效
				waf.alertPortIPVersionPending(inHost, listen.Port, listen.IPVersion, onlineServer.IPVersion)
			}
		}
	}
	//加载主机对于的规则
	ruleHelper := &utils.RuleHelper{}
	ruleHelper.InitRuleEngine()
	//查询规则
	var vcnt int
	global.GWAF_LOCAL_DB.Model(&model.Rules{}).Where("host_code = ? and rule_status<>999",
		inHost.Code).Select("sum(rule_version) as vcnt").Row().Scan(&vcnt)
	zlog.Debug("主机host" + inHost.Code + " 版本" + strconv.Itoa(vcnt))
	var ruleconfigs []model.Rules
	if vcnt > 0 {
		global.GWAF_LOCAL_DB.Where("host_code = ? and rule_status<>999", inHost.Code).Find(&ruleconfigs)
		ruleHelper.LoadRules(ruleconfigs)
	}
	//查询ip限流(应该针对一个网址只有一个)
	var anticcBean model.AntiCC

	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Limit(1).Find(&anticcBean)

	//初始化插件-ip计数器（与热更新 ApplyAntiCCConfig 共用同一构造函数，避免两处算法不一致）
	pluginIpRateLimiter := BuildIPRateLimiter(anticcBean, inHost.Host)

	//加载并编译CC多规则（编译放在加载期，请求期只做取值与比较）
	ccRules := LoadCompiledCCRules(inHost.Code)

	//查询ip白名单（停用条目不下发引擎，下同）
	var ipwhitelist []model.IPAllowList
	global.GWAF_LOCAL_DB.Where("host_code=? and is_enable = 1", inHost.Code).Find(&ipwhitelist)

	//查询url白名单
	var urlwhitelist []model.URLAllowList
	global.GWAF_LOCAL_DB.Where("host_code=? and is_enable = 1", inHost.Code).Find(&urlwhitelist)

	//查询ip黑名单
	var ipblocklist []model.IPBlockList
	global.GWAF_LOCAL_DB.Where("host_code=? and is_enable = 1", inHost.Code).Find(&ipblocklist)

	//查询url白名单
	var urlblocklist []model.URLBlockList
	global.GWAF_LOCAL_DB.Where("host_code=? and is_enable = 1", inHost.Code).Find(&urlblocklist)

	//查询url隐私保护
	var ldpurls []model.LDPUrl
	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Find(&ldpurls)

	//查询负载均衡
	var loadBalanceList []model.LoadBalance
	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Order("create_time asc").Find(&loadBalanceList)

	//查询HTTP AUTH
	httpAuthList, authLoadErr := waf_service.WafHttpAuthBaseServiceApp.LoadForHost(inHost.Code)
	if authLoadErr != nil {
		zlog.Error("HTTPAuth account load failed", inHost.Code)
	}

	//查询自定义拦截界面
	var blockingPageList []model.BlockingPage
	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Order("update_time ASC, id ASC").Find(&blockingPageList)
	blockingPageMap := map[string]model.BlockingPage{}
	if len(blockingPageList) > 0 {
		for i := 0; i < len(blockingPageList); i++ {
			if blockingPageList[i].BlockingType == "not_match_website" {
				// 域名不匹配使用固定的key
				blockingPageMap["not_match_website"] = blockingPageList[i]
			} else if blockingPageList[i].BlockingType == "other_block" && blockingPageList[i].ResponseCode != "not_match_website" {
				// other_block 类型根据 response_code 区分不同的错误页面
				// 例如: 403(WAF拦截), 404, 500, 502 等
				if blockingPageList[i].ResponseCode != "" {
					blockingPageMap[blockingPageList[i].ResponseCode] = blockingPageList[i]
				}
			}
		}
	}
	//查询缓存规则
	var cacheRuleList []model.CacheRule
	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Find(&cacheRuleList)

	//查询网页防篡改规则（含基线正文，供响应比对/回吐）
	var tamperRuleList []model.TamperRule
	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Find(&tamperRuleList)

	//查询路径路由规则
	var pathRuleList []model.HostPathRule
	global.GWAF_LOCAL_DB.Where("host_code=? ", inHost.Code).Order("priority asc, create_time asc").Find(&pathRuleList)

	//解析静态站点安全配置（供路径规则静态文件服务共享使用）
	var staticCfg model.StaticSiteConfig
	if inHost.StaticSiteJSON != "" {
		_ = json.Unmarshal([]byte(inHost.StaticSiteJSON), &staticCfg)
	}

	//初始化主机host
	hostsafe := &wafenginmodel.HostSafe{
		LoadBalanceRuntime: &wafenginmodel.LoadBalanceRuntime{
			CurrentProxyIndex:       0,
			RevProxies:              []*wafproxy.ReverseProxy{},
			WeightRoundRobinBalance: loadbalance.NewWeightRoundRobinBalance(inHost.Code),
			IpHashBalance:           loadbalance.NewConsistentHashBalance(nil, inHost.Code),
		},
		LoadBalanceLists:       loadBalanceList,
		Rule:                   ruleHelper,
		TargetHost:             inHost.Remote_host + ":" + strconv.Itoa(inHost.Remote_port),
		RuleData:               ruleconfigs,
		RuleVersionSum:         vcnt,
		Host:                   inHost,
		PluginIpRateLimiter:    pluginIpRateLimiter,
		CCRules:                ccRules,
		IPWhiteLists:           ipwhitelist,
		IPWhiteIndex:           BuildIPAllowIndex(ipwhitelist),
		IPWhiteGroupCodes:      ExtractAllowGroupCodes(ipwhitelist),
		UrlWhiteLists:          urlwhitelist,
		LdpUrlLists:            ldpurls,
		IPBlockLists:           ipblocklist,
		IPBlockIndex:           BuildIPBlockIndex(ipblocklist),
		IPBlockGroupCodes:      ExtractBlockGroupCodes(ipblocklist),
		IPLogExcludeIndex:      BuildIPLogExcludeIndex(inHost.EXCLUDE_IP_LOG),
		IPLogExcludeGroupCodes: ExtractIPLogExcludeGroupCodes(inHost.EXCLUDE_IP_LOG),
		UrlBlockLists:          urlblocklist,
		AntiCCBean:             anticcBean,
		HttpAuthBases:          httpAuthList,
		BlockingPage:           blockingPageMap,
		CacheRule:              cacheRuleList,
		TamperRules:            tamperRuleList,
		PathRules:              pathRuleList,
		StaticConfig:           staticCfg,
	}
	// 路由表(RCU)：在 writeMu 下克隆当前快照→在副本上登记本 host→原子发布。
	waf.withWriteTable(func(nt *routingTable) {
		// 原子替换：先清掉该 host(按旧 HostSafe 指针)此前占用的所有 key，再装新 key。
		// 全程在克隆表上操作、单次 Store 发布——主端口 key 不会出现"短暂缺失"，
		// 因此重载(改端口/SSL/绑定域名等)期间不会再返回 403 Host forbidden。
		if oldKey, ok := nt.HostCode[inHost.Code]; ok {
			if oldHS := nt.HostTarget[oldKey]; oldHS != nil {
				for k, v := range nt.HostTarget {
					if v == oldHS {
						delete(nt.HostTarget, k)
					}
				}
			}
		}
		for k, v := range nt.HostTargetMoreDomain {
			if v == inHost.Code {
				delete(nt.HostTargetMoreDomain, k)
			}
		}
		//目标关系情况
		nt.HostTarget[inHost.Host+":"+strconv.Itoa(inHost.Port)] = hostsafe
		//赋值到对照表里面
		nt.HostCode[inHost.Code] = inHost.Host + ":" + strconv.Itoa(inHost.Port)

		if len(ports) > 0 {
			for _, port := range ports {
				//目标关系情况
				nt.HostTarget[inHost.Host+":"+strconv.Itoa(port)] = hostsafe
				// 注意：HostCode[code] 始终指向主端口 key（已在上方赋值），不在此处覆盖
			}
		}

		//如果存在强制跳转
		if inHost.AutoJumpHTTPS == 1 {
			nt.HostTarget[inHost.Host+":80"] = hostsafe
			nt.HostCode[inHost.Code] = inHost.Host + ":80"
		}
		//如果是不限制端口的情况
		if inHost.UnrestrictedPort == 1 {
			zlog.Debug("来源端口宽松模式")
			nt.HostTargetNoPort[inHost.Host] = inHost.Host + ":" + strconv.Itoa(inHost.Port)
			// 多域名也注册到宽松端口映射
			if inHost.BindMoreHost != "" {
				for _, moreLine := range strings.Split(inHost.BindMoreHost, "\n") {
					moreLine = strings.TrimSpace(moreLine)
					if moreLine != "" {
						nt.HostTargetNoPort[moreLine] = inHost.Host + ":" + strconv.Itoa(inHost.Port)
					}
				}
			}
		} else {
			if _, ok := nt.HostTargetNoPort[inHost.Host]; ok {
				zlog.Debug("来源端口严苛模式")
				delete(nt.HostTargetNoPort, inHost.Host)
			}
			// 多域名也从宽松端口映射中移除
			if inHost.BindMoreHost != "" {
				for _, moreLine := range strings.Split(inHost.BindMoreHost, "\n") {
					moreLine = strings.TrimSpace(moreLine)
					if moreLine != "" {
						delete(nt.HostTargetNoPort, moreLine)
					}
				}
			}
		}
		//如果存在一个主机绑定了多个域名的情况
		if inHost.BindMoreHost != "" {
			lines := strings.Split(inHost.BindMoreHost, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// 主端口
				nt.HostTargetMoreDomain[line+":"+strconv.Itoa(inHost.Port)] = inHost.Code
				// 副端口（BindMorePort）也需要注册，否则副域名+副端口无法路由
				for _, extraPort := range ports {
					nt.HostTargetMoreDomain[line+":"+strconv.Itoa(extraPort)] = inHost.Code
				}
			}
		}
	})

	// 返回本站全部监听 runtime（含 AutoJumpHTTPS 隐式 80），供"新增站点"路径逐个启动
	var serverOnlines = []innerbean.ServerRunTime{}
	for _, listen := range listens {
		serverOnline, isExist := waf.ServerOnline.Get(listen.Port)
		if isExist {
			serverOnlines = append(serverOnlines, serverOnline)
		}
	}
	return serverOnlines
}

func normalizeIPV(ipv string) string {
	if ipv == "" {
		return utils.ListenIPVBoth
	}
	return ipv
}

// alertPortProtocolConflict 端口协议冲突发声：系统日志 + WebSocket 推送（issue #955 的静默点）。
// 引擎不因冲突拒绝启动，先到先得维持现状，由用户依提示处理。
func (waf *WafEngine) alertPortProtocolConflict(inHost model.Hosts, port int, wantProto, activeProto string) {
	msg := fmt.Sprintf("端口协议冲突：站点 %s 声明端口 %d 为 %s，但该端口当前按 %s 监听（先加载者生效）。若是多个网站对同一端口声明了不同协议，请统一协议或更换端口；若是您刚修改了本站该端口的协议，需重启引擎（或重启程序）后生效",
		inHost.Host, port, strings.ToUpper(wantProto), strings.ToUpper(activeProto))
	zlog.Warn(msg)
	global.GQEQUE_LOG_DB.Enqueue(&model.WafSysLog{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},
		OpType:    "系统运行错误",
		OpContent: msg,
	})
	global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.OpResultMessageInfo{
		BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "提示信息", Server: global.GWAF_CUSTOM_SERVER_NAME},
		Msg:             msg,
		Success:         "false",
	})
}

// alertPortIPVersionPending IP 版本变更未生效发声：已有监听不按新 ipv 自动重建，需重启引擎
func (waf *WafEngine) alertPortIPVersionPending(inHost model.Hosts, port int, wantIPV, activeIPV string) {
	msg := fmt.Sprintf("端口 %d 当前按 IP版本 %s 监听，站点 %s 本次声明为 %s：已有监听不会自动重建，需重启引擎（或重启程序）后生效",
		port, normalizeIPV(activeIPV), inHost.Host, normalizeIPV(wantIPV))
	zlog.Warn(msg)
	global.GQEQUE_LOG_DB.Enqueue(&model.WafSysLog{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},
		OpType:    "信息",
		OpContent: msg,
	})
	global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.OpResultMessageInfo{
		BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "提示信息", Server: global.GWAF_CUSTOM_SERVER_NAME},
		Msg:             msg,
		Success:         "false",
	})
}

// RemovePortServer 检测如果没有端口在占用了，可以关闭相应端口
func (waf *WafEngine) RemovePortServer() {
	// 使用Range方法安全地遍历ServerOnline
	portsToRemove := make([]int, 0)

	waf.ServerOnline.Range(func(onlinePort int, serverRuntime innerbean.ServerRunTime) bool {
		if waf_service.WafHostServiceApp.CheckAvailablePortExistApi(onlinePort) == 0 {
			//暂停服务 并 移除服务信息
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// 先停 HTTP/3：老代码只关了 Svr、从不碰 H3，删掉 map 条目后 UDP socket 就彻底泄漏了(issue #916)
			if serverRuntime.H3 != nil {
				waf.stopHTTP3(onlinePort, serverRuntime.H3)
			}
			if serverRuntime.Svr != nil {
				err := serverRuntime.Svr.Shutdown(ctx)
				if err != nil {
					zlog.Error("shutting down: " + err.Error())
				} else {
					zlog.Info("shutdown processed successfully port" + strconv.Itoa(onlinePort))
				}
			}
			// 记录需要删除的端口
			portsToRemove = append(portsToRemove, onlinePort)
		}
		return true // 继续遍历
	})

	// 删除已关闭的端口
	for _, port := range portsToRemove {
		waf.ServerOnline.Delete(port)
	}
}

// RemoveHost 移除主机相关信息
func (waf *WafEngine) RemoveHost(host model.Hosts) {

	// 移除当前信息：路由表(RCU)在 writeMu 下克隆→在副本上删除本 host 相关条目→原子发布。
	waf.withWriteTable(func(nt *routingTable) {
		//a.移除对照关系
		delete(nt.HostCode, host.Code)
		//b.移除主机保护信息：主端口/副端口/AutoJumpHTTPS 的 80 统一按监听表清理，避免残留 stale 指针
		delete(nt.HostTarget, host.Host+":"+strconv.Itoa(host.Port))
		for _, listen := range utils.ResolveHostListens(host) {
			delete(nt.HostTarget, host.Host+":"+strconv.Itoa(listen.Port))
		}
		//b4.移除 HostTargetNoPort 中的主域名和多域名条目
		delete(nt.HostTargetNoPort, host.Host)
		if host.BindMoreHost != "" {
			for _, moreLine := range strings.Split(host.BindMoreHost, "\n") {
				moreLine = strings.TrimSpace(moreLine)
				if moreLine != "" {
					delete(nt.HostTargetNoPort, moreLine)
				}
			}
		}
		//d.删除更多内容里面域名信息
		for moreHost, hostCode := range nt.HostTargetMoreDomain {
			if hostCode == host.Code {
				delete(nt.HostTargetMoreDomain, moreHost)
			}
		}
	})

	//c.移除某个端口下的证书数据
	waf.AllCertificate.RemoveSSL(host.Host)

	// 移除绑定的多个域名的证书
	if host.BindMoreHost != "" {
		lines := strings.Split(host.BindMoreHost, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				waf.AllCertificate.RemoveSSL(line)
			}
		}
	}

	// 清理与该主机及后端绑定的 Transport 缓存
	waf.purgeTransportForHost(host)
	//检测如果端口已经没有关联服务就直接关闭掉
	waf.RemovePortServer()
}

// ReLoadSensitive builds a complete immutable matcher before publication.
func (waf *WafEngine) ReLoadSensitive() {
	var rows []model.Sensitive
	if err := global.GWAF_LOCAL_DB.Find(&rows).Error; err != nil {
		zlog.Error("Sensitive configuration load failed")
		return
	}
	state := &sensitiveSnapshot{}
	var keywords [][]rune
	data := map[string]interface{}{}
	for _, row := range rows {
		if row.Content == "" {
			continue
		}
		keywords = append(keywords, []rune(row.Content))
		data[row.Content] = row
		state.in = state.in || row.CheckDirection == "in" || row.CheckDirection == "all"
		state.out = state.out || row.CheckDirection == "out" || row.CheckDirection == "all"
	}
	state.count = len(keywords)
	if len(keywords) > 0 {
		state.machine = new(goahocorasick.Machine)
		if err := state.machine.BuildByCustom(keywords, data); err != nil {
			zlog.Error("Sensitive configuration compilation failed")
			return
		}
	}
	waf.sensitive.Store(state)
}

type sensitiveSnapshot struct {
	machine *goahocorasick.Machine
	in, out bool
	count   int
}

func (waf *WafEngine) sensitiveCount() int {
	if state := waf.sensitive.Load(); state != nil {
		return state.count
	}
	return 0
}
func (waf *WafEngine) CheckRequestSensitive() bool {
	state := waf.sensitive.Load()
	return state != nil && state.in
}
func (waf *WafEngine) CheckResponseSensitive() bool {
	state := waf.sensitive.Load()
	return state != nil && state.out
}

// httpAuthResult 「网站密码访问」处理一次请求的结果。
//
// 状态码必须一并带出来：这道门拦下请求时响应是它自己写的，
// 调用方拿不到真实状态码，访问日志的状态码列就只能留空。
type httpAuthResult struct {
	Blocked   bool   // 请求被这道门接住，没有到达后端
	Tip       string // 写进响应体的提示文本
	Status    int    // 实际下发的 HTTP 状态码
	SelfServe bool   // 是认证网关自己的端点（登录页/登录校验），不是拦截
}

// DoHttpAuthBase Http auth base 检测
//
// clientIP 由调用方按站点「真实IP来源」解析后传入，与访问日志的 SRC_IP 同源。
// 认证链路自己再取一次连接 IP 的话，站点挂在 CDN／前置 Nginx 后面时会话列表里
// 全站都是同一个边缘节点地址，「绑定登录 IP」这条约束也会一并失效。
func (waf *WafEngine) DoHttpAuthBase(hostSafe *wafenginmodel.HostSafe, w http.ResponseWriter, r *http.Request, clientIP string) httpAuthResult {
	r = wafpipeline.WithTrustedExternalScheme(r, legacyRequestScheme(hostSafe.Host, r))
	cfg := model.DecodeHttpAuthConfig(hostSafe.Host.HttpAuthJSON)

	// 路径归一化：小写 + path.Clean。下面所有路径判定（登录页前缀、保护范围）都必须用 p，
	// 不能用 r.URL.Path —— 后者带着未消解的 . 与 ..，一个 /x/../admin 能绕过任何
	// 按前缀写的清单，而 Go 的 http.Server 对非 ServeMux 的 Handler 不做任何路径清洗。
	p := strings.ToLower(path.Clean(r.URL.Path))

	// ⓪ custom 模式的登录/校验/登出端点无条件视为在保护范围内：include 模式下这些
	// 前缀通常不在用户填的清单里，若按范围判定放行透传给后端就是 404，登录流程死锁。
	// 必须排在免密IP组与范围判定之前（自服务分支，参照 Access 模式 ⓶ 的设计）。
	// 与原实现一致只认 前缀+"/"：裸前缀精确访问维持走正常认证流程。
	if loginPrefix := httpAuthLoginPrefix(hostSafe.Host); loginPrefix != "" &&
		strings.HasPrefix(p, loginPrefix+"/") {
		res := waf.doCustomAuth(hostSafe, w, r, clientIP, cfg)
		// 认证网关自己的端点：请求由这道门应答，不是拦截。
		// 标出来是为了让调用方把它记成「放行」而不是「禁止」。
		res.SelfServe = true
		return res
	}

	// ① 免密IP组：命中的来源直接放行。组被删除时 GetGroupMatcher 返回 nil，
	//    ContainsStr 安全返回 false → 回落到要密码（fail-closed），删组不会敞开门。
	if cfg.BypassIPGroup != "" && ipset.GetGroupMatcher(cfg.BypassIPGroup).ContainsStr(clientIP) {
		return httpAuthResult{}
	}

	// ② 保护范围判定：范围外直接放行，不写日志也不记审计——
	//    这不是拦截，是这些路径本来就不归这道门管。
	if !httpAuthPathInScope(cfg, p) {
		return httpAuthResult{}
	}

	// ③ 空值(存量站点)与未知取值都按 Basic 走：开关已经开了，认不出类型就放行等于这道门形同虚设
	if hostSafe.Host.HttpAuthBaseType == model.HttpAuthTypeCustom {
		return waf.doCustomAuth(hostSafe, w, r, clientIP, cfg)
	}
	return waf.doBasicAuth(hostSafe, w, r, clientIP, cfg)
}

// httpAuthLoginPrefix custom 模式登录页前缀的归一化形态（小写、无尾斜杠）。
// 非 custom 模式返回空串，表示没有自服务路径。
func httpAuthLoginPrefix(host model.Hosts) string {
	if host.HttpAuthBaseType != model.HttpAuthTypeCustom {
		return ""
	}
	return strings.TrimRight(strings.ToLower(utils.GetHttpAuthPathOrDefault(host.HttpAuthPathPrefix)), "/")
}

// httpAuthRuleName 「网站密码访问」这道门在访问日志里的规则名。
//
// 这道门拦下请求时要在日志里留下规则名，与 Access 模式的「统一访问认证」同一口径：
// 访问日志的「触发规则」列据此可筛，动作列光有「禁止」是筛不出是哪道门的。
func httpAuthRuleName(host model.Hosts) string {
	if host.HttpAuthBaseType == model.HttpAuthTypeCustom {
		return "网站密码访问(自定义登录页)"
	}
	return "网站密码访问"
}

// httpAuthEmptyIncludeWarnAt include 模式空清单告警的节流时间戳（秒）。
// 这条告警发生在热路径上，不节流的话受保护站点的每个请求都会刷一行日志。
var httpAuthEmptyIncludeWarnAt atomic.Int64

// httpAuthPathInScope 判断已归一化的 p 是否落在「网站密码访问」的保护范围内。
func httpAuthPathInScope(cfg model.HttpAuthConfig, p string) bool {
	switch cfg.PathScope {
	case model.HttpAuthScopeInclude:
		patterns := accessgate.BuildPathPatterns(cfg.Paths)
		if len(patterns) == 0 {
			// fail-closed：选了「仅指定路径」却没填路径，按整站保护。
			// 静默当成"什么都不保护"等于一次配置失误就拆掉整道门。
			warnHttpAuthEmptyInclude()
			return true
		}
		return accessgate.MatchPathPatterns(p, patterns)
	case model.HttpAuthScopeExclude:
		patterns := accessgate.BuildPathPatterns(cfg.Paths)
		if len(patterns) == 0 {
			// 「除空集外全站要密码」的字面语义就是整站，正常特例，无需告警
			return true
		}
		return !accessgate.MatchPathPatterns(p, patterns)
	default:
		// all 与一切非法取值：整站（DecodeHttpAuthConfig 已把非法值洗成 all，这里兜底）
		return true
	}
}

func warnHttpAuthEmptyInclude() {
	now := time.Now().Unix()
	last := httpAuthEmptyIncludeWarnAt.Load()
	if now-last < 300 || !httpAuthEmptyIncludeWarnAt.CompareAndSwap(last, now) {
		return
	}
	zlog.Warn("网站密码访问：保护范围选了「仅指定路径」但路径清单为空，已按整站保护。" +
		"请到站点编辑里补全路径，或把保护范围改回整站")
}

// writeBasicChallenge 发 401 并要求浏览器输入密码。
//
// nonce 非空时 realm 会跟着变——浏览器按 realm 缓存凭证，realm 不变就只会拿旧凭证
// 自动重试，用户永远看不到弹窗。这是 Basic 模式下让「踢下线/到期」生效的唯一手段。
func (waf *WafEngine) writeBasicChallenge(w http.ResponseWriter, nonce, tip string) {
	realm := "Restricted"
	if nonce != "" {
		realm = "Restricted-" + nonce
	}
	w.Header().Set("WWW-Authenticate", "Basic realm=\""+realm+"\"")
	http.Error(w, tip, http.StatusUnauthorized)
}

// cutAuditName 截断用户名。用户名来自请求头或登录表单，是攻击者可控的任意长度字符串，
// 直接入库会撞 account_name 的列宽。
//
// 按字节切完还要过一次 ToValidUTF8：128 字节的边界可能落在一个多字节字符中间，
// 留下的半个字符会被 MySQL 的 utf8mb4 列拒收，那条审计记录就没了——
// 等于给了攻击者一个「用超长多字节用户名让自己的爆破不被记录」的口子。
func cutAuditName(name string) string {
	if len(name) <= 128 {
		return name
	}
	return strings.ToValidUTF8(name[:128], "")
}

// httpAuthAudit 组装一条网站密码访问的审计流水。
func (waf *WafEngine) httpAuthAudit(hostSafe *wafenginmodel.HostSafe, r *http.Request,
	clientIP, userName string) waf_service.AuditEntry {
	return waf_service.AuditEntry{
		AccountName: cutAuditName(userName),
		Host:        hostSafe.Host.Host,
		HostCode:    hostSafe.Host.Code,
		URL:         r.URL.Path, // 只记路径：查询串里可能带业务参数，审计表没有必要留
		ClientIP:    clientIP,
		UserAgent:   r.UserAgent(),
	}
}

// auditHttpAuthDenied 记一条「未登录被拦」。
// 这是本功能唯一的高频事件——一次目录扫描就是几千个未登录请求，必须走节流。
func (waf *WafEngine) auditHttpAuthDenied(hostSafe *wafenginmodel.HostSafe, r *http.Request, clientIP, msg string) {
	entry := waf.httpAuthAudit(hostSafe, r, clientIP, "")
	entry.Event = model.HttpAuthEventDenied
	entry.Result = model.AccessAuditFail
	entry.Message = msg
	waf_service.WafSecurityAuditServiceApp.WriteThrottled(entry)
}

// doBasicAuth 浏览器弹窗方式（HTTP Basic）
func (waf *WafEngine) doBasicAuth(hostSafe *wafenginmodel.HostSafe, w http.ResponseWriter, r *http.Request,
	clientIP string, cfg model.HttpAuthConfig) httpAuthResult {

	// 获取 Authorization 头部
	username, password, valid := r.BasicAuth()
	if !valid {
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "Invalid authorization format", http.StatusBadRequest)
			waf.auditHttpAuthDenied(hostSafe, r, clientIP, "Invalid authorization format")
			return httpAuthResult{Blocked: true, Tip: "Invalid authorization format", Status: http.StatusBadRequest}
		}
		tip := "Authentication required"
		waf.writeBasicChallenge(w, "", tip)
		waf.auditHttpAuthDenied(hostSafe, r, clientIP, tip)
		return httpAuthResult{Blocked: true, Tip: tip, Status: 401}
	}
	port := httpAuthPort(hostSafe.Host, r)
	hash, _ := legacyCredentialHash(hostSafe, username)
	if !legacyPasswordCache.Cached(hash, password) {
		if !legacyAllowAttempt(hostSafe.Host, r) || !legacyHashWork.Acquire(port, 4) {
			http.Error(w, "Please retry later", 429)
			return httpAuthResult{Blocked: true, Tip: "Login rate limited", Status: 429}
		}
		defer legacyHashWork.Release(port)
	}
	if !waf.checkCredentials(hostSafe, username, password) {
		tip := "密码错误"
		waf.writeBasicChallenge(w, "", tip)
		// Basic 模式没有登录表单，爆破就是一串带头的普通请求，逐条记会把审计表刷爆，走节流
		entry := waf.httpAuthAudit(hostSafe, r, clientIP, username)
		entry.Event = model.HttpAuthEventLoginFail
		entry.Result = model.AccessAuditFail
		entry.Message = "网站密码错误"
		waf_service.WafSecurityAuditServiceApp.WriteThrottled(entry)
		return httpAuthResult{Blocked: true, Tip: tip, Status: http.StatusUnauthorized}
	}

	// 凭证过关，接着做会话记账：到期或被踢时要换 realm 把浏览器重新逼回弹窗
	res := waf_service.WafHttpAuthSessionServiceApp.TouchBasicSession(hostSafe.Host, username,
		clientIP, r.UserAgent(), cfg, port)
	if res.Challenge {
		tip := "登录状态已失效，请重新输入密码"
		if res.Expired {
			tip = "登录已到期，请重新输入密码"
			entry := waf.httpAuthAudit(hostSafe, r, clientIP, username)
			entry.Event = model.HttpAuthEventExpired
			entry.Result = model.AccessAuditOK
			entry.Message = "网站密码会话到期，要求重新认证"
			waf_service.WafSecurityAuditServiceApp.WriteThrottled(entry)
		}
		waf.writeBasicChallenge(w, res.RealmNonce, tip)
		return httpAuthResult{Blocked: true, Tip: tip, Status: http.StatusUnauthorized}
	}

	// 只有真正建了会话行才算一次登录：Basic 每个请求都带凭证，逐个请求记就是成百上千条
	if res.Created {
		entry := waf.httpAuthAudit(hostSafe, r, clientIP, username)
		entry.Event = model.HttpAuthEventLoginOK
		entry.Result = model.AccessAuditOK
		entry.Message = "网站密码登录成功(浏览器弹窗方式)"
		if res.Session != nil {
			entry.SessionCode = res.Session.TokenCode
		}
		waf_service.WafSecurityAuditServiceApp.Write(entry)
	}
	return httpAuthResult{}
}

// doCustomAuth 自定义登录页方式
func (waf *WafEngine) doCustomAuth(hostSafe *wafenginmodel.HostSafe, w http.ResponseWriter, r *http.Request,
	clientIP string, cfg model.HttpAuthConfig) httpAuthResult {

	// 归一化后的登录页前缀：与 DoHttpAuthBase ⓪ 用同一份判定口径，
	// 大小写混淆、点号段变体进不了这个分支
	authPathPrefix := httpAuthLoginPrefix(hostSafe.Host)
	p := strings.ToLower(path.Clean(r.URL.Path))

	// 处理登录页面自身的请求（保持原口径：只认 前缀+"/"，裸前缀走正常认证流程）
	if strings.HasPrefix(p, authPathPrefix+"/") {
		code := waf.handleHttpAuthRequest(hostSafe, w, r, authPathPrefix, clientIP, cfg)
		return httpAuthResult{Blocked: true, Tip: "处理HTTP Auth请求", Status: code}
	}

	// 检查是否已经通过认证
	cookie, err := r.Cookie(httpAuthCookieName(hostSafe.Host, r))
	if err == nil && cookie.Value != "" {
		if _, ok := waf_service.WafHttpAuthSessionServiceApp.ValidateCustom(hostSafe.Host.Code,
			cookie.Value, clientIP, cfg, httpAuthPort(hostSafe.Host, r)); ok {
			return httpAuthResult{}
		}
	}

	// 未通过认证，显示登录页面
	tip := "需要登录认证"
	waf.auditHttpAuthDenied(hostSafe, r, clientIP, tip)
	code := waf.serveLoginPage(w, r, authPathPrefix)
	// 未认证时回的是登录页本身，状态码是 200 而不是 401：
	// 日志里记真实状态码，登录页能不能打开才看得出。
	return httpAuthResult{Blocked: true, Tip: tip, Status: code}
}

// handleHttpAuthRequest 处理HTTP Auth相关请求，返回实际下发的状态码。
func (waf *WafEngine) handleHttpAuthRequest(hostSafe *wafenginmodel.HostSafe, w http.ResponseWriter, r *http.Request,
	pathPrefix, clientIP string, cfg model.HttpAuthConfig) int {
	// 用归一化后的路径裁剪：进入本函数的请求已在 doCustomAuth 按归一化前缀判过，
	// 若按 r.URL.Path 裁剪，大小写或点号段变体会让 TrimPrefix 落空、validate 变 404
	path := strings.TrimPrefix(strings.ToLower(path.Clean(r.URL.Path)), pathPrefix+"/")

	// 处理验证接口
	if path == "validate" && r.Method == "POST" {
		return waf.handleHttpAuthValidate(hostSafe, w, r, clientIP, cfg)
	}

	// 处理静态文件（暂不需要，因为登录页面是独立的HTML）
	http.NotFound(w, r)
	return http.StatusNotFound
}

// handleHttpAuthValidate 处理登录验证，返回实际下发的状态码。
func (waf *WafEngine) handleHttpAuthValidate(hostSafe *wafenginmodel.HostSafe, w http.ResponseWriter, r *http.Request,
	clientIP string, cfg model.HttpAuthConfig) int {
	port := httpAuthPort(hostSafe.Host, r)
	if !legacyAllowAttempt(hostSafe.Host, r) {
		http.Error(w, "Please retry later", 429)
		return 429
	}
	if !legacyLoginOrigin(r) || !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		http.Error(w, "Request verification failed", 403)
		return 403
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer controller.SetReadDeadline(time.Time{})
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return 400
	}
	if !legacyHashWork.Acquire(port, 4) {
		http.Error(w, "Please retry later", 429)
		return 429
	}
	defer legacyHashWork.Release(port)
	if !waf.checkCredentials(hostSafe, req.Username, req.Password) {
		entry := waf.httpAuthAudit(hostSafe, r, clientIP, req.Username)
		entry.Event = model.HttpAuthEventLoginFail
		entry.Result = model.AccessAuditFail
		entry.Message = "Credential verification failed"
		waf_service.WafSecurityAuditServiceApp.WriteThrottled(entry)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"success":false,"message":"Invalid username or password"}`))
		return 401
	}
	authToken, sess, err := waf_service.WafHttpAuthSessionServiceApp.CreateCustomSession(hostSafe.Host,
		req.Username, clientIP, r.UserAgent(), cfg, port)
	if err != nil {
		zlog.Error("HTTP Auth建立会话失败", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"success": false, "message": "服务器错误"}`))
		return http.StatusInternalServerError
	}

	zlog.Info("HTTP Auth登录成功",
		zap.String("ip", clientIP),
		zap.String("username", req.Username))

	entry := waf.httpAuthAudit(hostSafe, r, clientIP, req.Username)
	entry.Event = model.HttpAuthEventLoginOK
	entry.Result = model.AccessAuditOK
	entry.SessionCode = sess.TokenCode
	entry.Message = "网站密码登录成功(自定义页面方式)"
	waf_service.WafSecurityAuditServiceApp.Write(entry)

	// 设置Cookie，有效期跟随站点配置的会话时长
	cookie := &http.Cookie{
		Name:     httpAuthCookieName(hostSafe.Host, r),
		Value:    authToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   wafpipeline.RequestScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cfg.SessionTTL) * 60,
	}
	http.SetCookie(w, cookie)

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true, "message": "登录成功", "redirect": "/"}`))
	return http.StatusOK
}

// serveLoginPage 提供登录页面，返回实际下发的状态码。
func (waf *WafEngine) serveLoginPage(w http.ResponseWriter, r *http.Request, pathPrefix string) int {
	// 读取登录页面文件
	loginPagePath := utils.GetCurrentDir() + "/data/httpauth/login.html"

	// 检查文件是否存在
	if _, err := os.Stat(loginPagePath); os.IsNotExist(err) {
		zlog.Warn("登录页面文件不存在", zap.String("path", loginPagePath))
		http.Error(w, "登录页面未配置", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}

	// 读取文件内容
	content, err := os.ReadFile(loginPagePath)
	if err != nil {
		zlog.Error("读取登录页面失败", zap.Error(err))
		http.Error(w, "服务器错误", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}

	// 替换路径前缀
	htmlStr := string(content)
	if pathPrefix == "" {
		pathPrefix = "/samwaf_httpauth"
	}
	htmlStr = strings.ReplaceAll(htmlStr, "/samwaf_httpauth/", pathPrefix+"/")
	htmlStr = strings.ReplaceAll(htmlStr, "'/samwaf_httpauth'", "'"+pathPrefix+"'")
	htmlStr = strings.ReplaceAll(htmlStr, "\"/samwaf_httpauth\"", "\""+pathPrefix+"\"")

	// 设置响应头
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlStr))
	return http.StatusOK
}

// checkCredentials 验证用户名和密码
func (waf *WafEngine) checkCredentials(hostSafe *wafenginmodel.HostSafe, username, password string) bool {
	hash, found := legacyCredentialHash(hostSafe, username)
	valid := legacyPasswordCache.Verify(hash, password)
	return found && valid
}

// purgeTransportForHost 清理指定主机相关的 TransportPool 键
func (waf *WafEngine) purgeTransportForHost(host model.Hosts) {
	// 构建需要匹配的 host:port 组合（主端口、隐式80、副端口、绑定域名），端口来源统一走监听表
	hostStrs := map[string]bool{
		fmt.Sprintf("%s:%d", host.Host, host.Port): true,
	}
	var morePorts []int
	for _, listen := range utils.ResolveHostListens(host) {
		hostStrs[fmt.Sprintf("%s:%d", host.Host, listen.Port)] = true
		if !listen.IsMain && !listen.Implied {
			morePorts = append(morePorts, listen.Port)
		}
	}
	// 绑定多域名
	if host.BindMoreHost != "" {
		lines := strings.Split(host.BindMoreHost, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// 绑定域名 + 主端口
			hostStrs[fmt.Sprintf("%s:%d", line, host.Port)] = true
			// 绑定域名 + 其他端口
			for _, p := range morePorts {
				hostStrs[fmt.Sprintf("%s:%d", line, p)] = true
			}
		}
	}

	// 扫描并删除匹配的 Transport 缓存
	waf.TransportMux.Lock()
	defer waf.TransportMux.Unlock()
	for key := range waf.TransportPool {
		parts := strings.Split(key, "_")
		if len(parts) != 5 {
			if t, ok := waf.TransportPool[key]; ok && t != nil {
				t.CloseIdleConnections()
			}
			delete(waf.TransportPool, key)
			continue
		}
		hostPart := parts[0]

		// 匹配 host：支持 hostPart 包含或不包含端口的情况
		matchHost := false
		// 1. 如果 hostPart 包含端口，直接匹配
		if _, exists := hostStrs[hostPart]; exists {
			matchHost = true
		} else {
			// 2. 如果 hostPart 不包含端口，检查 hostStrs 中是否有以 hostPart: 开头的项
			// 例如：hostPart = "example.com"，检查 hostStrs 中是否有 "example.com:80" 等
			for hostPortKey := range hostStrs {
				if strings.HasPrefix(hostPortKey, hostPart+":") {
					matchHost = true
					break
				}
			}
		}
		if matchHost {
			if t, ok := waf.TransportPool[key]; ok && t != nil {
				t.CloseIdleConnections()
			}
			delete(waf.TransportPool, key)
		}
	}
}
