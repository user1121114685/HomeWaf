package wafqueue

import (
	"HomeWaf/common/queue"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafenginecore"
	"HomeWaf/wafpipeline"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPipelineRequestsReachAccessAndSecurityStorage(t *testing.T) {
	oldQueue, oldDB, oldType := global.GQEQUE_LOG_DB, global.GWAF_LOCAL_LOG_DB, global.CurrentSettings().GWAF_RUNTIME_RECORD_LOG_TYPE
	global.GQEQUE_LOG_DB = queue.NewQueue()
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "all" })
	t.Cleanup(func() {
		global.GQEQUE_LOG_DB, global.GWAF_LOCAL_LOG_DB = oldQueue, oldDB
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = oldType })
	})
	defer resetTier("db")()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "logs.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.AccessLog{}, &model.SecurityEvent{}, &model.EventPayload{}); err != nil {
		t.Fatal(err)
	}
	global.GWAF_LOCAL_LOG_DB = db
	var originCalls atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originCalls.Add(1)
		w.WriteHeader(201)
	}))
	defer origin.Close()
	forward := &wafpipeline.Forwarder{}
	runtime := &wafpipeline.Runtime{ForwardPrepare: forward.Prepare, Forward: forward.Serve, OnResult: wafenginecore.RecordPipelineWebLog}
	snapshot := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: 15000, Protocol: "http", Upstream: origin.URL, HostCode: "pipeline-15000", HostNames: []string{"family.test"}, Policy: wafpipeline.DefaultPolicy()}}
	if err := runtime.Prepare(snapshot); err != nil {
		t.Fatal(err)
	}
	defer wafpipeline.CloseRetiredProxy(snapshot, nil)
	for _, tc := range []struct {
		host   string
		status int
	}{{"family.test", 201}, {"other.test", 403}} {
		req := httptest.NewRequest("GET", "http://"+tc.host+"/fixture", nil)
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: 15000}))
		rec := httptest.NewRecorder()
		runtime.Serve(snapshot, rec, req)
		if rec.Code != tc.status {
			t.Fatalf("HTTP status=%d, want %d", rec.Code, tc.status)
		}
		item, ok := global.GQEQUE_LOG_DB.Dequeue()
		lg, typed := item.(*innerbean.WebLog)
		if !ok || !typed || lg.STATUS_CODE != tc.status {
			t.Fatal("runtime result did not reach the access log queue")
		}
		storeTiered([]*innerbean.WebLog{lg})
	}
	var access []model.AccessLog
	var events []model.SecurityEvent
	if err := db.Find(&access).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(access) != 2 || len(events) != 1 || originCalls.Load() != 1 {
		t.Fatalf("access=%d events=%d origin=%d; want 2,1,1", len(access), len(events), originCalls.Load())
	}
	if events[0].RULE != "host_not_allowed" || events[0].HostCode != snapshot.Config.HostCode || events[0].STATUS_CODE != 403 {
		t.Fatal("stored security event lost request attribution")
	}
}
