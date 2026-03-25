package database

import (
	"log"
)

func Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS clusters (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			hosts       TEXT NOT NULL,
			auth_type   TEXT DEFAULT 'none',
			username    TEXT DEFAULT '',
			password    TEXT DEFAULT '',
			api_key     TEXT DEFAULT '',
			color       TEXT DEFAULT '#18a058',
			notes       TEXT DEFAULT '',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS snippets (
			id          TEXT PRIMARY KEY,
			cluster_id  TEXT,
			title       TEXT NOT NULL,
			method      TEXT NOT NULL,
			path        TEXT NOT NULL,
			body        TEXT DEFAULT '',
			category    TEXT DEFAULT '',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS console_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_id  TEXT NOT NULL,
			method      TEXT NOT NULL,
			path        TEXT NOT NULL,
			body        TEXT DEFAULT '',
			status_code INTEGER,
			duration_ms INTEGER,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}

	log.Println("Database migration completed successfully")
	return nil
}
