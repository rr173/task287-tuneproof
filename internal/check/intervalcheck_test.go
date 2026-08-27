package check

import (
	"testing"

	"task287-tuneproof/internal/model"
)

func TestCentsBetween(t *testing.T) {
	cases := []struct {
		name string
		f1   float64
		f2   float64
		want int
	}{
		{"unison", 220, 220, 0},
		{"perfect fifth (702 cents)", 220, 330, 702},
		{"perfect fourth (500 cents, rounded)", 220, 293.66, 500},
		{"octave", 220, 440, 1200},
		{"descending", 330, 220, -702},
		{"invalid zero", 0, 220, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := model.CentsBetween(c.f1, c.f2)
			if got != c.want {
				t.Fatalf("CentsBetween(%v,%v) = %d, want %d", c.f1, c.f2, got, c.want)
			}
		})
	}
}

func TestIntervalMatchTolerance(t *testing.T) {
	ic := NewIntervalCheck(model.DefaultToleranceCents)
	if !ic.Match(15) {
		t.Fatal("偏差 15 音分应在默认容差内")
	}
	if ic.Match(50) {
		t.Fatal("偏差 50 音分应超出默认容差")
	}
	if got := ic.Verdict(10); got != model.RelationFeasible {
		t.Fatalf("Verdict(10) = %s, want feasible", got)
	}
	if got := ic.Verdict(120); got != model.RelationIntervalConflict {
		t.Fatalf("Verdict(120) = %s, want interval_conflict", got)
	}
}

func TestToleranceForTerm(t *testing.T) {
	if got := model.ToleranceForTerm("纯四度"); got != model.DefaultToleranceCents {
		t.Fatalf("精确术语容差 = %d, want %d", got, model.DefaultToleranceCents)
	}
	if got := model.ToleranceForTerm("约四度"); got != 45 {
		t.Fatalf("近似术语容差 = %d, want 45", got)
	}
}

func TestStringConstraint(t *testing.T) {
	positions := []*model.StringPosition{
		{Position: 1, MinFreqHz: 80, MaxFreqHz: 200},
		{Position: 2, MinFreqHz: 180, MaxFreqHz: 400},
		{Position: 3, MinFreqHz: 380, MaxFreqHz: 900},
	}
	sc := NewStringConstraint(positions, 3)

	if f := sc.Check(3, 2, 440, 220); len(f) != 0 {
		t.Fatalf("合法关系不应失败: %v", f)
	}
	if f := sc.Check(3, 3, 330, 220); len(f) == 0 {
		t.Fatal("自配对应失败")
	}
	if f := sc.Check(3, 4, 330, 220); len(f) == 0 {
		t.Fatal("越界弦位应失败")
	}
	if f := sc.Check(3, 2, 2000, 220); len(f) == 0 {
		t.Fatal("超范围频率应失败")
	}
	if !sc.InRange(3, 440) {
		t.Fatal("440Hz 应在高弦范围内")
	}
	if sc.InRange(3, 50) {
		t.Fatal("50Hz 应超出高弦范围")
	}
	if !sc.MissingPosition(4) {
		t.Fatal("位置 4 应缺失")
	}
}

func TestBatchTransitions(t *testing.T) {
	valid := [][2]string{
		{model.BatchDraft, model.BatchAligning},
		{model.BatchAligning, model.BatchReviewing},
		{model.BatchReviewing, model.BatchPublished},
		{model.BatchPublished, model.BatchSealed},
	}
	for _, tr := range valid {
		if !model.ValidBatchTransition(tr[0], tr[1]) {
			t.Fatalf("流转 %s -> %s 应合法", tr[0], tr[1])
		}
	}
	if model.ValidBatchTransition(model.BatchDraft, model.BatchPublished) {
		t.Fatal("draft -> published 应非法（跳过中间态）")
	}
	if model.ValidBatchTransition(model.BatchSealed, model.BatchDraft) {
		t.Fatal("封存后不可回退")
	}
}
