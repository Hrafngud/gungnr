package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func CleanupLegacyHostWorker(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if !db.Migrator().HasTable("jobs") {
		return nil
	}

	if err := db.Exec("DELETE FROM jobs WHERE type IN ('host_deploy', 'host-deploy')").Error; err != nil {
		return fmt.Errorf("delete legacy host_deploy jobs: %w", err)
	}

	columnTypes, err := db.Migrator().ColumnTypes("jobs")
	if err != nil {
		return fmt.Errorf("load jobs columns: %w", err)
	}

	columns := make(map[string]bool, len(columnTypes))
	for _, columnType := range columnTypes {
		columns[columnType.Name()] = true
	}

	if !columns["host_token"] {
		return nil
	}

	setParts := []string{"host_token = ''"}
	if columns["host_token_expires_at"] {
		setParts = append(setParts, "host_token_expires_at = NULL")
	}
	if columns["host_token_claimed_at"] {
		setParts = append(setParts, "host_token_claimed_at = NULL")
	}
	if columns["host_token_used_at"] {
		setParts = append(setParts, "host_token_used_at = NULL")
	}

	if len(setParts) > 0 {
		query := fmt.Sprintf(`UPDATE jobs SET %s WHERE host_token IS NOT NULL AND host_token <> ''`, strings.Join(setParts, ", "))
		if err := db.Exec(query).Error; err != nil {
			return fmt.Errorf("clear legacy host tokens: %w", err)
		}
	}

	return nil
}
