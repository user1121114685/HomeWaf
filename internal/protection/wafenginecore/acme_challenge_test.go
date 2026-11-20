// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/platform/common/queue"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/services/wafacme"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testChallengeToken = "q1IHnW37smIYQO7piKBIhI7GbxI9BKHPmPbH45_pai4"
const testKeyAuth = "q1IHnW37smIYQO7piKBIhI7GbxI9BKHPmPbH45_pai4.Zm9vYmFyYmF6cXV4"

// TestMain 补齐引擎在真实进程里由启动流程完成的初始化：
// 日志器、以及 weblog 入库队列（命中放行时会往里 Enqueue）。
func TestMain(m *testing.M) {
	global.GSSL_HTTP_CHANGLE_PATH = "/.well-known/acme-challenge/"
	zlog.InitZLog(false, "console")
	if global.GQEQUE_LOG_DB == nil {
		global.GQEQUE_LOG_DB = queue.NewQueue()
	}
	os.Exit(m.Run())
}

func TestACMEChallengeReadsOnlyDuringActiveIssuance(t *testing.T) {
	const service = "pipeline-80"
	wafacme.SetDataDir(t.TempDir())
	wafacme.ResetForTest()
	t.Cleanup(wafacme.ResetForTest)
	previous := challengeFileOpen
	t.Cleanup(func() { challengeFileOpen = previous })
	opens := 0
	file := filepath.Join(t.TempDir(), testChallengeToken)
	if err := os.WriteFile(file, []byte(testKeyAuth), 0600); err != nil {
		t.Fatal(err)
	}
	challengeFileOpen = func(name string) (*os.File, error) {
		opens++
		if !strings.HasSuffix(filepath.ToSlash(name), "/data/vhost/pipeline-80/.well-known/acme-challenge/"+testChallengeToken) {
			return nil, os.ErrNotExist
		}
		return os.Open(file)
	}
	if _, found := loadChallengeContent(service, testChallengeToken); found || opens != 0 {
		t.Fatal("closed gate opened a challenge file")
	}
	wafacme.Present(service, testChallengeToken, testKeyAuth)
	if value, found := loadChallengeContent(service, testChallengeToken); !found || value != testKeyAuth || opens != 0 {
		t.Fatalf("registered challenge returned %q after %d file opens", value, opens)
	}
	wafacme.CleanUp(service, testChallengeToken)
	if value, found := loadChallengeContent(service, testChallengeToken); !found || value != testKeyAuth || opens != 1 {
		t.Fatalf("active gate did not serve the other worker's challenge: %q, %d file opens", value, opens)
	}
	if _, found := loadChallengeContent("other-service", testChallengeToken); found {
		t.Fatal("read another service's challenge file")
	}
	for i := 0; i < 500; i++ {
		loadChallengeContent(service, "absent")
	}
	if opens >= 500 {
		t.Fatalf("missing challenge caused %d file opens", opens)
	}
}

func TestACMEChallengeRequiresExactRequest(t *testing.T) {
	const target = "http://family.test/.well-known/acme-challenge/"
	for _, tc := range []struct {
		method, url, upgrade string
		want                 bool
	}{
		{http.MethodGet, target + testChallengeToken, "", true},
		{http.MethodHead, target + testChallengeToken, "", true},
		{http.MethodPost, target + testChallengeToken, "", false},
		{http.MethodGet, target + testChallengeToken + "?", "", false},
		{http.MethodGet, target + testChallengeToken + "?view=text", "", false},
		{http.MethodGet, target + "./" + testChallengeToken, "", false},
		{http.MethodGet, target + "../" + testChallengeToken, "", false},
		{http.MethodGet, target + "%71" + testChallengeToken[1:], "", false},
		{http.MethodGet, target + testChallengeToken, "websocket", false},
		{http.MethodGet, target + strings.Repeat("a", 300), "", false},
	} {
		req := httptest.NewRequest(tc.method, tc.url, nil)
		if tc.upgrade != "" {
			req.Header.Set("Upgrade", tc.upgrade)
		}
		if token := acmeChallengeToken(req); (token == testChallengeToken) != tc.want || !tc.want && token != "" {
			t.Errorf("%s %s upgrade %q returned token %q", tc.method, tc.url, tc.upgrade, token)
		}
	}
}
