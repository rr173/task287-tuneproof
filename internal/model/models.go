// Package model 定义民族乐器调弦口述史证据复核台的核心实体。
package model

import "time"

// ResearchBatch 研究批次：一次围绕某个民族乐器的调弦口述史证据复核。
// 状态机：draft(整理中) -> aligning(待对齐) -> reviewing(待复核) -> published(已发布) -> sealed(封存)。
type ResearchBatch struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Region      string    `json:"region"` // 主研究地区
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int64     `json:"version"` // 乐观锁版本号
}

// Instrument 乐器：承载弦位结构，调弦关系围绕乐器展开。
type Instrument struct {
	ID          int64     `json:"id"`
	BatchID     int64     `json:"batch_id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"` // 例如 弹拨/弓弦/击弦
	Region      string    `json:"region"`
	StringCount int       `json:"string_count"`
	CreatedAt   time.Time `json:"created_at"`
	Version     int64     `json:"version"`
}

// StringPosition 弦位：一根弦的物理约束（可调频率范围与标准音高）。
// 不变量：同一乐器内 position 唯一；min <= standard <= max。
type StringPosition struct {
	ID           int64     `json:"id"`
	InstrumentID int64     `json:"instrument_id"`
	Position     int       `json:"position"` // 从 1 开始
	Name         string    `json:"name"`
	MinFreqHz    float64   `json:"min_freq_hz"`
	MaxFreqHz    float64   `json:"max_freq_hz"`
	StandardHz   float64   `json:"standard_hz"`
	CreatedAt    time.Time `json:"created_at"`
}

// EvidenceSegment 证据片段：口述调弦说明的一段转录，可附录音音高观测。
// 状态机：raw(原始) -> aligned(已对齐) | ambiguous(转录歧义) -> excluded(排除)。
type EvidenceSegment struct {
	ID          int64     `json:"id"`
	BatchID     int64     `json:"batch_id"`
	SourceType  string    `json:"source_type"` // oral_history/field_note/treatise/recording_meta
	SourceRef   string    `json:"source_ref"`  // 出处引用（档案编号/访谈编号）
	Region      string    `json:"region"`      // 口述者地区
	Transcript  string    `json:"transcript"`  // 转录文本
	Status      string    `json:"status"`
	Fingerprint string    `json:"fingerprint"` // SHA-256 内容指纹，幂等导入依据
	CreatedAt   time.Time `json:"created_at"`
	Version     int64     `json:"version"`
}

// PitchObservation 音高观测：录音中某一弦位的实测音高。
type PitchObservation struct {
	ID          int64     `json:"id"`
	SegmentID   int64     `json:"segment_id"`
	StringPos   int       `json:"string_pos"`
	FrequencyHz float64   `json:"frequency_hz"`
	Unit        string    `json:"unit"` // hz / cents_from_a4
	Confidence  float64   `json:"confidence"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// TermMapping 术语映射：把某地区的口述调弦术语归一化为标准音程。
type TermMapping struct {
	ID            int64     `json:"id"`
	SourceTerm    string    `json:"source_term"`
	Region        string    `json:"region"`
	Normalized    string    `json:"normalized"` // 标准音程 token，如 perfect_fourth
	IntervalCents int       `json:"interval_cents"`
	Unit          string    `json:"unit"` // interval / frequency
	Confidence    float64   `json:"confidence"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
}

// TuningRelation 调弦关系：口述中"弦 from 比弦 to 高/低某音程"的断言。
// 状态机：candidate -> feasible | interval_conflict | string_conflict -> confirmed | rejected。
type TuningRelation struct {
	ID                 int64     `json:"id"`
	BatchID            int64     `json:"batch_id"`
	InstrumentID       int64     `json:"instrument_id"`
	SegmentID          int64     `json:"segment_id"`
	FromPosition       int       `json:"from_position"`
	ToPosition         int       `json:"to_position"`
	DescribedTerm      string    `json:"described_term"`       // 口述中的音程描述，如 "纯四度"
	DescribedInterval  int       `json:"described_interval_cents"` // 归一化后的描述音程
	MeasuredInterval   int       `json:"measured_interval_cents"`  // 录音实测音程（无观测时为 0）
	DeltaCents         int       `json:"delta_cents"`              // 实测-描述差值
	Status             string    `json:"status"`
	VerdictReason      string    `json:"verdict_reason"`
	Version            int64     `json:"version"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// VariantCandidate 地区变体候选：同一调弦在不同地区的口述差异。
type VariantCandidate struct {
	ID          int64     `json:"id"`
	RelationID  int64     `json:"relation_id"`
	Region      string    `json:"region"`
	Description string    `json:"description"`
	EvidenceRef string    `json:"evidence_ref"`
	Status      string    `json:"status"` // pending/accepted/declined
	CreatedAt   time.Time `json:"created_at"`
}

// TuningVersion 调弦版本：冻结后的不可变证据快照。
// 状态机：draft -> shared -> frozen -> superseded。
type TuningVersion struct {
	ID           int64     `json:"id"`
	BatchID      int64     `json:"batch_id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	SnapshotJSON string    `json:"snapshot_json"`
	Fingerprint  string    `json:"fingerprint"`
	SupersededBy int64     `json:"superseded_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Version      int64     `json:"version"`
}

// AuditLog 审计记录：关键操作留痕，保证证据链可追溯。
type AuditLog struct {
	ID        int64     `json:"id"`
	BatchID   int64     `json:"batch_id"`
	Action    string    `json:"action"`
	Entity    string    `json:"entity"`
	EntityID  int64     `json:"entity_id"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// CheckReport 检查报告：一次可行性检查的完整结论。
type CheckReport struct {
	RelationID     int64    `json:"relation_id"`
	DescribedCents int      `json:"described_cents"`
	MeasuredCents  int      `json:"measured_cents"`
	DeltaCents     int      `json:"delta_cents"`
	Verdict        string   `json:"verdict"`
	Reasons        []string `json:"reasons"`
}
