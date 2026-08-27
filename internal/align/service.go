package align

import (
	"strings"

	"task287-tuneproof/internal/model"
)

// Service 术语对齐服务：解析口述片段为结构化调弦声明。
type Service struct {
	mappings MappingStore
	audit    AuditStore
}

// NewService 构造对齐服务。
func NewService(mappings MappingStore, audit AuditStore) *Service {
	return &Service{mappings: mappings, audit: audit}
}

// MappingStore 术语映射存储接口。
type MappingStore interface {
	Upsert(*model.TermMapping) (int64, error)
	Get(string, string) (*model.TermMapping, error)
	List(string) ([]*model.TermMapping, error)
}

// AuditStore 审计接口。
type AuditStore interface {
	Log(int64, string, string, int64, string) error
}

// AlignedClaim 对齐后的调弦声明：从口述中解析出的结构化描述。
type AlignedClaim struct {
	Phrase         string  `json:"phrase"`          // 原始描述
	Normalized     string  `json:"normalized"`      // 归一化动作
	IntervalCents  int     `json:"interval_cents"`  // 音程（音分）
	Confidence     float64 `json:"confidence"`      // 对齐置信度
	RegionVariant  string  `json:"region_variant"`  // 命中的地区变体
	MappingMatched bool    `json:"mapping_matched"` // 是否命中已有术语映射
}

// AlignTranscript 对齐一段口述转录：
// 1. 词典/地区变体匹配；2. 标准音程解析；3. 术语映射表复核。
// 返回对齐声明；若音程无法解析则 ok=false（转录歧义）。
func (s *Service) AlignTranscript(transcript, region string) (AlignedClaim, bool, error) {
	phrase, term, ok := ExtractTuningClaims(transcript)
	if !ok {
		return AlignedClaim{}, false, nil
	}
	claim := AlignedClaim{Phrase: phrase, RegionVariant: region}

	// 1. 地区变体词典匹配（地区动作语义，如"高弦下调"）
	if t, found := Lookup(phrase, region); found {
		claim.Normalized = t.Normalized
		claim.IntervalCents = t.Interval
		claim.Confidence = t.Confidence
		claim.RegionVariant = t.Region
	} else {
		// 2. 标准音程解析（如"纯四度"）
		it, found, conf := model.FuzzyResolveIntervalTerm(term)
		if !found {
			return AlignedClaim{}, false, nil
		}
		claim.Normalized = it.Token
		claim.IntervalCents = it.Cents
		claim.Confidence = conf
	}

	// 3. 术语映射表复核：若有地区特化映射则采用
	if m, err := s.mappings.Get(term, region); err == nil {
		claim.IntervalCents = m.IntervalCents
		claim.Confidence = m.Confidence
		claim.Normalized = m.Normalized
		claim.MappingMatched = true
	} else if m, err := s.mappings.Get(term, ""); err == nil {
		claim.IntervalCents = m.IntervalCents
		claim.Confidence = m.Confidence
		claim.Normalized = m.Normalized
		claim.MappingMatched = true
	}
	return claim, true, nil
}

// UpsertMapping 录入/更新术语映射。
func (s *Service) UpsertMapping(m *model.TermMapping) (int64, error) {
	id, err := s.mappings.Upsert(m)
	if err != nil {
		return 0, err
	}
	_ = s.audit.Log(0, "mapping.upsert", "term_mapping", id, m.SourceTerm+"->"+m.Normalized)
	return id, nil
}

// ListMappings 列术语映射。
func (s *Service) ListMappings(region string) ([]*model.TermMapping, error) {
	return s.mappings.List(region)
}

// ParseStringPositions 从弦位指称解析位置号（"高弦"→position, "低弦"→position）。
// 返回 (position, ok)。解析依赖调用方传入的弦数：
// 高弦=最高编号，低弦=1，中弦=中间。
func ParseStringPositions(desc string, stringCount int) (int, bool) {
	d := strings.TrimSpace(desc)
	if d == "" || stringCount <= 0 {
		return 0, false
	}
	switch {
	case strings.Contains(d, "高"):
		return stringCount, true // 高弦 = 最高编号
	case strings.Contains(d, "低"):
		return 1, true
	case strings.Contains(d, "中"):
		return (stringCount + 1) / 2, true
	case strings.Contains(d, "一"):
		return 1, true
	case strings.Contains(d, "二"):
		return 2, true
	case strings.Contains(d, "三"):
		return 3, true
	case strings.Contains(d, "四"):
		return 4, true
	case strings.Contains(d, "五"):
		return 5, true
	case strings.Contains(d, "六"):
		return 6, true
	}
	return 0, false
}
