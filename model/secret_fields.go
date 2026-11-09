package model

import (
	"HomeWaf/internal/secretfield"
	"gorm.io/gorm"
)

func (Hosts) BeforeSave(tx *gorm.DB) error         { return secretfield.ProtectUpdates(tx) }
func (SslConfig) BeforeSave(tx *gorm.DB) error     { return secretfield.ProtectUpdates(tx) }
func (SslOrder) BeforeSave(tx *gorm.DB) error      { return secretfield.ProtectUpdates(tx) }
func (PrivateInfo) BeforeSave(tx *gorm.DB) error   { return secretfield.ProtectUpdates(tx) }
func (NotifyChannel) BeforeSave(tx *gorm.DB) error { return secretfield.ProtectUpdates(tx) }
func (HttpAuthBase) BeforeSave(tx *gorm.DB) error  { return secretfield.ProtectUpdates(tx) }
