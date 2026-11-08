// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package model

import "HomeWaf/model/baseorm"

type PrivateGroup struct {
	baseorm.BaseOrm
	PrivateGroupName        string `gorm:"size:255" json:"private_group_name"`         //分组名称
	PrivateGroupBelongCloud string `gorm:"size:100" json:"private_group_belong_cloud"` //分组归属云
}
