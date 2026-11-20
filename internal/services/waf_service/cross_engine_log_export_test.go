// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

//go:build crossdb

// 验证新库支持按租户和用户查询日志导出任务。
package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/common/uuid"
	"testing"
	"time"

	"gorm.io/gorm"
)

func runLogExportCases(t *testing.T, db *gorm.DB) {
	t.Run("LogExportUserCodeColumn", func(t *testing.T) {
		if !db.Migrator().HasColumn(&model.LogExportTask{}, "user_code") {
			t.Fatal("新库 log_export_task 应包含 user_code 列")
		}

		task := model.LogExportTask{
			Id:        uuid.GenUUID(),
			UserCode:  global.GWAF_USER_CODE,
			TenantID:  global.GWAF_TENANT_ID,
			CreatorID: "creator_" + sfx(),
			RequestID: "req_" + sfx(),
			Status:    "queued",
			CreatedAt: time.Now(),
		}
		must(t, db.Create(&task).Error)

		var got model.LogExportTask
		must(t, db.Where("id = ?", task.Id).First(&got).Error)
		if got.CreatorID != task.CreatorID {
			t.Fatalf("读回任务不匹配: got=%+v want=%+v", got, task)
		}
		var rows []model.LogExportTask
		must(t, db.Where("status NOT IN ?", []string{"running", "packaging"}).Find(&rows).Error)
	})
}
