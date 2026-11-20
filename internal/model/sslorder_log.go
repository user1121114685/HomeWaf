package model

import "HomeWaf/internal/model/customtype"

type SslOrderLog struct {
	ID        uint64              `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID   string              `gorm:"size:64;index:idx_ssl_order_log_order_id" json:"-"`
	TenantID  string              `gorm:"column:tenant_id;size:64" json:"-"`
	UserCode  string              `gorm:"column:user_code;size:64" json:"-"`
	CreatedAt customtype.JsonTime `json:"time"`
	Level     string              `gorm:"size:16" json:"level"`
	Message   string              `gorm:"type:text" json:"message"`
}
