package wafmangeweb

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"HomeWaf/cache"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/common/response"
	"github.com/gin-gonic/gin"
)

type auditedRoute struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Handler    string   `json:"handler"`
	Auth       string   `json:"auth"`
	RoleGuard  bool     `json:"role_guard"`
	Middleware []string `json:"middleware"`
}

func auditURL(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
			parts[i] = "audit-missing"
		}
	}
	return strings.Join(parts, "/")
}

// Exercise the registered production routes, not a manually maintained copy.
// Discovery aborts before middleware/handlers; rejection tests cannot perform
// file writes, issue certificates, send notifications, or change real services.
func TestAllBackendRouteAuthenticationBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSettings := global.CurrentSettings()
	oldRelease, oldEmergency := global.GWAF_RELEASE, global.GWAF_SECURITY_EMERGENCY_PATH
	oldCache := global.GCACHE_WAFCACHE
	oldIP, oldDomain := global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST
	store, err := cache.NewCacheStore("memory", nil)
	if err != nil {
		t.Fatal(err)
	}
	global.GCACHE_WAFCACHE = store
	global.GWAF_RELEASE, global.GWAF_SECURITY_EMERGENCY_PATH = "false", "audit-emergency"
	global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST = "0.0.0.0/0,::/0", ""
	global.UpdateSettings(func(s *global.RuntimeSettings) {
		s.GCONFIG_RECORD_DEBUG_ENABLE = 1
		s.GCONFIG_RECORD_DEBUG_PWD = ""
		s.GCONFIG_COMM_LEGACY_KEY = true
		s.GCONFIG_ENABLE_STRICT_IP_BINDING = 0
		s.GCONFIG_ENABLE_DEVICE_FINGERPRINT = 0
		s.GCONFIG_RECORD_FORCE_BIND_2FA = 0
	})
	t.Cleanup(func() {
		global.UpdateSettings(func(s *global.RuntimeSettings) { *s = oldSettings })
		global.GWAF_RELEASE, global.GWAF_SECURITY_EMERGENCY_PATH = oldRelease, oldEmergency
		global.GCACHE_WAFCACHE = oldCache
		global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST = oldIP, oldDomain
	})
	web := &WafWebManager{}
	discovery := gin.New()
	var names []string
	discovery.Use(func(c *gin.Context) { names = append([]string(nil), c.HandlerNames()...); c.AbortWithStatus(204) })
	web.RegisterRoutes(discovery)
	var routes []auditedRoute
	for _, route := range discovery.Routes() {
		discovery.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(route.Method, auditURL(route.Path), nil))
		item := auditedRoute{Method: route.Method, Path: route.Path, Handler: route.Handler, Auth: "public", Middleware: append([]string(nil), names[1:len(names)-1]...)}
		for _, name := range names {
			if strings.Contains(name, ".Auth.") {
				item.Auth = "session-or-api-key"
			}
			if strings.Contains(name, ".TokenOnlyAuth.") {
				item.Auth = "session-only"
			}
			if strings.Contains(name, ".RequireRole.") {
				item.RoleGuard = true
			}
		}
		if strings.HasSuffix(route.Path, "/exports/:id/download") {
			item.Auth = "download-ticket"
		}
		routes = append(routes, item)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Path+routes[i].Method < routes[j].Path+routes[j].Method })
	if file := os.Getenv("HOMEWAF_API_AUDIT_OUTPUT"); file != "" {
		raw, _ := json.MarshalIndent(routes, "", "  ")
		if err := os.WriteFile(file, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	actual := gin.New()
	web.RegisterRoutes(actual)
	store.SetWithTTl(enums.CACHE_TOKEN+"audit-invalid-role", model.TokenInfo{LoginAccount: "audit-fixture", Role: "invalid-role"}, time.Hour)
	store.SetWithTTl(enums.CACHE_TOKEN+"audit-role", model.TokenInfo{LoginAccount: "audit-fixture", Role: enums.ROLE_AUDIT_ADMIN}, time.Hour)
	store.SetWithTTl(enums.CACHE_TOKEN+"audit-system-role", model.TokenInfo{LoginAccount: "audit-fixture", Role: enums.ROLE_SYSTEM_ADMIN}, time.Hour)
	store.SetWithTTl(enums.CACHE_TOKEN+"audit-security-role", model.TokenInfo{LoginAccount: "audit-fixture", Role: enums.ROLE_SECURITY_ADMIN}, time.Hour)
	public, protected := 0, 0
	specialRoutes := map[string]bool{
		"GET /api/v1/public/dashboard-share":      true,
		"POST /api/v1/public/login":               true,
		"POST /api/v1/public/seckey":              true,
		"GET /api/v1/public/uiconfig":             true,
		"GET /api/v1/waflog/exports/:id/download": true,
		"GET /audit-emergency":                    true,
	}
	for _, route := range routes {
		if route.Auth == "public" || route.Auth == "download-ticket" {
			public++
			if !specialRoutes[route.Method+" "+route.Path] {
				t.Errorf("unexpected public route: %s %s", route.Method, route.Path)
			}
			t.Logf("special authentication: %s %s (%s)", route.Method, route.Path, route.Auth)
			continue
		}
		protected++
		if route.Auth == "session-only" {
			t.Run("api-key-rejected/"+route.Method+route.Path, func(t *testing.T) {
				req := httptest.NewRequest(route.Method, auditURL(route.Path), nil)
				req.Header.Set("X-API-Key", "isolated-audit-key")
				rec := httptest.NewRecorder()
				actual.ServeHTTP(rec, req)
				var result struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Code != response.AUTHFAIL {
					t.Fatalf("token-only route accepted API key: HTTP %d", rec.Code)
				}
			})
		}
		for _, token := range []string{"", "audit-expired-token", "audit-invalid-role"} {
			t.Run(route.Method+" "+route.Path+"/"+token, func(t *testing.T) {
				request := httptest.NewRequest(route.Method, auditURL(route.Path), nil)
				if token != "" {
					request.Header.Set("X-Token", token)
				}
				recorder := httptest.NewRecorder()
				actual.ServeHTTP(recorder, request)
				var result struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || result.Code != response.AUTHFAIL {
					t.Fatalf("expected authentication rejection before handler: HTTP %d, code %d, parse error %v", recorder.Code, result.Code, err)
				}
			})
		}
	}
	t.Logf("audited %d routes: %d protected routes × 3 rejection cases; %d public/ticket routes separately inventoried", len(routes), protected, public)
	// Audit administrators must not mutate service policy, credentials, files,
	// certificate orders, or send test notifications. Run the real middleware.
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.GCONFIG_ENABLE_REPLAY_PROTECT = 0 })
	for _, path := range []string{"/api/v1/public/login", "/api/v1/public/seckey"} {
		t.Run("public-empty/"+path, func(t *testing.T) {
			req := httptest.NewRequest("POST", path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			actual.ServeHTTP(rec, req)
			var result struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Code != response.ERROR {
				t.Fatalf("empty public authentication request accepted: %s", rec.Body.String())
			}
		})
	}
	t.Run("share-missing-capability", func(t *testing.T) {
		rec := httptest.NewRecorder()
		actual.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/public/dashboard-share", nil))
		if rec.Code != 401 {
			t.Fatalf("dashboard share without capability: HTTP %d", rec.Code)
		}
	})
	for _, route := range auditedWriteRoles {
		opposite := "audit-system-role"
		if route.role == enums.ROLE_SYSTEM_ADMIN {
			opposite = "audit-security-role"
		}
		for _, token := range []string{"audit-role", opposite} {
			t.Run("write-role/"+route.method+route.path+"/"+token, func(t *testing.T) {
				req := httptest.NewRequest(route.method, auditURL(route.path), nil)
				req.Header.Set("X-Token", token)
				rec := httptest.NewRecorder()
				actual.ServeHTTP(rec, req)
				var result struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Code != response.FORBIDDEN {
					t.Fatalf("audit role was not rejected: %s", rec.Body.String())
				}
			})
		}
	}
	if protected < 100 {
		t.Fatal("route inventory unexpectedly incomplete")
	}
}
