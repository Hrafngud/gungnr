package db

import (
	"fmt"

	"gorm.io/gorm"
)

func CleanupLegacyHostWorker(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if err := db.Exec("DELETE FROM jobs WHERE type IN ('host_deploy', 'host-deploy')").Error; err != nil {
		return fmt.Errorf("delete legacy host_deploy jobs: %w", err)
	}

	// Check if the legacy column exists before trying to update it.
	if db.Migrator().HasColumn("jobs", "host_token") {
		if err := db.Exec(`UPDATE jobs
			SET host_token = '',
				host_token_expires_at = NULL,
				host_token_claimed_at = NULL,
				host_token_used_at = NULL
			WHERE host_token IS NOT NULL AND host_token <> ''`).Error; err != nil {
			return fmt.Errorf("clear legacy host tokens: %w", err)
		}
	}

	return nil
}
