package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	commonrequest "HomeWaf/model/common/request"
	"HomeWaf/model/request"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestSystemLogFilteringAndFailures(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "system-log.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	previous := global.GWAF_LOCAL_LOG_DB
	global.GWAF_LOCAL_LOG_DB = db
	t.Cleanup(func() {
		global.GWAF_LOCAL_LOG_DB = previous
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(&model.WafSysLog{}); err != nil {
		t.Fatal(err)
	}
	for index, row := range []model.WafSysLog{{OpType: "信息", OpContent: "WAF启动"}, {OpType: "升级", OpContent: "下载完成"}, {OpType: "信息", OpContent: "升级失败"}} {
		row.Id = string(rune('a' + index))
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	query := request.WafSysLogSearchReq{PageInfo: commonrequest.PageInfo{PageIndex: 1, PageSize: 1, Keyword: "升级"}}
	rows, total, err := WafSysLogServiceApp.GetListApi(query)
	if err != nil || total != 2 || len(rows) != 1 {
		t.Fatalf("keyword/pagination lost: %d %d %v", len(rows), total, err)
	}
	query.OpType = "信息"
	query.OpContent = "失败"
	rows, total, err = WafSysLogServiceApp.GetListApi(query)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].OpContent != "升级失败" {
		t.Fatalf("filters lost: %+v %d %v", rows, total, err)
	}
	global.GWAF_LOCAL_LOG_DB = nil
	if _, _, err := WafSysLogServiceApp.GetListApi(query); err == nil {
		t.Fatal("unavailable database was treated as successful empty logs")
	}
}
