// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package model

import "HomeWaf/model/baseorm"

type CaServerInfo struct {
	baseorm.BaseOrm
	CaServerName    string `gorm:"size:255" json:"ca_server_name"`    //CA服务器名称
	CaServerAddress string `gorm:"size:500" json:"ca_server_address"` //CA服务器地址
	Remarks         string `gorm:"size:500" json:"remarks"`           //备注
}
