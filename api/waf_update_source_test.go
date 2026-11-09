package api

import (
	"HomeWaf/global"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHomeWafUpdateSourceResponses(t *testing.T) {
	oldManifest, oldRelease, oldBeta := global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL
	oldBetaCheck, oldUpdating, oldVersion, oldWin7 := global.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_IS_UPDATETING, global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_WIN7_VERSION
	t.Cleanup(func() {
		global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL = oldManifest, oldRelease, oldBeta
		global.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_IS_UPDATETING, global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_WIN7_VERSION = oldBetaCheck, oldUpdating, oldVersion, oldWin7
	})
	global.GUPDATE_VERSION_URL, global.GUPDATE_GITHUB_VERSION_URL = "", ""
	global.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_IS_UPDATETING = 0, false
	global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_WIN7_VERSION = "v2.0.0", "false"
	for _, status := range []int{404, 403, 500} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		global.GUPDATE_RELEASE_URL = s.URL
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest("GET", "/api/v1/sysinfo/checkversion", nil)
		ctx.Set("is_openapi", true)
		new(WafSysInfoApi).CheckVersionApi(ctx)
		s.Close()
		var result WafCheckVersionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(result.Msg, "未配置") {
			t.Fatalf("wrong missing-source message: %s", result.Msg)
		}
		if status == 404 {
			var data ComplexData
			if err := json.Unmarshal(result.Data, &data); err != nil {
				t.Fatal(err)
			}
			if result.Code != 0 || data.NeedUpdate || !strings.Contains(result.Msg, "尚未发布") {
				t.Fatalf("wrong empty release response: %s", rec.Body.String())
			}
		} else if result.Code == 0 || !strings.Contains(result.Msg, "失败") {
			t.Fatalf("network error hidden: %s", rec.Body.String())
		}
	}
}
