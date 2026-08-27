package model

import "math"

// 音程数学：把口述音程与录音实测频率统一到"音分"（cents）刻度上比较。
// 1 音分 = 1/1200 八度；十二平均律半音 = 100 音分。

const (
	// CentsPerSemitone 半音对应的音分数。
	CentsPerSemitone = 100.0
	// CentsPerOctave 八度对应的音分数。
	CentsPerOctave = 1200.0
	// DefaultToleranceCents 默认音程容差：口述是近似描述，允许 ±30 音分偏差。
	DefaultToleranceCents = 30
	// StrictToleranceCents 严格容差：用于已确认关系复核。
	StrictToleranceCents = 15
)

// CentsBetween 计算 f1 到 f2 的音程（音分）。f2 高于 f1 为正。
// 频率必须为正，否则返回 0。
func CentsBetween(f1, f2 float64) int {
	if f1 <= 0 || f2 <= 0 {
		return 0
	}
	return int(math.Round(CentsPerOctave * math.Log2(f2/f1)))
}

// FrequencyFromCents 从基准频率出发，按给定音分偏移计算目标频率。
func FrequencyFromCents(base float64, cents int) float64 {
	if base <= 0 {
		return 0
	}
	return base * math.Pow(2, float64(cents)/CentsPerOctave)
}

// ToleranceForTerm 根据口述措辞给出容差：带"约/近/大致"等修饰词时放宽到 45 音分。
func ToleranceForTerm(term string) int {
	if containsAny(term, []string{"约", "近", "大致", "差不多", "左右"}) {
		return 45
	}
	return DefaultToleranceCents
}

// InTolerance 判断实测与描述的差值是否落在容差内。
func InTolerance(delta, tolerance int) bool {
	return math.Abs(float64(delta)) <= float64(tolerance)
}

// NormalizeToOctave 把任意音分折进单八度 [0,1200)，用于比较等价音程。
func NormalizeToOctave(cents int) int {
	c := cents % CentsPerOctave
	if c < 0 {
		c += CentsPerOctave
	}
	return c
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
