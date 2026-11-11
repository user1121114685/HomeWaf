package model

import "HomeWaf/model/baseorm"

type PipelineFeed struct {
	baseorm.BaseOrm
	Enabled         bool   `json:"enabled"`
	URL             string `gorm:"type:text" json:"url"`
	IntervalMinutes int    `json:"interval_minutes"`
	AuthToken       string `gorm:"type:text;serializer:homewaf_secret" json:"-"`
	SnapshotJSON    string `gorm:"type:text;serializer:homewaf_secret" json:"-"`
}

func (PipelineFeed) TableName() string { return "pipeline_feed" }
