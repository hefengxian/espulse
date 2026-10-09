package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "espulse.db")
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// 采集器会以采集间隔（5s）频繁写入快照与样本，必须开 WAL 才能让读取不被写入阻塞。
	// busy_timeout 用于吸收偶发的写锁竞争（如手动刷新与定时刷新撞在一起）。
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
	}
	for _, p := range pragmas {
		if _, err := DB.Exec(p); err != nil {
			return fmt.Errorf("failed to apply %q: %v", p, err)
		}
	}

	log.Printf("Connected to SQLite database at: %s", dbPath)

	return Migrate()
}
