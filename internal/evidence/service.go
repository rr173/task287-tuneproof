// Package evidence 证据模块：保存口述片段与音高观测，负责指纹幂等导入。
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"task287-tuneproof/internal/model"
)

// Service 证据服务。
type Service struct {
	segments SegmentStore
	batches  BatchStore
	audit    AuditStore
}

// NewService 构造证据服务。
func NewService(segments SegmentStore, batches BatchStore, audit AuditStore) *Service {
	return &Service{segments: segments, batches: batches, audit: audit}
}

// SegmentStore 片段存储接口（由 store 包实现）。
type SegmentStore interface {
	Create(*model.EvidenceSegment) (int64, error)
	Get(int64) (*model.EvidenceSegment, error)
	GetByFingerprint(string) (*model.EvidenceSegment, error)
	List(int64, string) ([]*model.EvidenceSegment, error)
	UpdateStatus(int64, string, int64) (*model.EvidenceSegment, error)
	AddPitch(*model.PitchObservation) (int64, error)
	ListPitches(int64) ([]*model.PitchObservation, error)
	PitchFor(int64, int) (*model.PitchObservation, error)
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

// ImportInput 片段导入入参。
type ImportInput struct {
	BatchID    int64  `json:"batch_id"`
	SourceType string `json:"source_type"`
	SourceRef  string `json:"source_ref"`
	Region     string `json:"region"`
	Transcript string `json:"transcript"`
}

// ImportSegment 幂等导入口述片段：同一内容指纹只导入一次，重复返回既有片段。
func (s *Service) ImportSegment(in ImportInput) (*model.EvidenceSegment, bool, error) {
	batch, err := s.batches.Get(in.BatchID)
	if err != nil {
		return nil, false, err
	}
	if model.BatchClosed(batch.Status) {
		return nil, false, model.ErrFrozen
	}
	if strings.TrimSpace(in.Transcript) == "" {
		return nil, false, model.NewValidationError("transcript", "required")
	}
	if in.SourceType == "" {
		in.SourceType = "oral_history"
	}
	fp := Fingerprint(in.SourceType, in.SourceRef, in.Transcript)
	if existing, err := s.segments.GetByFingerprint(fp); err == nil {
		return existing, false, nil // 幂等：已存在
	}
	seg := &model.EvidenceSegment{
		BatchID:     in.BatchID,
		SourceType:  in.SourceType,
		SourceRef:   in.SourceRef,
		Region:      in.Region,
		Transcript:  in.Transcript,
		Status:      model.SegmentRaw,
		Fingerprint: fp,
	}
	id, err := s.segments.Create(seg)
	if err != nil {
		return nil, false, err
	}
	_ = s.audit.Log(in.BatchID, "segment.import", "evidence_segment", id, fp)
	created, err := s.segments.Get(id)
	return created, true, err
}

// AttachPitch 给片段附加音高观测：片段必须未排除且批次未封存。
func (s *Service) AttachPitch(ctx context.Context, p *model.PitchObservation) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	seg, err := s.segments.Get(p.SegmentID)
	if err != nil {
		return 0, err
	}
	if seg.Status == model.SegmentExcluded {
		return 0, model.ErrFrozen
	}
	batch, err := s.batches.Get(seg.BatchID)
	if err != nil {
		return 0, err
	}
	if model.BatchClosed(batch.Status) {
		return 0, model.ErrFrozen
	}
	return s.segments.AddPitch(p)
}

// Get 查片段详情（含音高观测）。
func (s *Service) Get(id int64) (*model.EvidenceSegment, []*model.PitchObservation, error) {
	seg, err := s.segments.Get(id)
	if err != nil {
		return nil, nil, err
	}
	pitches, err := s.segments.ListPitches(id)
	if err != nil {
		return nil, nil, err
	}
	return seg, pitches, nil
}

// List 列片段。
func (s *Service) List(batchID int64, status string) ([]*model.EvidenceSegment, error) {
	return s.segments.List(batchID, status)
}

// SetStatus 变更片段状态（原始/已对齐/转录歧义/排除），乐观锁。
func (s *Service) SetStatus(id int64, status string, expectedVersion int64) (*model.EvidenceSegment, error) {
	seg, err := s.segments.Get(id)
	if err != nil {
		return nil, err
	}
	if !model.ValidSegmentTransition(seg.Status, status) {
		return nil, model.ErrInvalidState
	}
	batch, err := s.batches.Get(seg.BatchID)
	if err != nil {
		return nil, err
	}
	if model.BatchClosed(batch.Status) {
		return nil, model.ErrFrozen
	}
	updated, err := s.segments.UpdateStatus(id, status, expectedVersion)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(seg.BatchID, "segment.status", "evidence_segment", id, fmt.Sprintf("%s -> %s", seg.Status, status))
	return updated, nil
}

// Fingerprint 计算片段指纹：来源类型+出处+转录的 SHA-256。
func Fingerprint(sourceType, sourceRef, transcript string) string {
	h := sha256.New()
	h.Write([]byte(sourceType))
	h.Write([]byte{0})
	h.Write([]byte(sourceRef))
	h.Write([]byte{0})
	h.Write([]byte(transcript))
	return hex.EncodeToString(h.Sum(nil))
}
