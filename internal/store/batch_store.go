package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task287-tuneproof/internal/model"
)

// BatchStore 研究批次存取。
type BatchStore struct{ db *DB }

// NewBatchStore 构造批次存储。
func NewBatchStore(db *DB) *BatchStore { return &BatchStore{db: db} }

// Create 创建批次，初始状态 draft。
func (s *BatchStore) Create(b *model.ResearchBatch) (int64, error) {
	now := Now()
	res, err := s.db.Exec(
		`INSERT INTO research_batches(name, description, region, status, created_at, updated_at, version)
		 VALUES(?,?,?,?,?,?,1)`,
		b.Name, b.Description, b.Region, model.BatchDraft, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert batch: %w", err)
	}
	return res.LastInsertId()
}

// Get 按 ID 查询批次。
func (s *BatchStore) Get(id int64) (*model.ResearchBatch, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, region, status, created_at, updated_at, version
		 FROM research_batches WHERE id = ?`, id)
	return scanBatch(row)
}

// List 列出全部批次，按创建时间倒序。
func (s *BatchStore) List() ([]*model.ResearchBatch, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, region, status, created_at, updated_at, version
		 FROM research_batches ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	defer rows.Close()
	var out []*model.ResearchBatch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateStatus 乐观锁推进批次状态：version 不匹配返回 ErrConflict。
func (s *BatchStore) UpdateStatus(id int64, status string, expectedVersion int64) (*model.ResearchBatch, error) {
	now := Now()
	res, err := s.db.Exec(
		`UPDATE research_batches SET status = ?, updated_at = ?, version = version + 1
		 WHERE id = ? AND version = ?`,
		status, now, id, expectedVersion)
	if err != nil {
		return nil, fmt.Errorf("update batch status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		existing, err := s.Get(id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if model.BatchClosed(existing.Status) {
			return existing, nil
		}
		return nil, model.ErrConflict
	}
	return s.Get(id)
}

// Touch 无状态变更的更新时间戳刷新（如添加子实体时）。
func (s *BatchStore) Touch(id int64) error {
	_, err := s.db.Exec(`UPDATE research_batches SET updated_at = ? WHERE id = ?`, Now(), id)
	return err
}

func scanBatch(sc interface{ Scan(...any) error }) (*model.ResearchBatch, error) {
	var b model.ResearchBatch
	var created, updated string
	if err := sc.Scan(&b.ID, &b.Name, &b.Description, &b.Region, &b.Status, &created, &updated, &b.Version); err != nil {
		return nil, err
	}
	b.CreatedAt, _ = parseTime(created)
	b.UpdatedAt, _ = parseTime(updated)
	return &b, nil
}
