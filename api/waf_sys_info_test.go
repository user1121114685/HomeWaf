// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package api

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafsec"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 响应体：{"message":"H       ello, World!"}，实际响应体：{"code":0,"data":{"need_update":false,"versi       on":"555","version_name":"1.0","version_release":"false"},"msg":" 已经是最新版本"}
type WafCheckVersionResponse struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}
type ComplexData struct {
	NeedUpdate     bool   `json:"need_update"`
	Version        string `json:"version"`
	VersionName    string `json:"version_name"`
	VersionRelease string `json:"version_release"`
}

// 测试用例 SysRuntimeInfoApi：系统信息探测必须永远有返回，不能因为取不到某项而失败
func TestSysRuntimeInfoApi(t *testing.T) {
	r := gin.Default()
	oldVersion, oldName := global.GWAF_RELEASE_VERSION, global.GWAF_RELEASE_VERSION_NAME
	t.Cleanup(func() { global.GWAF_RELEASE_VERSION, global.GWAF_RELEASE_VERSION_NAME = oldVersion, oldName })
	global.GWAF_RELEASE_VERSION = "v1.0.0"
	global.GWAF_RELEASE_VERSION_NAME = "20241028"
	r.GET("/api/v1/sysinfo/runtimeinfo", new(WafSysInfoApi).SysRuntimeInfoApi)

	req, err := http.NewRequest(http.MethodGet, "/api/v1/sysinfo/runtimeinfo", nil)
	if err != nil {
		t.Fatalf("无法创建请求：%v", err)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	t.Logf("响应体内容: %s", rec.Body.String())

	if rec.Code != http.StatusOK {
		t.Errorf("期望的状态码：%d，实际状态码：%d", http.StatusOK, rec.Code)
	}
	var response WafCheckVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("响应解析失败：%v", err)
	}
	if response.Code != 0 {
		t.Errorf("期望的响应码：0，实际：%v", response.Code)
	}
	// 响应体默认经通讯密钥加密，先解密再解析
	payload := response.Data
	var encrypted string
	if json.Unmarshal(response.Data, &encrypted) == nil {
		decrypted, err := wafsec.AesDecrypt(encrypted, global.GWAF_COMMUNICATION_KEY)
		if err != nil {
			t.Fatalf("响应解密失败：%v", err)
		}
		payload = decrypted
	}
	var info model.RuntimeSystemInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("系统信息解析失败：%v", err)
	}
	if info.OS == "" || info.Arch == "" || info.GoVersion == "" {
		t.Errorf("编译相关信息不应为空：%+v", info)
	}
	if info.OSName == "" {
		t.Error("操作系统名称至少应有兜底值")
	}
	if info.Version != "v1.0.0" || info.VersionName != "20241028" {
		t.Errorf("软件版本信息不正确：%+v", info)
	}
	if info.ProcessUptimeSeconds < 0 {
		t.Errorf("程序运行时长不应为负：%v", info.ProcessUptimeSeconds)
	}
}

// Each test owns its update source and restores every mutated global.
func isolateVersionCheck(t *testing.T, handler http.HandlerFunc) *gin.Engine {
	t.Helper()
	oldURL, oldRelease, oldBetaURL := global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL
	oldVersion, oldName, oldNew, oldDesc := global.GWAF_RELEASE_VERSION, global.GWAF_RELEASE_VERSION_NAME, global.GWAF_RUNTIME_NEW_VERSION, global.GWAF_RUNTIME_NEW_VERSION_DESC
	oldBeta, oldWin7, oldUpdating := global.CurrentSettings().GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_WIN7_VERSION, global.GWAF_RUNTIME_IS_UPDATETING
	t.Cleanup(func() {
		global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL = oldURL, oldRelease, oldBetaURL
		global.GWAF_RELEASE_VERSION, global.GWAF_RELEASE_VERSION_NAME, global.GWAF_RUNTIME_NEW_VERSION, global.GWAF_RUNTIME_NEW_VERSION_DESC = oldVersion, oldName, oldNew, oldDesc
		global.UpdateSettings(func(cfg *global.RuntimeSettings) {
			cfg.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_WIN7_VERSION, global.GWAF_RUNTIME_IS_UPDATETING = oldBeta, oldWin7, oldUpdating
		})
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	global.GUPDATE_VERSION_URL = server.URL + "/"
	global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL = "", ""
	global.UpdateSettings(func(cfg *global.RuntimeSettings) {
		cfg.GCONFIG_CHECK_BETA_VERSION, global.GWAF_RUNTIME_WIN7_VERSION, global.GWAF_RUNTIME_IS_UPDATETING = 0, "false", false
	})
	global.GWAF_RELEASE_VERSION, global.GWAF_RELEASE_VERSION_NAME = "v1.0.6", "fixture"
	r := gin.New()
	r.GET("/check", func(c *gin.Context) { c.Set("is_openapi", true); new(WafSysInfoApi).CheckVersionApi(c) })
	return r
}
func checkVersionFixture(t *testing.T, r *gin.Engine) WafCheckVersionResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/check", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status %d", rec.Code)
	}
	var out WafCheckVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestCheckVersionApi(t *testing.T) {
	r := isolateVersionCheck(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("invalid manifest")) })
	if out := checkVersionFixture(t, r); out.Code != -1 {
		t.Fatalf("invalid manifest must fail: %+v", out)
	}
}
func TestCheckVersionNeedUploadApi(t *testing.T) {
	for _, tc := range []struct {
		version   string
		available bool
	}{{"v1.0.6", false}, {"v1.0.7", true}} {
		t.Run(tc.version, func(t *testing.T) {
			calls := 0
			r := isolateVersionCheck(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				_ = json.NewEncoder(w).Encode(map[string]any{"Version": tc.version, "Sha256": make([]byte, 32), "Desc": "fixture release"})
			})
			global.GWAF_RUNTIME_WIN7_VERSION = "true"
			out := checkVersionFixture(t, r)
			if out.Code != 0 || calls != 1 {
				t.Fatalf("check failed: %+v calls=%d", out, calls)
			}
			var data model.VersionInfo
			if err := json.Unmarshal(out.Data, &data); err != nil {
				t.Fatal(err)
			}
			if data.NeedUpdate != tc.available {
				t.Fatalf("NeedUpdate=%v want %v", data.NeedUpdate, tc.available)
			}
			if tc.available && data.VersionNew != tc.version {
				t.Fatalf("new version %q", data.VersionNew)
			}
		})
	}
}
