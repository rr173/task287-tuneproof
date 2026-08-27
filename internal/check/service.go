package check

import (
	"fmt"
	"strings"

	"task287-tuneproof/internal/model"
)

// Service 检查服务：编排音程比对与弦位约束，输出可行性裁决。
type Service struct {
	relations   RelationStore
	segments    SegmentStore
	instruments InstrumentStore
	batches     BatchStore
	audit       AuditStore
	interval    *IntervalCheck
}

// NewService 构造检查服务。
func NewService(relations RelationStore, segments SegmentStore,
	instruments InstrumentStore, batches BatchStore, audit AuditStore) *Service {
	return &Service{
		relations:   relations,
		segments:    segments,
		instruments: instruments,
		batches:     batches,
		audit:       audit,
		interval:    NewIntervalCheck(model.DefaultToleranceCents),
	}
}

// RelationStore 关系存储接口。
type RelationStore interface {
	Get(int64) (*model.TuningRelation, error)
	UpdateCheckResult(int64, int, int, string, string, int64) (*model.TuningRelation, error)
	AddVariant(*model.VariantCandidate) (int64, error)
}

// SegmentStore 片段接口（取音高观测）。
type SegmentStore interface {
	Get(int64) (*model.EvidenceSegment, error)
	ListPitches(int64) ([]*model.PitchObservation, error)
	PitchFor(int64, int) (*model.PitchObservation, error)
}

// InstrumentStore 乐器接口（取弦位）。
type InstrumentStore interface {
	Get(int64) (*model.Instrument, []*model.StringPosition, error)
}

// BatchStore 批次接口。
type BatchStore interface {
	Get(int64) (*model.ResearchBatch, error)
}

// AuditStore 审计接口。
type AuditStore interface {
	Log(int64, string, string, int64, string) error
}

// CheckRelation 对一条调弦关系执行可行性检查：
//  1. 取关系的口述音程（described_interval）；
//  2. 取片段在两个弦位上的实测音高，换算实测音程；
//  3. 弦位约束检查（自配对/越界/超范围）；
//  4. 综合裁决 feasible / interval_conflict / string_conflict。
//
// 裁决结果写回关系并返回报告。检查不会覆盖已有确认/否决裁决。
func (s *Service) CheckRelation(relationID int64) (*Report, error) {
	rel, err := s.relations.Get(relationID)
	if err != nil {
		return nil, err
	}
	if model.RelationTerminal(rel.Status) {
		return nil, model.ErrInvalidState // 终态不可重检
	}
	batch, err := s.batches.Get(rel.BatchID)
	if err != nil {
		return nil, err
	}
	if model.BatchClosed(batch.Status) {
		return &Report{
			RelationID:     rel.ID,
			InstrumentID:   rel.InstrumentID,
			SegmentID:      rel.SegmentID,
			FromPosition:   rel.FromPosition,
			ToPosition:     rel.ToPosition,
			DescribedTerm:  rel.DescribedTerm,
			DescribedCents: rel.DescribedInterval,
			ToleranceCents: s.interval.tolerance,
			Verdict:        model.RelationFeasible,
			VerdictReason:  "batch sealed",
		}, nil
	}
	report := &Report{
		RelationID:     rel.ID,
		InstrumentID:   rel.InstrumentID,
		SegmentID:      rel.SegmentID,
		FromPosition:   rel.FromPosition,
		ToPosition:     rel.ToPosition,
		DescribedTerm:  rel.DescribedTerm,
		DescribedCents: rel.DescribedInterval,
		ToleranceCents: s.interval.tolerance,
	}

	// 音程比对
	fromPitch, errFrom := s.segments.PitchFor(rel.SegmentID, rel.FromPosition)
	toPitch, errTo := s.segments.PitchFor(rel.SegmentID, rel.ToPosition)
	if errFrom != nil || errTo != nil {
		report.Verdict = "insufficient_evidence"
		report.VerdictReason = "缺少任一弦位的实测音高观测"
		report.Reasons = append(report.Reasons, "from/to 弦位至少一个没有音高观测")
		return report, s.persist(rel, report)
	}
	measured, delta, ok := s.interval.Compare(rel.DescribedInterval, fromPitch.FrequencyHz, toPitch.FrequencyHz)
	if !ok {
		report.Verdict = "insufficient_evidence"
		report.VerdictReason = "实测频率非法"
		return report, s.persist(rel, report)
	}
	report.MeasuredCents = measured
	report.DeltaCents = delta
	report.Reasons = append(report.Reasons,
		fmt.Sprintf("描述音程 %s(%d 音分), 实测音程 %s(%d 音分), 偏差 %d 音分",
			rel.DescribedTerm, rel.DescribedInterval, Describe(measured), measured, delta))

	// 弦位约束
	ins, positions, err := s.instruments.Get(rel.InstrumentID)
	if err != nil {
		return nil, err
	}
	sc := NewStringConstraint(positions, ins.StringCount)
	if failures := sc.Check(rel.FromPosition, rel.ToPosition, fromPitch.FrequencyHz, toPitch.FrequencyHz); len(failures) > 0 {
		report.Verdict = model.RelationStringConflict
		report.VerdictReason = strings.Join(failures, "; ")
		report.Reasons = append(report.Reasons, failures...)
		return report, s.persist(rel, report)
	}
	report.Reasons = append(report.Reasons, "弦位与频率范围约束通过")

	// 音程裁决
	report.Verdict = s.interval.Verdict(delta)
	switch report.Verdict {
	case model.RelationFeasible:
		report.VerdictReason = fmt.Sprintf("偏差 %d 音分在容差 %d 音分内", delta, s.interval.tolerance)
	case model.RelationIntervalConflict:
		report.VerdictReason = fmt.Sprintf("偏差 %d 音分超出容差 %d 音分", delta, s.interval.tolerance)
	}
	return report, s.persist(rel, report)
}

// persist 把裁决写回关系（乐观锁）。
func (s *Service) persist(rel *model.TuningRelation, report *Report) error {
	status := report.Verdict
	if report.Verdict == "insufficient_evidence" {
		status = model.RelationCandidate // 证据不足保留候选态，允许补观测后重检
	}
	updated, err := s.relations.UpdateCheckResult(rel.ID, report.MeasuredCents, report.DeltaCents,
		status, report.VerdictReason, rel.Version)
	if err != nil {
		return err
	}
	report.RelationID = updated.ID
	_ = s.audit.Log(rel.BatchID, "relation.check", "tuning_relation", rel.ID, status)
	return nil
}
