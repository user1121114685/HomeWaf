package model

// DashboardShare is a capability for one read-only, redacted screen endpoint.
// The random token is returned once; only its SHA-256 digest is persisted.
type DashboardShare struct {
	ID        string `gorm:"size:32;primaryKey" json:"id"`
	TenantID  string `gorm:"size:64;index" json:"-"`
	UserCode  string `gorm:"size:64" json:"-"`
	TokenHash string `gorm:"size:64;uniqueIndex" json:"-"`
	Label     string `gorm:"size:64" json:"label"`
	Scope     string `gorm:"size:64" json:"scope"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `gorm:"index" json:"expires_at"`
	RevokedAt int64  `json:"revoked_at"`
}

func (DashboardShare) TableName() string { return "dashboard_share" }

type DashboardTarget struct {
	ID        string  `gorm:"size:64;primaryKey" json:"-"`
	TenantID  string  `gorm:"size:64" json:"-"`
	UserCode  string  `gorm:"size:64" json:"-"`
	Mode      string  `gorm:"size:16" json:"mode"`
	Label     string  `gorm:"size:32" json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// Discovery metadata is runtime state, never a configured/fabricated location.
	Status         string `gorm:"-" json:"status"`
	PublicIP       string `gorm:"-" json:"public_ip,omitempty"`
	Country        string `gorm:"-" json:"country,omitempty"`
	Region         string `gorm:"-" json:"region,omitempty"`
	City           string `gorm:"-" json:"city,omitempty"`
	HasCoordinates bool   `gorm:"-" json:"has_coordinates"`
	Source         string `gorm:"-" json:"source,omitempty"`
	CheckedAt      int64  `gorm:"-" json:"checked_at,omitempty"`
	Message        string `gorm:"-" json:"message,omitempty"`
}

func (DashboardTarget) TableName() string { return "dashboard_target" }
