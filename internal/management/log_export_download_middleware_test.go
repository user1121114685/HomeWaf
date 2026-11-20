// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package management

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/storage/cache"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLogExportDownloadAcceptsReplayQueryWithoutManagementKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := cache.NewCacheStore("memory", nil)
	if err != nil {
		t.Fatal(err)
	}
	oldCache, oldReplay := global.GCACHE_WAFCACHE, global.CurrentSettings().GCONFIG_ENABLE_REPLAY_PROTECT
	oldIP, oldDomain := global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST
	defer func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) {
			global.GCACHE_WAFCACHE, cfg.GCONFIG_ENABLE_REPLAY_PROTECT = oldCache, oldReplay
		})
		global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST = oldIP, oldDomain
	}()
	global.GCACHE_WAFCACHE = store
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_ENABLE_REPLAY_PROTECT = 1 })
	global.GWAF_IP_WHITELIST = "0.0.0.0/0,::/0"
	global.GWAF_DOMAIN_WHITELIST = ""

	r := gin.New()
	(&WafWebManager{}).RegisterRoutes(r)

	now := strconv.FormatInt(time.Now().Unix(), 10)
	url := "/api/v1/waflog/exports/task-1/download?ticket=ticket-value&X-Request-Time=" + now + "&X-Request-Id=nonce-download-123456"
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, url, nil))
	var validation struct {
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &validation); err != nil || recorder.Code != http.StatusBadRequest || validation.ErrorCode != "EXPORT_INVALID_REQUEST" {
		t.Fatalf("ticket request must reach the download handler's validation: HTTP %d %s", recorder.Code, recorder.Body.String())
	}

	// 同一 nonce 再次请求应被防重放拒绝，且不能进入下载处理器。
	recorder = httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, url, nil))
	var replay struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &replay); err != nil || replay.Code != -1 || replay.Msg != "重复请求，已拒绝" {
		t.Fatalf("重复 nonce 不应进入下载处理器: %s", recorder.Body.String())
	}
}
