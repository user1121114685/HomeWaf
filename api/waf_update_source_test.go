package api

import (
	"HomeWaf/global"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHomeWafUpdateSourceResponses(t *testing.T) {
	oldManifest, oldRelease, oldBeta := global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL
	oldBetaCheck, oldUpdating, oldVersion, oldWin7 := global.CurrentSettings().GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_IS_UPDATETING, global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_WIN7_VERSION
	t.Cleanup(func() {
		global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL = oldManifest, oldRelease, oldBeta
		global.UpdateSettings(func(cfg *global.RuntimeSettings) {
			cfg.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_IS_UPDATETING, global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_WIN7_VERSION = oldBetaCheck, oldUpdating, oldVersion, oldWin7
		})
	})
	global.GUPDATE_VERSION_URL, global.GUPDATE_GITHUB_VERSION_URL = "", ""
	global.UpdateSettings(func(cfg *global.RuntimeSettings) {
		cfg.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_IS_UPDATETING = 0, false
	})
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

func TestHomeWafUpdateSourceGroupedNotes(t *testing.T) {
	oldManifest, oldRelease, oldBeta := global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL
	oldVersion, oldUpdating := global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_IS_UPDATETING
	t.Cleanup(func() {
		global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL = oldManifest, oldRelease, oldBeta
		global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_IS_UPDATETING = oldVersion, oldUpdating
	})
	global.GUPDATE_VERSION_URL, global.GUPDATE_GITHUB_VERSION_URL = "", ""
	global.GWAF_RELEASE_VERSION, global.GWAF_RUNTIME_IS_UPDATETING = "v2.0.1", false
	platform := map[string]string{"windows-amd64": "Windows_x86_64", "linux-amd64": "Linux_x86_64", "linux-arm64": "Linux_arm64", "darwin-amd64": "Darwin_x86_64", "darwin-arm64": "Darwin_arm64"}[runtime.GOOS+"-"+runtime.GOARCH]
	if platform == "" {
		t.Skip("unsupported release platform")
	}
	extension := ".tar.gz"
	if runtime.GOOS == "windows" {
		extension = ".zip"
	}
	asset := "HomeWaf_" + platform + "_2.0.3" + extension
	historyFails := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "v2.0.3", "body": "* " + strings.Repeat("a", 40) + " fix: 修复升级按钮", "assets": []map[string]any{{"name": asset, "browser_download_url": "https://github.com/example/HomeWaf/releases/download/v2.0.3/" + asset, "digest": "sha256:" + strings.Repeat("0", 64)}}})
			return
		}
		if historyFails {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{{"tag_name": "v2.0.2", "body": "改进证书管理"}, {"tag_name": "v2.0.1", "body": "已安装版本"}})
	}))
	defer server.Close()
	global.GUPDATE_RELEASE_URL = server.URL + "/releases/latest"
	for _, fails := range []bool{false, true} {
		historyFails = fails
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("GET", "/api/v1/sysinfo/checkversion", nil)
		ctx.Set("is_openapi", true)
		new(WafSysInfoApi).CheckVersionApi(ctx)
		var response struct {
			Code int `json:"code"`
			Data struct {
				NeedUpdate bool `json:"need_update"`
				Notes      []struct {
					Version string   `json:"version"`
					Items   []string `json:"items"`
				} `json:"version_notes"`
				Warning string `json:"version_notes_warning"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code != 0 || !response.Data.NeedUpdate || len(response.Data.Notes) == 0 || response.Data.Notes[0].Version != "v2.0.3" || response.Data.Notes[0].Items[0] != "修复升级按钮" {
			t.Fatalf("invalid update response: %s", recorder.Body.String())
		}
		if fails {
			if len(response.Data.Notes) != 1 || response.Data.Warning == "" {
				t.Fatal("history failure was hidden")
			}
		} else if len(response.Data.Notes) != 2 || response.Data.Notes[1].Version != "v2.0.2" || response.Data.Warning != "" {
			t.Fatal("cross-version release notes are incomplete")
		}
	}
}
