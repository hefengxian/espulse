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
		// snapshots 保存每个 (集群, 数据类型) 的最新一份采集快照。
		// 它让「很久没用过 / 进程重启过」的集群也能秒开：先渲染旧快照，再后台刷新（见 PRD §6.3）。
		`CREATE TABLE IF NOT EXISTS snapshots (
			cluster_id TEXT NOT NULL,
			kind       TEXT NOT NULL,
			data       TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_id, kind)
		);`,
		// index_samples 是索引计数的短期样本环，仅用于差分计算写入 / 搜索速率。
		// 主键顺序把 at 放在 index_name 之前，使「按时间裁剪」能直接命中主键前缀。
		`CREATE TABLE IF NOT EXISTS index_samples (
			cluster_id   TEXT    NOT NULL,
			at           INTEGER NOT NULL,
			index_name   TEXT    NOT NULL,
			idx_total    INTEGER NOT NULL,
			search_total INTEGER NOT NULL,
			docs_count   INTEGER NOT NULL,
			PRIMARY KEY (cluster_id, at, index_name)
		) WITHOUT ROWID;`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}

	log.Println("Database migration completed successfully")
	return nil
}
