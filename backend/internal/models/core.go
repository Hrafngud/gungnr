package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	GitHubID    int64     `gorm:"uniqueIndex;not null"`
	Login       string    `gorm:"size:64;not null"`
	AvatarURL   string    `gorm:"size:512"`
	LastLoginAt time.Time `gorm:"not null"`
}

type Project struct {
	gorm.Model
	Name      string `gorm:"size:120;not null"`
	RepoURL   string `gorm:"size:512"`
	Path      string `gorm:"size:512"`
	ProxyPort int
	DBPort    int
	Status    string `gorm:"size:32"`
}

type Deployment struct {
	gorm.Model
	ProjectID uint   `gorm:"index;not null"`
	Subdomain string `gorm:"size:120;not null"`
	Hostname  string `gorm:"size:255"`
	Port      int
	State     string `gorm:"size:32"`
	LastRunAt *time.Time
}

type Job struct {
	gorm.Model
	Type               string `gorm:"size:64;not null"`
	Status             string `gorm:"size:32;not null"`
	StartedAt          *time.Time
	FinishedAt         *time.Time
	Error              string `gorm:"type:text"`
	Input              string `gorm:"type:text"`
	LogLines           string `gorm:"type:text"`
	HostToken          string `gorm:"size:64;index"`
	HostTokenExpiresAt *time.Time
	HostTokenClaimedAt *time.Time
	HostTokenUsedAt    *time.Time
}

type AuditLog struct {
	gorm.Model
	UserID    uint   `gorm:"index"`
	UserLogin string `gorm:"size:64"`
	Action    string `gorm:"size:64;not null"`
	Target    string `gorm:"size:255"`
	Metadata  string `gorm:"type:text"`
}

type Settings struct {
	gorm.Model
	BaseDomain            string `gorm:"size:255"`
	GitHubToken           string `gorm:"type:text"`
	CloudflareToken       string `gorm:"type:text"`
	CloudflareAccountID   string `gorm:"size:255"`
	CloudflareZoneID      string `gorm:"size:255"`
	CloudflaredTunnel     string `gorm:"size:255"`
	CloudflaredConfigPath string `gorm:"size:512"`
}

type OnboardingState struct {
	gorm.Model
	UserID       uint `gorm:"uniqueIndex"`
	Home         bool
	HostSettings bool
	Networking   bool
	GitHub       bool
}
