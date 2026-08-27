// Package store SQLite 持久化层：建表迁移 + 各实体 CRUD。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB 包装 *sql.DB，提供统一访问入口。
type DB struct {
	*sql.DB
}

// Open 打开（或创建）SQLite 数据库并执行迁移。
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	raw.SetMaxOpenConns(1) // SQLite 单写者，串行化连接避免锁竞争
	if err := raw.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	db := &DB{raw}
	if err := db.migrate(); err != nil {
		raw.Close()
		return nil, err
	}
	return db, nil
}

// migrate 建表迁移（幂等）。
func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS research_batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS instruments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES research_batches(id),
			name TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			string_count INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS string_positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instrument_id INTEGER NOT NULL REFERENCES instruments(id),
			position INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			min_freq_hz REAL NOT NULL,
			max_freq_hz REAL NOT NULL,
			standard_hz REAL NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(instrument_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS evidence_segments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES research_batches(id),
			source_type TEXT NOT NULL,
			source_ref TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			transcript TEXT NOT NULL,
			status TEXT NOT NULL,
			fingerprint TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS pitch_observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			segment_id INTEGER NOT NULL REFERENCES evidence_segments(id),
			string_pos INTEGER NOT NULL,
			frequency_hz REAL NOT NULL,
			unit TEXT NOT NULL DEFAULT 'hz',
			confidence REAL NOT NULL DEFAULT 1,
			recorded_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS term_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_term TEXT NOT NULL,
			region TEXT NOT NULL DEFAULT '',
			normalized TEXT NOT NULL,
			interval_cents INTEGER NOT NULL,
			unit TEXT NOT NULL DEFAULT 'cents',
			confidence REAL NOT NULL DEFAULT 1,
			notes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE(source_term, region)
		)`,
		`CREATE TABLE IF NOT EXISTS tuning_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES research_batches(id),
			instrument_id INTEGER NOT NULL REFERENCES instruments(id),
			segment_id INTEGER NOT NULL REFERENCES evidence_segments(id),
			from_position INTEGER NOT NULL,
			to_position INTEGER NOT NULL,
			described_term TEXT NOT NULL DEFAULT '',
			described_interval INTEGER NOT NULL,
			measured_interval INTEGER NOT NULL DEFAULT 0,
			delta_cents INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			verdict_reason TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS variant_candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			relation_id INTEGER NOT NULL REFERENCES tuning_relations(id),
			region TEXT NOT NULL,
			description TEXT NOT NULL,
			evidence_ref TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tuning_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES research_batches(id),
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			snapshot_json TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			superseded_by INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL DEFAULT 0,
			action TEXT NOT NULL,
			entity TEXT NOT NULL,
			entity_id INTEGER NOT NULL DEFAULT 0,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_instruments_batch ON instruments(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_segments_batch ON evidence_segments(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_batch ON tuning_relations(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_batch ON tuning_versions(batch_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, stmt[:60])
		}
	}
	return nil
}

// Close 关闭数据库。
func (db *DB) Close() error { return db.DB.Close() }

// Now 返回统一时间戳字符串（UTC RFC3339）。
func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
}

// parseTime 解析存储的时间戳。
func parseTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05Z07:00", s)
}
