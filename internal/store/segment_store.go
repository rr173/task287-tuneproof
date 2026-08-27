package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task287-tuneproof/internal/model"
)

// SegmentStore 证据片段与音高观测存取。
type SegmentStore struct{ db *DB }

// NewSegmentStore 构造片段存储。
func NewSegmentStore(db *DB) *SegmentStore { return &SegmentStore{db: db} }

// Create 导入片段：指纹唯一，重复导入返回 ErrDuplicate（幂等由调用方处理）。
func (s *SegmentStore) Create(seg *model.EvidenceSegment) (int64, error) {
	if seg.Fingerprint == "" {
		return 0, model.NewValidationError("fingerprint", "required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(
		`INSERT INTO evidence_segments(batch_id, source_type, source_ref, region, transcript, status, fingerprint, created_at, version)
		 VALUES(?,?,?,?,?,?,?,?,1)`,
		seg.BatchID, seg.SourceType, seg.SourceRef, seg.Region, seg.Transcript, seg.Status, seg.Fingerprint, Now())
	if err != nil {
		return 0, model.ErrDuplicate // UNIQUE(fingerprint)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// Get 按 ID 查询片段。
func (s *SegmentStore) Get(id int64) (*model.EvidenceSegment, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_id, source_type, source_ref, region, transcript, status, fingerprint, created_at, version
		 FROM evidence_segments WHERE id = ?`, id)
	return scanSegment(row)
}

// GetByFingerprint 按指纹查询（幂等导入用）。
func (s *SegmentStore) GetByFingerprint(fp string) (*model.EvidenceSegment, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_id, source_type, source_ref, region, transcript, status, fingerprint, created_at, version
		 FROM evidence_segments WHERE fingerprint = ?`, fp)
	return scanSegment(row)
}

// List 按批次列出片段（可按状态过滤）。
func (s *SegmentStore) List(batchID int64, status string) ([]*model.EvidenceSegment, error) {
	q := `SELECT id, batch_id, source_type, source_ref, region, transcript, status, fingerprint, created_at, version
		  FROM evidence_segments WHERE batch_id = ?`
	args := []any{batchID}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	defer rows.Close()
	var out []*model.EvidenceSegment
	for rows.Next() {
		seg, err := scanSegment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, seg)
	}
	return out, rows.Err()
}

// UpdateStatus 乐观锁更新片段状态。
func (s *SegmentStore) UpdateStatus(id int64, status string, expectedVersion int64) (*model.EvidenceSegment, error) {
	res, err := s.db.Exec(
		`UPDATE evidence_segments SET status = ?, version = version + 1
		 WHERE id = ? AND version = ?`, status, id, expectedVersion)
	if err != nil {
		return nil, fmt.Errorf("update segment status: %w", err)
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

// AddPitch 附加音高观测到片段。
func (s *SegmentStore) AddPitch(p *model.PitchObservation) (int64, error) {
	if p.FrequencyHz <= 0 {
		return 0, model.NewValidationError("frequency_hz", "must be positive")
	}
	if p.StringPos <= 0 {
		return 0, model.NewValidationError("string_pos", "must be positive")
	}
	if p.Confidence <= 0 || p.Confidence > 1 {
		return 0, model.NewValidationError("confidence", "must be in (0,1]")
	}
	res, err := s.db.Exec(
		`INSERT INTO pitch_observations(segment_id, string_pos, frequency_hz, unit, confidence, recorded_at)
		 VALUES(?,?,?,?,?,?)`,
		p.SegmentID, p.StringPos, p.FrequencyHz, p.Unit, p.Confidence, p.RecordedAt.Format("2006-01-02T15:04:05Z07:00"))
	if err != nil {
		return 0, fmt.Errorf("insert pitch: %w", err)
	}
	return res.LastInsertId()
}

// ListPitches 列出片段全部音高观测。
func (s *SegmentStore) ListPitches(segmentID int64) ([]*model.PitchObservation, error) {
	rows, err := s.db.Query(
		`SELECT id, segment_id, string_pos, frequency_hz, unit, confidence, recorded_at
		 FROM pitch_observations WHERE segment_id = ? ORDER BY string_pos`, segmentID)
	if err != nil {
		return nil, fmt.Errorf("list pitches: %w", err)
	}
	defer rows.Close()
	var out []*model.PitchObservation
	for rows.Next() {
		var p model.PitchObservation
		var recorded string
		if err := rows.Scan(&p.ID, &p.SegmentID, &p.StringPos, &p.FrequencyHz, &p.Unit, &p.Confidence, &recorded); err != nil {
			return nil, err
		}
		p.RecordedAt, _ = parseTime(recorded)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// PitchFor 取片段在指定弦位的音高观测（无则 NotFound）。
func (s *SegmentStore) PitchFor(segmentID int64, stringPos int) (*model.PitchObservation, error) {
	var p model.PitchObservation
	var recorded string
	err := s.db.QueryRow(
		`SELECT id, segment_id, string_pos, frequency_hz, unit, confidence, recorded_at
		 FROM pitch_observations WHERE segment_id = ? AND string_pos = ?`, segmentID, stringPos).
		Scan(&p.ID, &p.SegmentID, &p.StringPos, &p.FrequencyHz, &p.Unit, &p.Confidence, &recorded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.RecordedAt, _ = parseTime(recorded)
	return &p, nil
}

func scanSegment(sc interface{ Scan(...any) error }) (*model.EvidenceSegment, error) {
	var s model.EvidenceSegment
	var created string
	if err := sc.Scan(&s.ID, &s.BatchID, &s.SourceType, &s.SourceRef, &s.Region,
		&s.Transcript, &s.Status, &s.Fingerprint, &created, &s.Version); err != nil {
		return nil, err
	}
	s.CreatedAt, _ = parseTime(created)
	return &s, nil
}
