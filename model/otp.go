// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package model

import "HomeWaf/model/baseorm"

type Otp struct {
	baseorm.BaseOrm
	UserName string `gorm:"size:255" json:"user_name"` //用户名
	Url      string `gorm:"size:500" json:"url"`       //URL
	Secret   string `gorm:"size:255" json:"-"`         //密钥(TOTP)：wafsec 加密存储，永不回显
	Issuer   string `gorm:"size:255" json:"issuer"`    //发行者标识
	Remarks  string `gorm:"size:500" json:"remarks"`   //备注
}
