package api

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPipelineHoneypotLogsRejectsInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/logs", func(c *gin.Context) {
		c.Set("is_openapi", true)
		new(WafPipelineApi).PipelineLogs(c)
	})
	for _, query := range []string{"port=5000&honeypot=all", "port=0&honeypot=true"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/logs?"+query, nil))
		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Code == 0 || result.Msg == "" || strings.Contains(result.Msg, "database") {
			t.Fatalf("invalid query reached log database: %s", w.Body.String())
		}
	}
}

func TestPipelineHoneypotLogsQueryUsesBoundedFilter(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "logs.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	old := global.GWAF_LOCAL_LOG_DB
	global.GWAF_LOCAL_LOG_DB = db
	t.Cleanup(func() { global.GWAF_LOCAL_LOG_DB = old })
	rows := []model.PipelineLog{}
	for i := range 25 {
		rows = append(rows, model.PipelineLog{ID: fmt.Sprintf("h%02d", i), PortID: "port:5000", Kind: "request", Verdict: "honeypot", TimeMillis: int64(i)})
	}
	rows = append(rows, model.PipelineLog{ID: "state", PortID: "port:5000", Kind: "state", Verdict: "honeypot", TimeMillis: 999})
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/logs", func(c *gin.Context) {
		c.Set("is_openapi", true)
		new(WafPipelineApi).PipelineLogs(c)
	})
	for _, tc := range []struct {
		query string
		count int
		first string
	}{{"&honeypot=true", 20, "h24"}, {"&honeypot=false", 26, "state"}, {"", 26, "state"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/logs?port=5000"+tc.query, nil))
		var result struct {
			Code int                 `json:"code"`
			Data []model.PipelineLog `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Code != 0 || len(result.Data) != tc.count {
			t.Fatalf("wrong query response: %s", w.Body.String())
		}
		if result.Data[0].ID != tc.first {
			t.Fatalf("filter/order not applied: %+v", result.Data[0])
		}
	}
}
