package model

import "strings"

// 术语词典：把口述中的调弦音程描述归一化为标准音程（音分）。
// 同时覆盖标准音乐术语与常见民族乐器口述说法。

// IntervalTerm 一条标准音程术语。
type IntervalTerm struct {
	Token   string   // 标准 token，如 perfect_fourth
	Cents   int      // 音分数
	Aliases []string // 别名/口语说法
}

// IntervalTerms 标准音程表（单八度内）。
var IntervalTerms = []IntervalTerm{
	{Token: "unison", Cents: 0, Aliases: []string{"同度", "齐奏", "同音", "unison"}},
	{Token: "minor_second", Cents: 100, Aliases: []string{"小二度", "半音", "minor second"}},
	{Token: "major_second", Cents: 200, Aliases: []string{"大二度", "全音", "major second"}},
	{Token: "minor_third", Cents: 300, Aliases: []string{"小三度", "minor third"}},
	{Token: "major_third", Cents: 400, Aliases: []string{"大三度", "major third"}},
	{Token: "perfect_fourth", Cents: 500, Aliases: []string{"纯四度", "四度", "perfect fourth"}},
	{Token: "tritone", Cents: 600, Aliases: []string{"三全音", "增四度", "tritone", "augmented fourth"}},
	{Token: "perfect_fifth", Cents: 700, Aliases: []string{"纯五度", "五度", "perfect fifth"}},
	{Token: "minor_sixth", Cents: 800, Aliases: []string{"小六度", "minor sixth"}},
	{Token: "major_sixth", Cents: 900, Aliases: []string{"大六度", "major sixth"}},
	{Token: "minor_seventh", Cents: 1000, Aliases: []string{"小七度", "minor seventh"}},
	{Token: "major_seventh", Cents: 1100, Aliases: []string{"大七度", "major seventh"}},
	{Token: "octave", Cents: 1200, Aliases: []string{"八度", "octave", "高八度"}},
}

// ResolveIntervalTerm 精确解析音程术语（标准 token 或别名精确匹配）。
func ResolveIntervalTerm(term string) (IntervalTerm, bool) {
	t := strings.TrimSpace(strings.ToLower(term))
	for _, it := range IntervalTerms {
		if it.Token == t {
			return it, true
		}
		for _, a := range it.Aliases {
			if strings.EqualFold(strings.TrimSpace(a), t) {
				return it, true
			}
		}
	}
	return IntervalTerm{}, false
}

// FuzzyResolveIntervalTerm 模糊解析：子串/包含匹配，返回置信度。
// 精确匹配置信度 1.0；包含匹配 0.7。找不到返回 ok=false。
func FuzzyResolveIntervalTerm(term string) (IntervalTerm, bool, float64) {
	t := strings.TrimSpace(strings.ToLower(term))
	if t == "" {
		return IntervalTerm{}, false, 0
	}
	// 先精确
	if it, ok := ResolveIntervalTerm(t); ok {
		return it, true, 1.0
	}
	// 数字音程描述：如 "四度"、"五度"（无"纯"字）→ 默认纯音程
	if n, ok := parseDegreeName(t); ok {
		return degreeTerm(n)
	}
	// 包含匹配
	best := IntervalTerm{}
	bestLen := 0
	found := false
	for _, it := range IntervalTerms {
		for _, a := range it.Aliases {
			al := strings.TrimSpace(strings.ToLower(a))
			if len(al) >= 2 && strings.Contains(t, al) && len(al) > bestLen {
				best = it
				bestLen = len(al)
				found = true
			}
		}
	}
	if found {
		return best, true, 0.7
	}
	return IntervalTerm{}, false, 0
}

// parseDegreeName 识别 "N度" 形式的度数描述（2-8 度）。
func parseDegreeName(t string) (int, bool) {
	if strings.HasSuffix(t, "度") {
		num := strings.TrimSuffix(t, "度")
		if len(num) == 1 && num[0] >= '1' && num[0] <= '8' {
			return int(num[0] - '0'), true
		}
	}
	return 0, false
}

// degreeTerm 度数（2-8）到默认纯/大音程的映射。
func degreeTerm(n int) (IntervalTerm, bool, float64) {
	switch n {
	case 2:
		return IntervalTerm{Token: "major_second", Cents: 200}, true, 0.7
	case 3:
		return IntervalTerm{Token: "major_third", Cents: 400}, true, 0.7
	case 4:
		return IntervalTerm{Token: "perfect_fourth", Cents: 500}, true, 0.7
	case 5:
		return IntervalTerm{Token: "perfect_fifth", Cents: 700}, true, 0.7
	case 6:
		return IntervalTerm{Token: "major_sixth", Cents: 900}, true, 0.7
	case 7:
		return IntervalTerm{Token: "major_seventh", Cents: 1100}, true, 0.7
	case 8:
		return IntervalTerm{Token: "octave", Cents: 1200}, true, 0.7
	}
	return IntervalTerm{}, false, 0
}
