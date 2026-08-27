package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task287-tuneproof/internal/model"
)

// RelationStore 调弦关系与变体候选存取。
type RelationStore struct{ db *DB }

// NewRelationStore 构造关系存储。
func NewRelationStore(db *DB) *RelationStore { return &RelationStore{db: db} }

// Create 创建调弦关系，初始状态 candidate。
func (s *RelationStore) Create(r *model.TuningRelation) (int64, error) {
	if r.FromPosition == r.ToPosition {
		return 0, model.ErrRelation // 拒绝自配对
	}
	if r.FromPosition <= 0 || r.ToPosition <= 0 {
		return 0, model.NewValidationError("position", "must be positive")
	}
	now := Now()
	res, err := s.db.Exec(
		`INSERT INTO tuning_relations(batch_id, instrument_id, segment_id, from_position, to_position,
		   described_term, described_interval, measured_interval, delta_cents, status, verdict_reason, version, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,0,0,?,?,1,?,?)`,
		r.BatchID, r.InstrumentID, r.SegmentID, r.FromPosition, r.ToPosition,
		r.DescribedTerm, r.DescribedInterval, model.RelationCandidate, "", now, now)
	if err != nil {
		return 0, fmt.Errorf("insert relation: %w", err)
	}
	return res.LastInsertId()
}

// Get 按 ID 查询关系。
func (s *RelationStore) Get(id int64) (*model.TuningRelation, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_id, instrument_id, segment_id, from_position, to_position,
		   described_term, described_interval, measured_interval, delta_cents, status, verdict_reason, version, created_at, updated_at
		 FROM tuning_relations WHERE id = ?`, id)
	return scanRelation(row)
}

// List 按批次列出关系（可按状态过滤）。
func (s *RelationStore) List(batchID int64, status string) ([]*model.TuningRelation, error) {
	q := `SELECT id, batch_id, instrument_id, segment_id, from_position, to_position,
		   described_term, described_interval, measured_interval, delta_cents, status, verdict_reason, version, created_at, updated_at
		  FROM tuning_relations WHERE batch_id = ?`
	args := []any{batchID}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()
	var out []*model.TuningRelation
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateCheckResult 写入检查结果（音程/弦位裁决），乐观锁版本校验。
func (s *RelationStore) UpdateCheckResult(id int64, measured, delta int, status, reason string, expectedVersion int64) (*model.TuningRelation, error) {
	res, err := s.db.Exec(
		`UPDATE tuning_relations SET measured_interval = ?, delta_cents = ?, status = ?, verdict_reason = ?,
		   version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ?`,
		measured, delta, status, reason, Now(), id, expectedVersion)
	if err != nil {
		return nil, fmt.Errorf("update check result: %w", err)
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

// Adjudicate 研究者裁决（确认/否决），乐观锁版本校验。
func (s *RelationStore) Adjudicate(id int64, status string, expectedVersion int64) (*model.TuningRelation, error) {
	res, err := s.db.Exec(
		`UPDATE tuning_relations SET status = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ?`,
		status, Now(), id, expectedVersion)
	if err != nil {
		return nil, fmt.Errorf("adjudicate relation: %w", err)
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

// AddVariant 添加变体候选。
func (s *RelationStore) AddVariant(v *model.VariantCandidate) (int64, error) {
	if v.Region == "" || v.Description == "" {
		return 0, model.NewValidationError("variant", "region and description required")
	}
	res, err := s.db.Exec(
		`INSERT INTO variant_candidates(relation_id, region, description, evidence_ref, status, created_at)
		 VALUES(?,?,?,?,?,?)`,
		v.RelationID, v.Region, v.Description, v.EvidenceRef, model.VariantPending, Now())
	if err != nil {
		return 0, fmt.Errorf("insert variant: %w", err)
	}
	return res.LastInsertId()
}

// ListVariants 列出变体候选（按关系或全局）。
func (s *RelationStore) ListVariants(relationID int64) ([]*model.VariantCandidate, error) {
	q := `SELECT id, relation_id, region, description, evidence_ref, status, created_at FROM variant_candidates`
	args := []any{}
	if relationID > 0 {
		q += ` WHERE relation_id = ?`
		args = append(args, relationID)
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()
	var out []*model.VariantCandidate
	for rows.Next() {
		var v model.VariantCandidate
		var created string
		if err := rows.Scan(&v.ID, &v.RelationID, &v.Region, &v.Description, &v.EvidenceRef, &v.Status, &created); err != nil {
			return nil, err
		}
		v.CreatedAt, _ = parseTime(created)
		out = append(out, &v)
	}
	return out, rows.Err()
}

// AdjudicateVariant 裁决变体候选（采纳/否决）。
func (s *RelationStore) AdjudicateVariant(id int64, status string) error {
	res, err := s.db.Exec(`UPDATE variant_candidates SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("adjudicate variant: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

func scanRelation(sc interface{ Scan(...any) error }) (*model.TuningRelation, error) {
	var r model.TuningRelation
	var created, updated string
	if err := sc.Scan(&r.ID, &r.BatchID, &r.InstrumentID, &r.SegmentID, &r.FromPosition, &r.ToPosition,
		&r.DescribedTerm, &r.DescribedInterval, &r.MeasuredInterval, &r.DeltaCents,
		&r.Status, &r.VerdictReason, &r.Version, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	r.CreatedAt, _ = parseTime(created)
	r.UpdatedAt, _ = parseTime(updated)
	return &r, nil
}
