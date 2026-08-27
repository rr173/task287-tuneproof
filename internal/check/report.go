// Package check 检查模块：验证调弦关系的物理可行性与证据一致性。
package check

import "task287-tuneproof/internal/model"

// Report 一次可行性检查的完整报告。
type Report struct {
	RelationID       int64    `json:"relation_id"`
	InstrumentID     int64    `json:"instrument_id"`
	SegmentID        int64    `json:"segment_id"`
	FromPosition     int      `json:"from_position"`
	ToPosition       int      `json:"to_position"`
	DescribedTerm    string   `json:"described_term"`
	DescribedCents   int      `json:"described_cents"`
	MeasuredCents    int      `json:"measured_cents"`
	DeltaCents       int      `json:"delta_cents"`
	ToleranceCents   int      `json:"tolerance_cents"`
	Verdict          string   `json:"verdict"` // feasible / interval_conflict / string_conflict / insufficient_evidence
	VerdictReason    string   `json:"verdict_reason"`
	Reasons          []string `json:"reasons"` // 逐条检查结论
	Frozen           bool     `json:"frozen"`  // 是否冻结版本约束
}

// Feasible 是否通过（可行）。
func (r *Report) Feasible() bool { return r.Verdict == model.RelationFeasible }

// Conflict 是否冲突。
func (r *Report) Conflict() bool {
	return r.Verdict == model.RelationIntervalConflict || r.Verdict == model.RelationStringConflict
}
