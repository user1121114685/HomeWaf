// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package model

import (
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
)

/*
*
分库
*/
type ShareDb struct {
	baseorm.BaseOrm
	DbLogicType string              `gorm:"size:50" json:"db_logic_type"` //数据库逻辑类型  默认："log"
	StartTime   customtype.JsonTime `json:"start_time"`                   //开始时间
	EndTime     customtype.JsonTime `json:"end_time"`                     //结束时间
	FileName    string              `gorm:"size:255" json:"file_name"`    //文件名：SQLite 下为 .db 文件名；MySQL 下为表名（无后缀）
	Cnt         int64               `json:"cnt"`                          //当前数量
	PeriodKey   string              `gorm:"size:16" json:"period_key"`    //周期键（月，如 202609）
}
