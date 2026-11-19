package wafmangeweb

import (
	"HomeWaf/api"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafdb"
	"HomeWaf/wafdb/dialect"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMCPHTTPPermissionsPersistenceAndRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "mcp.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&model.OPlatformKey{}, &model.OPlatformLog{}, &model.PortConfig{}, &model.ConfigTemplate{}, &model.GateUser{}, &model.GateSession{}, &model.ShareDb{}, &model.AccessLog{}, &model.SecurityEvent{}, &model.EventPayload{}, &model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLog, oldPorts, oldService := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, waf_service.WafPipelineServiceApp
	oldSettings, oldIP, oldDomain := global.CurrentSettings(), global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST
	oldDialect := func() (d dialect.DBDialect) { defer func() { _ = recover() }(); return dialect.Get() }()
	dialect.Register(&dialect.SQLiteDialect{})
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, db
	global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST = "0.0.0.0/0,::/0", ""
	global.UpdateSettings(func(s *global.RuntimeSettings) {
		s.OpenPlatformEnabledUntil = time.Now().Add(model.OpenPlatformWindow).Unix()
	})
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](100)
	live := &wafpipeline.Runtime{RuleSteps: wafpipeline.DefaultRuleEngine().Steps, Forward: func(_ *wafpipeline.Snapshot, _ *wafpipeline.RequestContext, w http.ResponseWriter) {
		w.WriteHeader(204)
	}}
	waf_service.WafPipelineServiceApp = &waf_service.WafPipelineService{Prepare: live.Prepare}
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = oldDB, oldLog
		wafpipeline.Ports = oldPorts
		waf_service.WafPipelineServiceApp = oldService
		global.GWAF_IP_WHITELIST, global.GWAF_DOMAIN_WHITELIST = oldIP, oldDomain
		global.UpdateSettings(func(s *global.RuntimeSettings) { *s = oldSettings })
		dialect.Register(oldDialect)
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	key := func(scope string) (string, string) {
		id, value, err := waf_service.WafOPlatformKeyServiceApp.AddApi(request.WafOPlatformKeyAddReq{KeyName: "test-" + scope, MCPAccess: scope, IPWhitelist: "192.0.2.0/24"})
		if err != nil {
			t.Fatal(err)
		}
		return id, value
	}
	writeID, writeKey := key("write")
	_, readKey := key("read")
	_, restKey := key("")
	r := gin.New()
	(&WafWebManager{}).RegisterRoutes(r)
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, op := range api.MCPOperations() {
		if !routes[op.Method+" "+op.Path] {
			t.Fatalf("unregistered MCP operation: %s", op.Name)
		}
	}
	rpc := func(credential, method string, params any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req := httptest.NewRequest("POST", "http://localhost/mcp", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+credential)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	decode := func(w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var v map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatalf("not JSON (%d): %s", w.Code, w.Body.String())
		}
		return v
	}
	call := func(credential, name string, args any, fail bool) map[string]any {
		t.Helper()
		w := rpc(credential, "tools/call", map[string]any{"name": name, "arguments": args})
		v := decode(w)
		result, _ := v["result"].(map[string]any)
		failed := w.Code != 200 || v["error"] != nil || result["isError"] == true
		if failed != fail {
			t.Fatalf("%s: failed=%v expected=%v: %s", name, failed, fail, w.Body.String())
		}
		if failed {
			return v
		}
		structured, _ := result["structuredContent"].(map[string]any)
		return structured
	}
	init := rpc(writeKey, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "integration", "version": "1"}})
	if init.Code != 200 || !strings.Contains(init.Body.String(), "HomeWaf") {
		t.Fatalf("initialize: %s", init.Body.String())
	}
	listed := rpc(readKey, "tools/list", map[string]any{})
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), "log_detail") || strings.Contains(listed.Body.String(), `"name":"service_save"`) {
		t.Fatalf("readonly list: %s", listed.Body.String())
	}
	call(readKey, "service_save", map[string]any{}, true)
	for _, name := range []string{"account_change_password", "systemconfig_save", "sql_query", "file_read", "execute_command"} {
		call(writeKey, name, map[string]any{}, true)
	}
	if rpc(restKey, "tools/list", map[string]any{}).Code != 401 {
		t.Fatal("REST key became an MCP credential")
	}
	req := httptest.NewRequest("POST", "/api/v1/account/changemypwd", strings.NewReader(`{}`))
	req.Header.Set("X-API-Key", writeKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if decode(w)["code"] != float64(-999) {
		t.Fatalf("MCP key escaped into REST: %d", w.Code)
	}
	for _, raw := range []string{"https://evil.example", "null"} {
		req = httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+writeKey)
		req.Header.Set("Origin", raw)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal("untrusted Origin accepted")
		}
	}

	draft := call(writeKey, "service_new_draft", map[string]any{"port": 45129, "name": "MCP fixture", "upstream": "http://127.0.0.1:18888", "template_id": "observe", "defense_mode": "quiet"}, false)
	config := draft["data"].(map[string]any)["config"].(map[string]any)
	policy := config["policy"].(map[string]any)
	policy["gate"].(map[string]any)["enabled"] = false
	policy["access"].(map[string]any)["enabled"] = true
	policy["ruleset"].(map[string]any)["scripts"] = []any{map[string]any{"id": "mcp-rule", "mode": "block", "severity": "high", "source": `rule MCPBlock "" salience 10 { when MF.URL == "/blocked" then RF.Deny(); }`}}
	saved := call(writeKey, "service_save", map[string]any{"config": config}, false)
	var persisted wafpipeline.PortConfig
	wire, _ := json.Marshal(saved["data"])
	if err = json.Unmarshal(wire, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Revision != 1 || persisted.ListenEnabled {
		t.Fatalf("bad created state: %+v", persisted)
	}
	call(writeKey, "service_save", map[string]any{"config": config}, true) // stale revision zero
	call(writeKey, "service_save", map[string]any{"config": persisted, "plugin_profiles": map[string]any{"run": "cmd.exe"}}, true)
	call(writeKey, "service_save", map[string]any{"config": persisted, "initial_password": "DO-NOT-SET"}, true)
	loaded, err := waf_service.WafPipelineServiceApp.Get(persisted.Port)
	if err != nil || loaded.Revision != 1 {
		t.Fatal("save did not reach canonical persistence", err)
	}
	persisted.Name = "edited over MCP"
	saved = call(writeKey, "service_save", map[string]any{"config": persisted}, false)
	wire, _ = json.Marshal(saved["data"])
	_ = json.Unmarshal(wire, &persisted)
	if persisted.Revision != 2 {
		t.Fatal("edit lost optimistic concurrency")
	}
	index := call(readKey, "services_index", map[string]any{}, false)
	if len(index["data"].(map[string]any)["services"].([]any)) != 1 {
		t.Fatal("service index lost saved identity")
	}
	detail := call(readKey, "service_detail", map[string]any{"port": persisted.Port}, false)
	if detail["data"].(map[string]any)["revision"] != float64(2) {
		t.Fatal("targeted service read lost revision")
	}
	logID := "a1234567-1234-4234-8234-123456789abc"
	now := time.Now()
	entry := model.LogNarrow{
		ReqUUID: logID, HostCode: "pipeline-45129", Host: "example.test",
		URL: "/login?id=1%20UNION%20SELECT%201", Method: "POST", SRC_IP: "192.0.2.23",
		CONTENT_LENGTH: 90, STATUS_CODE: 403, RULE: "sqli", ACTION: "deny",
		CREATE_TIME: now.Format("2006-01-02 15:04:05"), UNIX_ADD_TIME: now.Unix(),
		TenantId: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE,
	}
	payload := model.EventPayload{
		ReqUUID: logID, HostCode: entry.HostCode, TenantId: entry.TenantId, UserCode: entry.UserCode, Kind: "event",
		HEADER: "User-Agent: sqlmap\r\nAuthorization: Bearer PRIVATE-MCP-FIXTURE",
		BODY:   `{"search":"1 UNION SELECT password FROM users--","password":"PRIVATE-LOGIN-FIXTURE"}`,
	}
	for _, row := range []any{&model.AccessLog{LogNarrow: entry}, &model.SecurityEvent{LogNarrow: entry}, &payload} {
		if err = db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	logResult := call(readKey, "log_detail", map[string]any{"req_uuid": logID, "current_db_name": wafdb.LiveLogName()}, false)
	logWire, _ := json.Marshal(logResult)
	for _, needed := range []string{logID, "sqlmap", "UNION SELECT password FROM users", "192.0.2.23", "diagnostics", "sqli"} {
		if !strings.Contains(string(logWire), needed) {
			t.Fatalf("MCP log is missing %s: %s", needed, logWire)
		}
	}
	if strings.Contains(string(logWire), "PRIVATE-MCP-FIXTURE") || strings.Contains(string(logWire), "PRIVATE-LOGIN-FIXTURE") {
		t.Fatal("log credential leaked")
	}
	call(readKey, "log_detail", map[string]any{"req_uuid": "a1234567-1234-4234-8234-000000000000"}, true)
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](100)
	if err = waf_service.WafPipelineServiceApp.Load(); err != nil {
		t.Fatalf("reload saved service: %v", err)
	}
	for _, path := range []string{"/allowed", "/blocked"} {
		snap, ok := wafpipeline.Ports.Get(persisted.Port)
		if !ok {
			t.Fatal("runtime was not published")
		}
		req := httptest.NewRequest("GET", "http://example.test"+path, nil)
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(persisted.Port)}))
		w := httptest.NewRecorder()
		live.Serve(snap, w, req)
		want := 204
		if path == "/blocked" {
			want = 403
		}
		if w.Code != want {
			t.Fatalf("actual runtime %s: got %d want %d: %s", path, w.Code, want, w.Body.String())
		}
	}
	// Deletion must remain explicit and stop-state guarded.
	call(writeKey, "service_delete", map[string]any{"port": persisted.Port}, true)
	call(writeKey, "service_delete", map[string]any{"port": persisted.Port, "confirm": true}, false)
	if _, err := waf_service.WafPipelineServiceApp.Get(persisted.Port); err == nil {
		t.Fatal("delete was not persisted")
	}
	var audit []model.OPlatformLog
	if err = db.Where("api_key_id = ?", writeID).Find(&audit).Error; err != nil || len(audit) < 4 {
		t.Fatalf("missing MCP audit: %d %v", len(audit), err)
	}
	recordedResult := false
	for _, row := range audit {
		if strings.Contains(row.RequestBody, writeKey) || strings.Contains(row.RequestBody, "MCPBlock") || strings.Contains(row.ResponseBody, "MCPBlock") {
			t.Fatal("raw credentials/config stored in MCP audit")
		}
		if row.MCPResult != "" {
			result, decryptErr := wafsec.DataDecrypt(row.MCPResult)
			if decryptErr != nil || result == "" {
				t.Fatalf("MCP result was not stored as decryptable content: %v", decryptErr)
			}
			recordedResult = true
		}
	}
	if !recordedResult {
		t.Fatal("MCP audit did not retain the result returned to the client")
	}
	if err = db.Model(&model.OPlatformKey{}).Where("id = ?", writeID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	if rpc(writeKey, "tools/list", map[string]any{}).Code != 401 {
		t.Fatal("disabled key still connects")
	}
	_ = db.Model(&model.OPlatformKey{}).Where("id = ?", writeID).Updates(map[string]any{"status": 1, "expire_time": time.Now().UTC().Add(-time.Hour).Format("2006-01-02 15:04:05")}).Error
	if rpc(writeKey, "tools/list", map[string]any{}).Code != 401 {
		t.Fatal("expired key still connects")
	}
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.OpenPlatformEnabledUntil = time.Now().Unix() })
	if rpc(readKey, "tools/list", map[string]any{}).Code != 401 {
		t.Fatal("platform switch did not disable MCP")
	}
}
