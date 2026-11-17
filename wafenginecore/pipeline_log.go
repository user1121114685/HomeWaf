package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RecordPipelineWebLog connects completed port requests to the existing access,
// security-event, statistics and export sinks. Pipeline traces remain separate:
// state changes and protocol diagnostics are not additional HTTP requests.
func (waf *WafEngine) RecordPipelineWebLog(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
	if global.GQEQUE_LOG_DB == nil || s == nil || s.Preview || c == nil || c.DryRun || c.Request == nil {
		return
	}
	r := c.Request
	started := c.Started
	if started.IsZero() {
		started = time.Now()
	}
	peerIP, peerPort, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerIP = r.RemoteAddr
	}
	day, _ := strconv.Atoi(started.Format("20060102"))
	lg := &innerbean.WebLog{
		REQ_UUID: c.ReqID, HOST_CODE: s.Config.HostCode,
		USER_CODE: global.GWAF_USER_CODE, TenantId: global.GWAF_TENANT_ID,
		HOST: wafpipeline.RequestScheme(r) + "://" + r.Host,
		URL:  r.URL.RequestURI(), RawQuery: r.URL.RawQuery, METHOD: r.Method,
		REFERER: r.Referer(), USER_AGENT: r.UserAgent(), Scheme: r.Proto,
		SRC_IP: c.ClientIP.String(), SRC_PORT: peerPort, NetSrcIp: peerIP,
		CREATE_TIME: started.Format("2006-01-02 15:04:05"), UNIX_ADD_TIME: started.UnixMilli(), Day: day,
		CONTENT_LENGTH: r.ContentLength, TimeSpent: time.Since(started).Milliseconds(),
		ACTION: "放行", STATUS: "正常访问", STATUS_CODE: result.Verdict.Status,
		GUEST_IDENTIFICATION: "未识别访客", TASK_FLAG: -1, WebLogVersion: global.GWEBLOG_VERSION,
	}
	if lg.HOST_CODE == "" {
		lg.HOST_CODE = "pipeline-" + strconv.Itoa(int(s.Config.Port))
	}
	switch result.Verdict.Action {
	case wafpipeline.Allow, wafpipeline.Continue, wafpipeline.Bypass:
	default:
		lg.ACTION, lg.STATUS = "阻止", "阻止访问"
		lg.RULE = result.Verdict.RuleID
		if lg.RULE == "" {
			lg.RULE = result.Verdict.Reason
		}
	}
	// A final upstream allow must not erase observe-only detector matches. Login
	// success signals, however, are ordinary traffic rather than security events.
	for _, hit := range result.Hits {
		if hit.Category == "auth_success" || hit.Category == "business_auth_success" {
			continue
		}
		if lg.RULE == "" {
			lg.RULE = hit.RuleID
			if lg.RULE == "" {
				lg.RULE = hit.Category
			}
		}
		lg.RISK_LEVEL = max(lg.RISK_LEVEL, hit.Severity.Rank())
		if lg.ACTION == "放行" && (hit.Observe || hit.Trusted) {
			lg.LogOnlyMode = 1
		}
	}
	// Resolve exclusions by the configured site identity, never an untrusted Host.
	host := &wafenginmodel.HostSafe{}
	if waf != nil {
		table := waf.rt()
		if existing := table.HostTarget[table.HostCode[lg.HOST_CODE]]; existing != nil {
			host = existing
		}
	}
	if !shouldRecordWebLog(lg, host) {
		return
	}
	region, resolved := utils.GetCountryEx(lg.SRC_IP)
	lg.COUNTRY, lg.PROVINCE, lg.CITY, lg.GeoUnresolved = region[0], region[2], region[3], !resolved
	if verified, _ := c.Values["bot_verified"].(bool); verified {
		lg.IsBot, lg.BotVerifyStrong = 1, 1
		lg.GUEST_IDENTIFICATION = "已验证爬虫"
	}
	var headers strings.Builder
	_ = r.Header.Write(&headers)
	lg.HEADER = headers.String()
	lg.COOKIES = r.Header.Get("Cookie")
	// Only reuse the already inspected, bounded body. Logging must never consume
	// the request stream or interfere with uploads and WebSocket forwarding.
	if c.View != nil {
		limit := min(len(c.View.Body), int(max(0, global.CurrentSettings().GCONFIG_RECORD_MAX_BODY_LENGTH)))
		lg.BODY = string(c.View.Body[:limit])
		lg.BodyTruncated = c.View.BodyTruncated || limit < len(c.View.Body)
	}
	if w, ok := c.Values["response_writer"].(http.ResponseWriter); ok {
		headers.Reset()
		_ = w.Header().Write(&headers)
		lg.ResHeader = headers.String()
		lg.RES_CONTENT_LENGTH, _ = strconv.ParseInt(w.Header().Get("Content-Length"), 10, 64)
	}
	for _, entry := range result.Trace {
		if entry.Stage == 5 {
			lg.ForwardCost += entry.ElapsedMicros / 1000
		} else {
			lg.PreCheckCost += entry.ElapsedMicros / 1000
		}
	}
	// Queue admission makes an independent, redacted copy; the existing consumer
	// applies persistence, access sampling and retention settings unchanged.
	global.GQEQUE_LOG_DB.Enqueue(lg)
}
