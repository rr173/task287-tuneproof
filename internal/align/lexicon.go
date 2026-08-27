// Package align 对齐模块：把口述片段中的调弦术语解析为标准化音程与音高单位。
package align

import (
	"regexp"
	"strings"

	"task287-tuneproof/internal/model"
)

// Lexicon 地区术语词典：不同地区对同一调弦动作/音程有不同的口述说法。
// 词条同时给出归一化音程与单位，供对齐引擎使用。
var Lexicon = []RegionTerm{
	// 高弦下调：把某根弦调低到与相邻弦构成特定音程
	{Region: "江南", Phrase: "高弦下调", Normalized: "relax_high_string", Interval: 500, Confidence: 0.9},
	{Region: "岭南", Phrase: "松高弦", Normalized: "relax_high_string", Interval: 500, Confidence: 0.85},
	{Region: "西北", Phrase: "把高弦放下来", Normalized: "relax_high_string", Interval: 700, Confidence: 0.8},
	{Region: "江南", Phrase: "低弦绷紧", Normalized: "tighten_low_string", Interval: 500, Confidence: 0.8},
	{Region: "中原", Phrase: "紧低弦", Normalized: "tighten_low_string", Interval: 500, Confidence: 0.85},
	// 中音基准：以中弦为基准对齐
	{Region: "江南", Phrase: "以中弦为宫", Normalized: "reference_middle_string", Interval: 0, Confidence: 0.95},
	{Region: "闽台", Phrase: "中弦定调", Normalized: "reference_middle_string", Interval: 0, Confidence: 0.9},
	// 回松：调回标准音高
	{Region: "闽台", Phrase: "回松", Normalized: "return_to_standard", Interval: 0, Confidence: 0.8},
	{Region: "西北", Phrase: "复原", Normalized: "return_to_standard", Interval: 0, Confidence: 0.8},
}

// RegionTerm 地区术语词条。
type RegionTerm struct {
	Region     string
	Phrase     string
	Normalized string
	Interval   int
	Confidence float64
}

// Lookup 在词典中查找短语（按地区优先，其次全局包含匹配）。
func Lookup(phrase, region string) (RegionTerm, bool) {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return RegionTerm{}, false
	}
	// 精确（地区优先）
	for _, t := range Lexicon {
		if t.Region == region && t.Phrase == phrase {
			return t, true
		}
	}
	// 精确（全局）
	for _, t := range Lexicon {
		if t.Phrase == phrase {
			return t, true
		}
	}
	// 包含匹配（地区优先）
	for _, t := range Lexicon {
		if t.Region == region && strings.Contains(phrase, t.Phrase) {
			return t, true
		}
	}
	// 包含匹配（全局，取最长）
	best := RegionTerm{}
	bestLen := 0
	found := false
	for _, t := range Lexicon {
		if strings.Contains(phrase, t.Phrase) && len(t.Phrase) > bestLen {
			best = t
			bestLen = len(t.Phrase)
			found = true
		}
	}
	return best, found
}

var (
	// intervalRe 匹配 "第N弦"、"N弦"、"上弦/下弦" 等弦位指称。
	intervalRe = regexp.MustCompile(`[上中下高低一二三四五六七八九十]{1,2}[弦]`)
	// degreeRe 匹配 "纯四度/五度/八度/N度" 等音程描述。
	degreeRe = regexp.MustCompile(`(纯|大|小|增|减)?[一二三四五六七八][度]`)
)

// ExtractTuningClaims 从口述转录中提取候选"调弦声明"：
// 返回 (描述文本, 音程词, 置信度)。若无法识别任何音程描述，ok=false。
func ExtractTuningClaims(transcript string) (phrase string, term string, ok bool) {
	phrase = strings.TrimSpace(transcript)
	if phrase == "" {
		return "", "", false
	}
	// 音程描述（如"纯四度"）
	if m := degreeRe.FindString(phrase); m != "" {
		term = m
		return phrase, term, true
	}
	// 弦位指称（如"高弦下调"中的"高弦"）
	if m := intervalRe.FindString(phrase); m != "" {
		return phrase, m, true
	}
	// 直接尝试音程术语解析
	if _, found, _ := model.FuzzyResolveIntervalTerm(phrase); found {
		return phrase, phrase, true
	}
	return "", "", false
}
