// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package model

import (
	"HomeWaf/internal/model/baseorm"
)

type WafSysLog struct {
	baseorm.BaseOrm
	OpType    string `gorm:"size:100" json:"op_type"`     //操作类型
	OpContent string `gorm:"type:text" json:"op_content"` //操作内容
}
