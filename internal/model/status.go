package model

// 全量状态常量与流转校验：状态机是本题的核心不变量。

// 研究批次状态。
const (
	BatchDraft     = "draft"     // 整理中
	BatchAligning  = "aligning"  // 待对齐
	BatchReviewing = "reviewing" // 待复核
	BatchPublished = "published" // 已发布
	BatchSealed    = "sealed"    // 封存
)

// 证据片段状态。
const (
	SegmentRaw       = "raw"       // 原始
	SegmentAligned   = "aligned"   // 已对齐
	SegmentAmbiguous = "ambiguous" // 转录歧义
	SegmentExcluded  = "excluded"  // 排除
)

// 调弦关系状态。
const (
	RelationCandidate        = "candidate"         // 候选
	RelationFeasible         = "feasible"          // 可行
	RelationIntervalConflict = "interval_conflict" // 音程冲突
	RelationStringConflict   = "string_conflict"   // 弦位冲突
	RelationConfirmed        = "confirmed"         // 确认
	RelationRejected         = "rejected"          // 否决
)

// 调弦版本状态。
const (
	VersionDraft      = "draft"      // 草稿
	VersionShared     = "shared"     // 共享
	VersionFrozen     = "frozen"     // 冻结
	VersionSuperseded = "superseded" // 替代
)

// 变体候选状态。
const (
	VariantPending  = "pending"  // 待裁决
	VariantAccepted = "accepted" // 采纳
	VariantDeclined = "declined" // 否决
)

var batchTransitions = map[string][]string{
	BatchDraft:     {BatchAligning, BatchSealed},
	BatchAligning:  {BatchReviewing, BatchDraft, BatchSealed},
	BatchReviewing: {BatchPublished, BatchAligning, BatchSealed},
	BatchPublished: {BatchSealed},
	BatchSealed:    {},
}

var segmentTransitions = map[string][]string{
	SegmentRaw:       {SegmentAligned, SegmentAmbiguous, SegmentExcluded},
	SegmentAligned:   {SegmentAmbiguous, SegmentExcluded},
	SegmentAmbiguous: {SegmentAligned, SegmentExcluded},
	SegmentExcluded:  {},
}

var relationTransitions = map[string][]string{
	RelationCandidate:        {RelationFeasible, RelationIntervalConflict, RelationStringConflict},
	RelationFeasible:         {RelationConfirmed, RelationRejected, RelationCandidate},
	RelationIntervalConflict: {RelationConfirmed, RelationRejected, RelationCandidate},
	RelationStringConflict:   {RelationConfirmed, RelationRejected, RelationCandidate},
	RelationConfirmed:        {RelationRejected},
	RelationRejected:         {},
}

var versionTransitions = map[string][]string{
	VersionDraft:      {VersionShared, VersionFrozen},
	VersionShared:     {VersionFrozen, VersionDraft},
	VersionFrozen:     {VersionSuperseded},
	VersionSuperseded: {},
}

// ValidBatchTransition 判断批次状态流转是否合法。
func ValidBatchTransition(from, to string) bool { return contains(batchTransitions[from], to) }

// ValidSegmentTransition 判断片段状态流转是否合法。
func ValidSegmentTransition(from, to string) bool { return contains(segmentTransitions[from], to) }

// ValidRelationTransition 判断调弦关系状态流转是否合法。
func ValidRelationTransition(from, to string) bool { return contains(relationTransitions[from], to) }

// ValidVersionTransition 判断版本状态流转是否合法。
func ValidVersionTransition(from, to string) bool { return contains(versionTransitions[from], to) }

// BatchClosed 批次是否封存（封存后一切子实体禁止修改）。
func BatchClosed(status string) bool { return status == BatchSealed }

// RelationTerminal 调弦关系是否已到终态（确认/否决）。
func RelationTerminal(status string) bool { return status == RelationConfirmed || status == RelationRejected }

// VersionImmutable 版本是否不可变（冻结/替代后禁止修改）。
func VersionImmutable(status string) bool { return status == VersionFrozen || status == VersionSuperseded }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
