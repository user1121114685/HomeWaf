package model

import "HomeWaf/model/baseorm"

// Port policy and secrets are separate so listing or exporting policy can never
// accidentally serialize the per-port signing key or TLS private key.
type PortConfig struct {
	baseorm.BaseOrm
	Port        int    `gorm:"uniqueIndex:idx_pipeline_port" json:"port"`
	ConfigJSON  string `gorm:"type:text" json:"-"`
	SecretsJSON string `gorm:"type:text;serializer:homewaf_secret" json:"-"`
	Revision    int64  `json:"revision"`
}

func (PortConfig) TableName() string { return "port_config" }

// Ownership survives deletion of a migrated port configuration so an old
// listener can never silently return on restart. Original host data is kept.
type PipelinePortMigration struct {
	Port        int    `gorm:"primaryKey" json:"port"`
	State       string `gorm:"size:24" json:"state"`
	SourcesJSON string `gorm:"type:text" json:"sources_json"`
	SourceHash  string `gorm:"size:64" json:"-"`
	ImportedAt  int64  `json:"imported_at"`
	Operator    string `gorm:"size:128" json:"operator"`
}

func (PipelinePortMigration) TableName() string { return "pipeline_port_migration" }

type ConfigTemplate struct {
	baseorm.BaseOrm
	Code       string `gorm:"size:64;uniqueIndex:idx_pipeline_template" json:"code"`
	Name       string `gorm:"size:128" json:"name"`
	Version    int    `json:"version"`
	ConfigJSON string `gorm:"type:text" json:"-"`
}

func (ConfigTemplate) TableName() string { return "config_template" }

type GateUser struct {
	baseorm.BaseOrm
	PortID         string `gorm:"size:32;uniqueIndex:idx_gate_user,priority:1" json:"port_id"`
	Username       string `gorm:"size:128;uniqueIndex:idx_gate_user,priority:2" json:"username"`
	PasswordHash   string `gorm:"size:128" json:"-"`
	TOTPSecret     string `gorm:"type:text;serializer:homewaf_secret" json:"-"`
	TOTPConfigured bool   `gorm:"-" json:"totp_configured"`
	AuthVersion    string `gorm:"size:64" json:"-"`
	Enabled        bool   `json:"enabled"`
	ExpiresAt      int64  `json:"expires_at"`
	Remark         string `gorm:"size:500" json:"remark"`
}

func (GateUser) TableName() string { return "gate_user" }

type GateSession struct {
	baseorm.BaseOrm
	PortID      string `gorm:"size:32;index:idx_gate_session_port" json:"port_id"`
	Username    string `gorm:"size:128" json:"username"`
	TokenHash   string `gorm:"size:64;uniqueIndex:idx_gate_session_hash" json:"-"`
	AuthVersion string `gorm:"size:64" json:"-"`
	ClientIP    string `gorm:"size:64" json:"client_ip"`
	CreatedAt   int64  `json:"created_at"`
	LastSeen    int64  `json:"last_seen"`
	ExpiresAt   int64  `gorm:"index:idx_gate_session_expire" json:"expires_at"`
	Revoked     bool   `json:"revoked"`
}

func (GateSession) TableName() string { return "gate_session" }

// A dirty shutdown discards persisted browser sessions. This closes the window
// where an asynchronous logout could otherwise be resurrected after a crash.
type GateRuntime struct {
	ID            int `gorm:"primaryKey"`
	CleanShutdown bool
	Owner         string `gorm:"size:64" json:"-"`
}

func (GateRuntime) TableName() string { return "gate_runtime" }
