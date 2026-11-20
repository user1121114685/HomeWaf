package waf_service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
)

func resetZeroSSLForTest(t *testing.T) {
	t.Helper()
	settings := global.CurrentSettings()
	key, kid, hmac := settings.GCONFIG_ZEROSSL_ACCESS_KEY, settings.GCONFIG_ZEROSSL_EAB_KID, settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY
	global.UpdateSettings(func(c *global.RuntimeSettings) {
		c.GCONFIG_ZEROSSL_ACCESS_KEY, c.GCONFIG_ZEROSSL_EAB_KID, c.GCONFIG_ZEROSSL_EAB_HMAC_KEY = "", "", ""
	})
	t.Cleanup(func() {
		global.UpdateSettings(func(c *global.RuntimeSettings) {
			c.GCONFIG_ZEROSSL_ACCESS_KEY, c.GCONFIG_ZEROSSL_EAB_KID, c.GCONFIG_ZEROSSL_EAB_HMAC_KEY = key, kid, hmac
		})
	})
}

func TestZeroSSLAccountConfigurationAndSecretStorage(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	resetZeroSSLForTest(t)
	if GetZeroSSLAccountStatus().Configured {
		t.Fatal("unconfigured account appears ready")
	}
	if err := EnsureZeroSSLAccount(); err == nil || !strings.Contains(err.Error(), "配置 ZeroSSL 认证") {
		t.Fatal("missing credentials must fail before contacting the CA")
	}
	if err := SaveZeroSSLAccount(ZeroSSLAccountInput{Mode: "eab", EABKID: "partial-id"}); err == nil {
		t.Fatal("partial EAB accepted")
	}
	if err := SaveZeroSSLAccount(ZeroSSLAccountInput{Mode: "eab", EABKID: "fixture-kid", EABHMACKey: "fixture-hmac-key"}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureZeroSSLAccount(); err != nil {
		t.Fatal(err)
	}
	if status := GetZeroSSLAccountStatus(); !status.Configured || !status.HasEABKID || !status.HasEABHMACKey {
		t.Fatal("complete EAB not usable immediately")
	}
	if err := SaveZeroSSLAccount(ZeroSSLAccountInput{Mode: "access_key", AccessKey: "fixture-access-key"}); err != nil {
		t.Fatal(err)
	}
	if status := GetZeroSSLAccountStatus(); !status.Configured || !status.HasAccessKey || status.HasEABKID || status.HasEABHMACKey {
		t.Fatal("changing API account retained stale EAB credentials")
	}
	var rows []model.SystemConfig
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if strings.Contains(row.Value, "fixture-") {
			t.Fatal("credential stored as plaintext")
		}
	}
	statusJSON, _ := json.Marshal(GetZeroSSLAccountStatus())
	if strings.Contains(string(statusJSON), "fixture-") {
		t.Fatal("status exposed credentials")
	}
	if err := SaveZeroSSLFetchedEAB("replaced-access-key", "wrong-kid", "wrong-hmac"); err == nil {
		t.Fatal("stale network response overwrote new account")
	}
	otherKey, err := encConfigValue("zerossl_access_key", "other-worker-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.SystemConfig{}).Where("item = ?", "zerossl_access_key").Update("value", otherKey).Error; err != nil {
		t.Fatal(err)
	}
	if err := SaveZeroSSLFetchedEAB("fixture-access-key", "old-worker-kid", "old-worker-hmac"); !errors.Is(err, errZeroSSLAccountChanged) {
		t.Fatalf("stale worker overwrote the current database account: %v", err)
	}
	if GetZeroSSLAccountStatus().HasEABKID {
		t.Fatal("failed stale-worker response changed runtime EAB")
	}
	if err := SaveZeroSSLAccount(ZeroSSLAccountInput{Mode: "access_key", AccessKey: "fixture-access-key"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveZeroSSLFetchedEAB("fixture-access-key", "fetched-kid", "fetched-hmac"); err != nil {
		t.Fatal(err)
	}
	if !GetZeroSSLAccountStatus().HasEABHMACKey {
		t.Fatal("fetched EAB not applied")
	}
	if err := db.Migrator().DropTable(&model.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveZeroSSLAccount(ZeroSSLAccountInput{Mode: "access_key", AccessKey: "must-not-apply"}); err == nil {
		t.Fatal("database failure ignored")
	}
	if global.CurrentSettings().GCONFIG_ZEROSSL_ACCESS_KEY != "fixture-access-key" {
		t.Fatal("failed save mutated runtime credentials")
	}
}

type zeroSSLTransport func(*http.Request) (*http.Response, error)

func (f zeroSSLTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestZeroSSLEABHTTPDoesNotExposeSecrets(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		body           string
		transportError bool
		success        bool
	}{
		{"success", 200, `{"success":true,"eab_kid":"safe-kid","eab_hmac_key":"safe-hmac"}`, false, true},
		{"numeric-success", 200, `{"success":1,"eab_kid":"safe-kid","eab_hmac_key":"safe-hmac"}`, false, true},
		{"body-error", 200, `{"success":false,"error":{"code":101,"message":"fixture-secret"}}`, false, false},
		{"malformed", 200, `fixture-secret`, false, false},
		{"status", 403, `fixture-secret`, false, false},
		{"transport", 0, "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: zeroSSLTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost || r.URL.Host != "api.zerossl.com" || r.URL.Query().Get("access_key") != "fixture-secret" {
					t.Fatal("invalid request")
				}
				if tc.transportError {
					return nil, errors.New("fixture-secret")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{}, Request: r}, nil
			})}
			kid, hmac, err := requestZeroSSLEAB(client, "fixture-secret")
			if tc.success {
				if err != nil || kid != "safe-kid" || hmac != "safe-hmac" {
					t.Fatalf("valid EAB response rejected: %v", err)
				}
			} else if err == nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal("failed request leaked secret or was accepted")
			}
		})
	}
}

func TestZeroSSLUsesConfiguredHTTPSProxy(t *testing.T) {
	if os.Getenv("HOMEWAF_TEST_ZEROSSL_PROXY") == "1" {
		client := newZeroSSLHTTPClient()
		defer client.CloseIdleConnections()
		response, err := client.Head("https://api.zerossl.com/")
		if response != nil {
			response.Body.Close()
		}
		if err == nil {
			t.Fatal("fixture proxy should reject the CONNECT tunnel")
		}
		return
	}
	var connected atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect && r.Host == "api.zerossl.com:443" {
			connected.Store(true)
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestZeroSSLUsesConfiguredHTTPSProxy$", "-test.count=1")
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "HOMEWAF_TEST_ZEROSSL_PROXY":
			continue
		}
		command.Env = append(command.Env, variable)
	}
	command.Env = append(command.Env, "HOMEWAF_TEST_ZEROSSL_PROXY=1", "HTTPS_PROXY="+proxy.URL, "HTTP_PROXY=", "NO_PROXY=")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("proxy test failed: %v %s", err, output)
	}
	if !connected.Load() {
		t.Fatal("ZeroSSL ignored HTTPS_PROXY and attempted a direct connection")
	}
}
