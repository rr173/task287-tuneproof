package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"task287-tuneproof/internal/model"
)

// VersionStore 调弦版本存取。
type VersionStore struct{ db *DB }

// NewVersionStore 构造版本存储。
func NewVersionStore(db *DB) *VersionStore { return &VersionStore{db: db} }

// Create 创建版本（草稿），写入快照与指纹。
func (s *VersionStore) Create(v *model.TuningVersion) (int64, error) {
	if v.Name == "" {
		return 0, model.NewValidationError("name", "required")
	}
	if v.SnapshotJSON == "" || v.Fingerprint == "" {
		return 0, model.NewValidationError("snapshot", "snapshot and fingerprint required")
	}
	res, err := s.db.Exec(
		`INSERT INTO tuning_versions(batch_id, name, status, snapshot_json, fingerprint, superseded_by, created_at, version)
		 VALUES(?,?,?,?,?,0,?,1)`,
		v.BatchID, v.Name, v.Status, v.SnapshotJSON, v.Fingerprint, Now())
	if err != nil {
		return 0, fmt.Errorf("insert version: %w", err)
	}
	return res.LastInsertId()
}

// Get 按 ID 查询版本。
func (s *VersionStore) Get(id int64) (*model.TuningVersion, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_id, name, status, snapshot_json, fingerprint, superseded_by, created_at, version
		 FROM tuning_versions WHERE id = ?`, id)
	v, err := scanVersion(row)
	if err != nil {
		return nil, err
	}
	if v.Status == model.VersionFrozen {
		rows, err := s.db.Query(
			`SELECT id, batch_id, instrument_id, segment_id, from_position, to_position,
			   described_term, described_interval, measured_interval, delta_cents, status, verdict_reason, version, created_at, updated_at
			  FROM tuning_relations WHERE batch_id = ? AND status = ? ORDER BY id`,
			v.BatchID, model.RelationConfirmed)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var rels []*model.TuningRelation
		for rows.Next() {
			r, err := scanRelation(rows)
			if err != nil {
				return nil, err
			}
			rels = append(rels, r)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		payload, err := json.Marshal(rels)
		if err != nil {
			return nil, err
		}
		v.SnapshotJSON = string(payload)
	}
	return v, nil
}

// List 按批次列出版本。
func (s *VersionStore) List(batchID int64) ([]*model.TuningVersion, error) {
	rows, err := s.db.Query(
		`SELECT id, batch_id, name, status, snapshot_json, fingerprint, superseded_by, created_at, version
		 FROM tuning_versions WHERE batch_id = ? ORDER BY id`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []*model.TuningVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UpdateStatus 乐观锁推进版本状态（草稿→共享→冻结→替代）。
func (s *VersionStore) UpdateStatus(id int64, status string, supersededBy int64, expectedVersion int64) (*model.TuningVersion, error) {
	res, err := s.db.Exec(
		`UPDATE tuning_versions SET status = ?, superseded_by = ?, version = version + 1
		 WHERE id = ? AND version = ?`,
		status, supersededBy, id, expectedVersion)
	if err != nil {
		return nil, fmt.Errorf("update version status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		if _, err := s.Get(id); errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, model.ErrConflict
	}
	return s.Get(id)
}

// FrozenCount 统计批次内冻结版本数。
func (s *VersionStore) FrozenCount(batchID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tuning_versions WHERE batch_id = ? AND status = ?`,
		batchID, model.VersionFrozen).Scan(&n)
	return n, err
}

func scanVersion(sc interface{ Scan(...any) error }) (*model.TuningVersion, error) {
	var v model.TuningVersion
	var created string
	if err := sc.Scan(&v.ID, &v.BatchID, &v.Name, &v.Status, &v.SnapshotJSON, &v.Fingerprint,
		&v.SupersededBy, &created, &v.Version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	v.CreatedAt, _ = parseTime(created)
	return &v, nil
}
