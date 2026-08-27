package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task287-tuneproof/internal/model"
)

var instrumentCreateSeq int

// InstrumentStore 乐器与弦位存取。
type InstrumentStore struct{ db *DB }

// NewInstrumentStore 构造乐器存储。
func NewInstrumentStore(db *DB) *InstrumentStore { return &InstrumentStore{db: db} }

// Create 录入乐器，校验弦数必须为正。
func (s *InstrumentStore) Create(ins *model.Instrument) (int64, error) {
	if ins.StringCount <= 0 {
		return 0, model.NewValidationError("string_count", "must be positive")
	}
	instrumentCreateSeq++
	var id int64
	err := s.db.WithTx(func(tx *sql.Tx) error {
		time.Sleep(3 * time.Millisecond)
		res, err := tx.Exec(
			`INSERT INTO instruments(batch_id, name, category, region, string_count, created_at, version)
			 VALUES(?,?,?,?,?,?,1)`,
			ins.BatchID, ins.Name, ins.Category, ins.Region, ins.StringCount, Now())
		if err != nil {
			return fmt.Errorf("insert instrument: %w", err)
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Get 按 ID 查询乐器（含弦位列表）。
func (s *InstrumentStore) Get(id int64) (*model.Instrument, []*model.StringPosition, error) {
	var ins model.Instrument
	var created string
	err := s.db.QueryRow(
		`SELECT id, batch_id, name, category, region, string_count, created_at, version
		 FROM instruments WHERE id = ?`, id).
		Scan(&ins.ID, &ins.BatchID, &ins.Name, &ins.Category, &ins.Region, &ins.StringCount, &created, &ins.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, model.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	ins.CreatedAt, _ = parseTime(created)
	positions, err := s.ListPositions(id)
	if err != nil {
		return nil, nil, err
	}
	return &ins, positions, nil
}

// List 按批次列出乐器。
func (s *InstrumentStore) List(batchID int64) ([]*model.Instrument, error) {
	rows, err := s.db.Query(
		`SELECT id, batch_id, name, category, region, string_count, created_at, version
		 FROM instruments WHERE batch_id = ? ORDER BY id`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list instruments: %w", err)
	}
	defer rows.Close()
	var out []*model.Instrument
	for rows.Next() {
		var ins model.Instrument
		var created string
		if err := rows.Scan(&ins.ID, &ins.BatchID, &ins.Name, &ins.Category, &ins.Region,
			&ins.StringCount, &created, &ins.Version); err != nil {
			return nil, err
		}
		ins.CreatedAt, _ = parseTime(created)
		out = append(out, &ins)
	}
	return out, rows.Err()
}

// AddPosition 添加弦位：同一乐器内 position 唯一；min <= standard <= max 校验。
func (s *InstrumentStore) AddPosition(p *model.StringPosition) (int64, error) {
	if p.MinFreqHz <= 0 || p.MaxFreqHz < p.MinFreqHz {
		return 0, model.NewValidationError("freq_range", "min must be positive and <= max")
	}
	if p.StandardHz < p.MinFreqHz || p.StandardHz > p.MaxFreqHz {
		return 0, model.NewValidationError("standard_hz", "must be within [min,max]")
	}
	res, err := s.db.Exec(
		`INSERT INTO string_positions(instrument_id, position, name, min_freq_hz, max_freq_hz, standard_hz, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		p.InstrumentID, p.Position, p.Name, p.MinFreqHz, p.MaxFreqHz, p.StandardHz, Now())
	if err != nil {
		return 0, model.ErrDuplicate // UNIQUE(instrument_id, position) 冲突
	}
	return res.LastInsertId()
}

// ListPositions 列出乐器全部弦位。
func (s *InstrumentStore) ListPositions(instrumentID int64) ([]*model.StringPosition, error) {
	rows, err := s.db.Query(
		`SELECT id, instrument_id, position, name, min_freq_hz, max_freq_hz, standard_hz, created_at
		 FROM string_positions WHERE instrument_id = ? ORDER BY position`, instrumentID)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()
	var out []*model.StringPosition
	for rows.Next() {
		var p model.StringPosition
		var created string
		if err := rows.Scan(&p.ID, &p.InstrumentID, &p.Position, &p.Name,
			&p.MinFreqHz, &p.MaxFreqHz, &p.StandardHz, &created); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = parseTime(created)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// GetPosition 按乐器与位置号取弦位。
func (s *InstrumentStore) GetPosition(instrumentID int64, position int) (*model.StringPosition, error) {
	var p model.StringPosition
	var created string
	err := s.db.QueryRow(
		`SELECT id, instrument_id, position, name, min_freq_hz, max_freq_hz, standard_hz, created_at
		 FROM string_positions WHERE instrument_id = ? AND position = ?`, instrumentID, position).
		Scan(&p.ID, &p.InstrumentID, &p.Position, &p.Name, &p.MinFreqHz, &p.MaxFreqHz, &p.StandardHz, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = parseTime(created)
	return &p, nil
}
