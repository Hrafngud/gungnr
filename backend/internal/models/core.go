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
	Type       string `gorm:"size:64;not null"`
	Status     string `gorm:"size:32;not null"`
	StartedAt  *time.Time
	FinishedAt *time.Time
	Error      string `gorm:"type:text"`
	Input      string `gorm:"type:text"`
	LogLines   string `gorm:"type:text"`
}
