package check

import (
	"task287-tuneproof/internal/model"
)

// StringConstraint 弦位约束器：验证调弦关系在乐器构造上的物理可行性。
type StringConstraint struct {
	positions []*model.StringPosition
	stringCount int
}

// NewStringConstraint 构造弦位约束器。
func NewStringConstraint(positions []*model.StringPosition, stringCount int) *StringConstraint {
	return &StringConstraint{positions: positions, stringCount: stringCount}
}

// Check 执行全部弦位约束检查，返回失败原因列表（空 = 全部通过）。
func (c *StringConstraint) Check(fromPos, toPos int, fromHz, toHz float64) []string {
	var failures []string
	if fromPos == toPos {
		failures = append(failures, "from 与 to 弦位相同（自配对）")
	}
	if fromPos <= 0 || toPos <= 0 {
		failures = append(failures, "弦位号必须为正")
	}
	if fromPos > c.stringCount || toPos > c.stringCount {
		failures = append(failures, "弦位号超出乐器弦数")
	}
	from := c.positionByNum(fromPos)
	to := c.positionByNum(toPos)
	if from != nil && fromHz > 0 && (fromHz < from.MinFreqHz || fromHz > from.MaxFreqHz) {
		failures = append(failures, "from 弦实测频率超出可调范围")
	}
	if to != nil && toHz > 0 && (toHz < to.MinFreqHz || toHz > to.MaxFreqHz) {
		failures = append(failures, "to 弦实测频率超出可调范围")
	}
	return failures
}

// positionByNum 按位置号取弦位。
func (c *StringConstraint) positionByNum(num int) *model.StringPosition {
	for _, p := range c.positions {
		if p.Position == num {
			return p
		}
	}
	return nil
}

// InRange 判断频率是否在弦的可调范围内。
func (c *StringConstraint) InRange(position int, freq float64) bool {
	p := c.positionByNum(position)
	if p == nil || freq <= 0 {
		return false
	}
	return freq >= p.MinFreqHz && freq <= p.MaxFreqHz
}

// MissingPosition 是否缺少指定弦位定义（未录入）。
func (c *StringConstraint) MissingPosition(position int) bool {
	return c.positionByNum(position) == nil
}
