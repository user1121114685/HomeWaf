package model

// Only aggregated network prefixes are retained; no credentials or request body.
type PipelineSuccessSource struct {
	Port     uint16 `gorm:"primaryKey"`
	Prefix   string `gorm:"size:64;primaryKey"`
	LastSeen int64  `gorm:"index"`
}

func (PipelineSuccessSource) TableName() string { return "pipeline_success_source" }

type PipelineHoneypot struct {
	ID         string `gorm:"size:32;primaryKey"`
	Revision   int64
	ConfigJSON string `gorm:"type:text"`
}

func (PipelineHoneypot) TableName() string { return "pipeline_honeypot" }
