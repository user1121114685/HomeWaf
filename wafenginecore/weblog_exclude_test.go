// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/wafenginecore/ipset"
	"testing"
)

// TestShouldRecordWebLogIPExclude D13 语义：排除 IP 只静音正常请求，安全事件照记。
func TestShouldRecordWebLogIPExclude(t *testing.T) {
	origin := global.CurrentSettings().GWAF_RUNTIME_RECORD_LOG_TYPE
	defer func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = origin })
	}()
	previous := globalIPLogExclude.Load()
	defer globalIPLogExclude.Store(previous)

	cases := []struct {
		name    string
		logType string
		weblog  *innerbean.WebLog
		exclude string
		want    bool
	}{
		{"all_排除IP的正常请求不记", "all",
			&innerbean.WebLog{ACTION: "放行", URL: "/a", SRC_IP: "203.0.113.7"},
			"203.0.113.7", false},
		{"all_排除IP的安全事件照记", "all",
			&innerbean.WebLog{ACTION: "阻止", RULE: "SQL注入", URL: "/a", SRC_IP: "203.0.113.7"},
			"203.0.113.7", true},
		{"abnormal_排除IP的安全事件照记", "abnormal",
			&innerbean.WebLog{ACTION: "阻止", RULE: "SQL注入", URL: "/a", SRC_IP: "203.0.113.7"},
			"203.0.113.7", true},
		{"all_未排除IP不受影响", "all",
			&innerbean.WebLog{ACTION: "放行", URL: "/a", SRC_IP: "198.51.100.1"},
			"203.0.113.7", true},
		{"all_排除网段命中", "all",
			&innerbean.WebLog{ACTION: "放行", URL: "/a", SRC_IP: "10.0.0.9"},
			"10.0.0.0/8", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = tc.logType })
			SetGlobalIPLogExclude(tc.exclude)
			if got := shouldRecordWebLog(tc.weblog); got != tc.want {
				t.Errorf("期望 %v 实际 %v", tc.want, got)
			}
		})
	}

	t.Run("all_排除清单里的IP组命中", func(t *testing.T) {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "all" })
		ipset.UpsertGroupMatcher("m6-test-grp", "测试组", ipset.BuildMatchSet([]string{"192.0.2.66"}))
		defer ipset.RemoveGroupMatcher("m6-test-grp")
		SetGlobalIPLogExclude("group:m6-test-grp")
		if got := shouldRecordWebLog(&innerbean.WebLog{ACTION: "放行", URL: "/a", SRC_IP: "192.0.2.66"}); got {
			t.Error("组内 IP 的正常请求应被排除")
		}
	})

}
