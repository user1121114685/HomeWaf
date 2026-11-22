package waf_service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/model/request"
)

func TestNotifyEmailRecordsEffectiveRecipients(t *testing.T) {
	channel := model.NotifyChannel{Type: "email", ConfigJSON: `{"smtp_host":"127.0.0.1","smtp_port":"1","from_email":"waf@example.test","to_emails":["default@example.test"]}`}
	for _, tc := range []struct{ recipients, want string }{
		{"", "default@example.test"},
		{" , ", "default@example.test"},
		{" one@example.test, two@example.test ", "one@example.test, two@example.test"},
	} {
		sub := model.NotifySubscription{Recipients: tc.recipients}
		// An invalid subject stops delivery before SMTP, while preserving the recipients for the log.
		got, err := WafNotifySenderServiceApp.deliverToChannel(channel, sub, NotifyEvent{}, "invalid\nsubject", "")
		if err == nil || got != tc.want {
			t.Fatalf("recipients %q: want %q, got %q (error %v)", tc.recipients, tc.want, got, err)
		}
	}
}

func TestNotifyEveryCategoryDeliversAndRecordsFullText(t *testing.T) {
	_, db, _ := setupLogLookupTest(t)
	if err := db.AutoMigrate(&model.NotifyLog{}); err != nil {
		t.Fatal(err)
	}
	oldHosts := global.CurrentSettings().GCONFIG_OUTBOUND_ALLOWED_HOSTS
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	t.Cleanup(func() {
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = oldHosts })
	})
	received := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		received <- payload
		fmt.Fprint(w, `{"code":200}`)
	}))
	defer srv.Close()
	cfg, _ := json.Marshal(map[string]string{"server": srv.URL + "/test"})
	ch := model.NotifyChannel{Type: "bark", ConfigJSON: string(cfg)}
	ch.Id = "test-channel"
	checks := map[string][]string{
		model.MSG_TYPE_RULE_TRIGGER:          {"处理结果", "请求 ID", "发生时间"},
		model.MSG_TYPE_ATTACK_INFO:           {"攻击类型", "URL", "攻击IP", "攻击时间"},
		model.MSG_TYPE_IP_BAN:                {"封禁范围", "封禁时长", "剩余时间", "封禁时间"},
		model.MSG_TYPE_OPERATION_NOTICE:      {"操作类型", "服务器", "操作内容", "发生时间"},
		model.MSG_TYPE_SYSTEM_ERROR:          {"错误类型", "错误信息", "发生时间"},
		model.MSG_TYPE_SOFTWARE_UPDATE:       {"已发布", "可更新到"},
		model.MSG_TYPE_USER_LOGIN:            {"用户", "IP地址", "登录时间"},
		model.MSG_TYPE_MANAGE_LOGIN_ABNORMAL: {"本次登录时间", "上次登录IP", "上次归属地", "上次登录时间"},
		model.MSG_TYPE_WEEKLY_REPORT:         {"周期", "总请求数", "拦截请求数", "拦截率"},
		model.MSG_TYPE_SSL_EXPIRE:            {"过期时间", "有效期状态", "检测地址", "处理提示"},
		model.MSG_TYPE_ACCESS_LOGIN:          {"登录成功", "访问地址", "来源IP", "时间"},
		model.MSG_TYPE_ACCESS_ABNORMAL:       {"回跳地址异常", "访问地址", "来源IP", "时间"},
	}
	if len(checks) != len(AllMessageTypes()) {
		t.Fatal("notification category coverage drifted")
	}
	for _, typ := range AllMessageTypes() {
		t.Run(typ, func(t *testing.T) {
			ev := SampleNotifyEvent(typ)
			sub := model.NotifySubscription{MessageType: typ, ChannelId: ch.Id}
			sub.Id = "test-" + typ
			WafNotifySenderServiceApp.DispatchEvents(sub, ch, []NotifyEvent{ev}, sub.Id, 3)
			list, n, err := WafNotifyLogServiceApp.GetListApi(request.WafNotifyLogSearchReq{MessageType: typ, PageIndex: 1, PageSize: 10})
			if err != nil || n != 1 || len(list) != 1 || list[0].Status != 1 {
				t.Fatalf("delivery log: %+v, %d, %v", list, n, err)
			}
			packet := <-received
			row := list[0]
			if row.MessageContent != packet["body"] || row.MessageTitle != packet["title"] || row.SubscriptionId != sub.Id || row.TemplateUsed != "default" {
				t.Fatalf("payload and log differ: %+v", row)
			}
			for _, want := range checks[typ] {
				if !strings.Contains(row.MessageTitle+row.MessageContent, want) {
					t.Fatalf("missing %s in %s", want, row.MessageContent)
				}
			}
			if strings.Contains(row.MessageContent, ":**") || strings.Contains(row.MessageContent, "<no value>") {
				t.Fatal("malformed plain text")
			}
			sub.ContentTemplate = "{{.Time}} / {{.MessageTypeName}}"
			_, body, used := RenderNotifyMessage(sub, "bark", ev)
			if used != "custom" || body != ev.Vars["Time"]+" / "+GetMessageTypeName(typ) {
				t.Fatalf("custom template: %q", body)
			}
		})
	}
	failed := 0
	if err := WafNotifyLogServiceApp.AddLog(ch.Id, "fixture", "bark", "system_error", "failed", "body", "", 0, "test error"); err != nil {
		t.Fatal(err)
	}
	list, n, err := WafNotifyLogServiceApp.GetListApi(request.WafNotifyLogSearchReq{Status: &failed, PageIndex: 1, PageSize: 20})
	if err != nil || n != 1 || len(list) != 1 || list[0].Status != 0 {
		t.Fatalf("failed filter %d %v", n, err)
	}
	if err := db.Migrator().DropTable(&model.NotifyLog{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WafNotifyLogServiceApp.GetListApi(request.WafNotifyLogSearchReq{}); err == nil {
		t.Fatal("read error reported as empty history")
	}
}

func TestNotifyBoundaryValuesAndCertificateRetryMetadata(t *testing.T) {
	s := WafNotifySenderServiceApp
	login := s.BuildNotifyEvent(innerbean.UserLoginMessageInfo{Ip: "192.0.2.7", Location: "内网"})
	if !matchIPList([]string{"192.0.2.0/24"}, login.DedupParts[model.DedupKeyIp]) || !strings.Contains(login.DefaultContent, "192.0.2.7 (内网)") {
		t.Fatal("login location broke IP filters")
	}
	if !matchIPList([]string{"2001:db8::/32"}, "2001:db8:1::/64") || matchIPList([]string{"2001:db8:1::1"}, "2001:db8:1::/64") {
		t.Fatal("IP ban scope was partially excluded")
	}
	for _, tc := range []struct {
		msg  any
		want string
	}{
		{innerbean.WeeklyReportMessageInfo{WeekRange: "本周"}, "0.00%"},
		{innerbean.IPBanMessageInfo{Permanent: true}, "剩余时间:** 永久"},
		{innerbean.IPBanMessageInfo{DurationSeconds: 15, RemainingSeconds: 0}, "封禁时长:** 15秒"},
		{innerbean.SSLExpireMessageInfo{DaysLeft: -1}, "已过期"},
		{innerbean.SSLExpireMessageInfo{DaysLeft: 0}, "24 小时内"},
	} {
		ev := s.BuildNotifyEvent(tc.msg)
		if !strings.Contains(ev.DefaultContent, tc.want) {
			t.Fatal(ev.DefaultContent)
		}
	}
	at := time.Date(2026, 11, 22, 1, 0, 0, 0, time.UTC)
	expires := at.Add(-time.Hour)
	e := model.CertificateEvent{Kind: "expiry", Domain: "nas.test", Severity: "critical", ExpiresAt: customtype.JsonTime(expires)}
	e.CREATE_TIME = customtype.JsonTime(at)
	for _, legacy := range []bool{false, true} {
		if legacy {
			e.ExpiresAt = customtype.JsonTime{}
			e.Message = "有效期至：" + expires.Format(time.RFC3339)
		}
		ev := certificateNotifyEvent(e)
		env := buildWebhookMessage(ev, "title", "content")
		if ev.Vars["DaysLeft"] != "-1" || ev.Vars["ExpireTime"] != expires.Format(time.RFC3339) || env.Time != at.Format(time.RFC3339) || env.Severity != "critical" {
			t.Fatalf("incorrect snapshot: %+v", ev)
		}
	}
}

func TestNotifySubscriptionDeliversNewProviderWithTemplate(t *testing.T) {
	old := global.CurrentSettings().GCONFIG_OUTBOUND_ALLOWED_HOSTS
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	t.Cleanup(func() {
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = old })
	})
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"code":200}`)
	}))
	defer server.Close()
	config, _ := json.Marshal(map[string]interface{}{"server": server.URL + "/device-key"})
	channel := model.NotifyChannel{Type: "bark", ConfigJSON: string(config)}
	sub := model.NotifySubscription{MessageType: model.MSG_TYPE_SYSTEM_ERROR, TitleTemplate: "告警：{{.ErrorType}}", ContentTemplate: "{{.ErrorMsg}}"}
	event := WafNotifySenderServiceApp.BuildNotifyEvent(innerbean.SystemErrorMessageInfo{ErrorType: "pipeline_lockdown", ErrorMsg: "[blocked] a_b *detail* \"quoted\""})
	title, content, used := RenderNotifyMessage(sub, channel.Type, event)
	if used != model.TemplateUsedCustom {
		t.Fatal("custom template not used")
	}
	if _, err := WafNotifySenderServiceApp.deliverToChannel(channel, sub, event, title, content); err != nil {
		t.Fatal(err)
	}
	if received["title"] != "告警：pipeline_lockdown" || received["body"] != "[blocked] a_b *detail* \"quoted\"" {
		t.Fatalf("template text was corrupted: %#v", received)
	}
}
