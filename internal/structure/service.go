// Package structure 结构模块：维护乐器与弦位构造，约束调弦关系的物理可行性。
package structure

import (
	"task287-tuneproof/internal/model"
)

// Service 乐器结构服务。
type Service struct {
	instruments InstrumentStore
	batches     BatchStore
	audit       AuditStore
}

// NewService 构造结构服务。
func NewService(instruments InstrumentStore, batches BatchStore, audit AuditStore) *Service {
	return &Service{instruments: instruments, batches: batches, audit: audit}
}

// InstrumentStore 乐器存储接口。
type InstrumentStore interface {
	Create(*model.Instrument) (int64, error)
	Get(int64) (*model.Instrument, []*model.StringPosition, error)
	List(int64) ([]*model.Instrument, error)
	AddPosition(*model.StringPosition) (int64, error)
	ListPositions(int64) ([]*model.StringPosition, error)
	GetPosition(int64, int) (*model.StringPosition, error)
}

// BatchStore 批次接口。
type BatchStore interface {
	Get(int64) (*model.ResearchBatch, error)
	Touch(int64) error
}

// AuditStore 审计接口。
type AuditStore interface {
	Log(int64, string, string, int64, string) error
}

// RegisterInstrument 录入乐器：批次必须存在且未封存。
func (s *Service) RegisterInstrument(ins *model.Instrument) (*model.Instrument, []*model.StringPosition, error) {
	batch, err := s.batches.Get(ins.BatchID)
	if err != nil {
		return nil, nil, err
	}
	if model.BatchClosed(batch.Status) {
		return nil, nil, model.ErrFrozen
	}
	id, err := s.instruments.Create(ins)
	if err != nil {
		return nil, nil, err
	}
	_ = s.batches.Touch(ins.BatchID)
	_ = s.audit.Log(ins.BatchID, "instrument.register", "instrument", id, ins.Name)
	return s.instruments.Get(id)
}

// Get 查乐器详情（含弦位）。
func (s *Service) Get(id int64) (*model.Instrument, []*model.StringPosition, error) {
	return s.instruments.Get(id)
}

// List 按批次列乐器。
func (s *Service) List(batchID int64) ([]*model.Instrument, error) {
	return s.instruments.List(batchID)
}

// AddStringPosition 添加弦位：位置唯一、频率范围合法、弦数不超上限。
func (s *Service) AddStringPosition(p *model.StringPosition) ([]*model.StringPosition, error) {
	ins, positions, err := s.instruments.Get(p.InstrumentID)
	if err != nil {
		return nil, err
	}
	batch, err := s.batches.Get(ins.BatchID)
	if err != nil {
		return nil, err
	}
	if model.BatchClosed(batch.Status) {
		return nil, model.ErrFrozen
	}
	// 弦数约束：position 不得超过声明弦数
	if p.Position > ins.StringCount {
		return nil, model.NewValidationError("position", "exceeds string_count")
	}
	// 位置唯一：拒绝重复弦位
	for _, pos := range positions {
		if pos.Position == p.Position {
			return nil, model.ErrDuplicate
		}
	}
	if _, err := s.instruments.AddPosition(p); err != nil {
		return nil, err
	}
	_ = s.batches.Touch(ins.BatchID)
	_ = s.audit.Log(ins.BatchID, "string.add", "string_position", p.InstrumentID, p.Name)
	return s.instruments.ListPositions(p.InstrumentID)
}

// ListPositions 列弦位。
func (s *Service) ListPositions(instrumentID int64) ([]*model.StringPosition, error) {
	return s.instruments.ListPositions(instrumentID)
}

// GetPosition 取指定弦位。
func (s *Service) GetPosition(instrumentID int64, position int) (*model.StringPosition, error) {
	return s.instruments.GetPosition(instrumentID, position)
}
