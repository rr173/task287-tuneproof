package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task287-tuneproof/internal/model"
)

// MappingStore 术语映射存取。
type MappingStore struct{ db *DB }

// NewMappingStore 构造术语映射存储。
func NewMappingStore(db *DB) *MappingStore { return &MappingStore{db: db} }

// Upsert 写入术语映射：(source_term, region) 唯一，冲突时更新。
func (s *MappingStore) Upsert(m *model.TermMapping) (int64, error) {
	if m.SourceTerm == "" || m.Normalized == "" {
		return 0, model.NewValidationError("term", "source and normalized required")
	}
	if m.IntervalCents < 0 {
		return 0, model.NewValidationError("interval_cents", "must be >= 0")
	}
	if m.Confidence <= 0 || m.Confidence > 1 {
		return 0, model.NewValidationError("confidence", "must be in (0,1]")
	}
	res, err := s.db.Exec(
		`INSERT INTO term_mappings(source_term, region, normalized, interval_cents, unit, confidence, notes, created_at)
		 VALUES(?,?,?,?,?,?,?,?)
		 ON CONFLICT(source_term, region) DO UPDATE SET
		   normalized=excluded.normalized, interval_cents=excluded.interval_cents,
		   unit=excluded.unit, confidence=excluded.confidence, notes=excluded.notes`,
		m.SourceTerm, m.Region, m.Normalized, m.IntervalCents, m.Unit, m.Confidence, m.Notes, Now())
	if err != nil {
		return 0, fmt.Errorf("upsert mapping: %w", err)
	}
	return res.LastInsertId()
}

// Get 按 (source_term, region) 查映射。
func (s *MappingStore) Get(sourceTerm, region string) (*model.TermMapping, error) {
	row := s.db.QueryRow(
		`SELECT id, source_term, region, normalized, interval_cents, unit, confidence, notes, created_at
		 FROM term_mappings WHERE source_term = ? AND region = ?`, sourceTerm, region)
	return scanMapping(row)
}

// List 列出全部映射（可按地区过滤）。
func (s *MappingStore) List(region string) ([]*model.TermMapping, error) {
	q := `SELECT id, source_term, region, normalized, interval_cents, unit, confidence, notes, created_at
		  FROM term_mappings`
	args := []any{}
	if region != "" {
		q += ` WHERE region = ?`
		args = append(args, region)
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	defer rows.Close()
	var out []*model.TermMapping
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMapping(sc interface{ Scan(...any) error }) (*model.TermMapping, error) {
	var m model.TermMapping
	var created string
	if err := sc.Scan(&m.ID, &m.SourceTerm, &m.Region, &m.Normalized, &m.IntervalCents,
		&m.Unit, &m.Confidence, &m.Notes, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	m.CreatedAt, _ = parseTime(created)
	return &m, nil
}
