package check

import (
	"fmt"
	"math"

	"task287-tuneproof/internal/model"
)

// IntervalCheck 音程比对器：把口述音程与录音实测音高换算到同一音分刻度。
type IntervalCheck struct {
	tolerance int
}

// NewIntervalCheck 构造音程比对器，容差可配置（默认 30 音分）。
func NewIntervalCheck(tolerance int) *IntervalCheck {
	if tolerance <= 0 {
		tolerance = model.DefaultToleranceCents
	}
	return &IntervalCheck{tolerance: tolerance}
}

// Compare 比对描述音程与实测音程。
// describedCents 来自口述解析；fromHz/toHz 来自录音音高观测。
// 音程本身无方向（上行四度与下行四度同为"四度"），实测取绝对值。
// 返回 (measuredCents, deltaCents, ok)。
func (c *IntervalCheck) Compare(describedCents int, fromHz, toHz float64) (measured, delta int, ok bool) {
	if fromHz <= 0 || toHz <= 0 {
		return 0, 0, false
	}
	raw := model.CentsBetween(fromHz, toHz)
	measured = raw
	if measured < 0 {
		measured = -measured
	}
	delta = measured - describedCents
	return measured, delta, true
}

// Match 判断 delta 是否落在容差内。
func (c *IntervalCheck) Match(delta int) bool {
	return model.InTolerance(delta, c.tolerance)
}

// Verdict 生成音程层面裁决字符串（可行/音程冲突）。
func (c *IntervalCheck) Verdict(delta int) string {
	if c.Match(delta) {
		return model.RelationFeasible
	}
	return model.RelationIntervalConflict
}

// Describe 生成人类可读的音程说明。
func Describe(cents int) string {
	octaves := cents / model.CentsPerOctave
	rest := cents % model.CentsPerOctave
	semitones := float64(rest) / model.CentsPerSemitone
	desc := fmt.Sprintf("%.1f 半音", semitones)
	if octaves > 0 {
		desc = fmt.Sprintf("%d 八度 + %.1f 半音", octaves, semitones)
	}
	return desc
}

// RoundDelta 将 delta 规整到整音分。
func RoundDelta(delta float64) int {
	return int(math.Round(delta))
}
