package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"task287-tuneproof/internal/evidence"
	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/store"
)

// TestFullLoop 端到端：批次→乐器→片段→对齐→检查→裁决→版本冻结→重启恢复。
func TestFullLoop(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// ---- 第一段 ----
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	app := New(db)

	batchID, err := app.Batches.Create(&model.ResearchBatch{Name: "端到端批次", Region: "闽台"})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	insID, err := app.Instruments.Create(&model.Instrument{
		BatchID: batchID, Name: "三弦", Category: "弹拨", Region: "闽台", StringCount: 3,
	})
	if err != nil {
		t.Fatalf("create instrument: %v", err)
	}
	for _, p := range []*model.StringPosition{
		{InstrumentID: insID, Position: 1, Name: "低弦", MinFreqHz: 60, MaxFreqHz: 200, StandardHz: 98},
		{InstrumentID: insID, Position: 2, Name: "中弦", MinFreqHz: 150, MaxFreqHz: 300, StandardHz: 196},
		{InstrumentID: insID, Position: 3, Name: "高弦", MinFreqHz: 240, MaxFreqHz: 600, StandardHz: 261.6},
	} {
		if _, err := app.Instruments.AddPosition(p); err != nil {
			t.Fatalf("add position %d: %v", p.Position, err)
		}
	}
	seg, created, err := app.Evidence.ImportSegment(evidence.ImportInput{
		BatchID: batchID, SourceType: "oral_history", SourceRef: "口述 1",
		Region: "闽台", Transcript: "中弦定调，高弦与中弦成纯四度",
	})
	if err != nil || !created {
		t.Fatalf("import segment: %v created=%v", err, created)
	}
	now := time.Now().UTC()
	// 实测：中弦 196Hz、高弦 261.6Hz ≈ 纯四度 498 音分
	for pos, hz := range map[int]float64{1: 98, 2: 196, 3: 261.6} {
		if _, err := app.Evidence.AttachPitch(context.Background(), &model.PitchObservation{
			SegmentID: seg.ID, StringPos: pos, FrequencyHz: hz, Unit: "hz", Confidence: 0.9, RecordedAt: now,
		}); err != nil {
			t.Fatalf("attach pitch %d: %v", pos, err)
		}
	}
	relID, err := app.Relations.Create(&model.TuningRelation{
		BatchID: batchID, InstrumentID: insID, SegmentID: seg.ID,
		FromPosition: 3, ToPosition: 2, DescribedTerm: "纯四度", DescribedInterval: 500,
	})
	if err != nil {
		t.Fatalf("create relation: %v", err)
	}
	report, err := app.Check.CheckRelation(relID)
	if err != nil {
		t.Fatalf("check relation: %v", err)
	}
	if report.Verdict != model.RelationFeasible {
		t.Fatalf("纯四度实测应可行, got %s (delta %d)", report.Verdict, report.DeltaCents)
	}
	rel, err := app.Relations.Get(relID)
	if err != nil {
		t.Fatalf("get relation: %v", err)
	}
	confirmed, err := app.Relations.Adjudicate(relID, model.RelationConfirmed, rel.Version)
	if err != nil {
		t.Fatalf("adjudicate: %v", err)
	}
	if confirmed.Status != model.RelationConfirmed {
		t.Fatalf("status = %s", confirmed.Status)
	}
	// 推进批次 + 发布冻结版本
	for _, st := range []string{model.BatchAligning, model.BatchReviewing, model.BatchPublished} {
		b, _ := app.Batches.Get(batchID)
		if _, err := app.Batches.UpdateStatus(batchID, st, b.Version); err != nil {
			t.Fatalf("advance %s: %v", st, err)
		}
	}
	ver, err := app.CreateVersion(batchID, "v1")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	if _, err := app.FreezeVersion(context.Background(), ver.ID, ver.Version); err != nil {
		t.Fatalf("freeze version: %v", err)
	}
	db.Close()

	// ---- 重启 ----
	db2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db2.Close()
	app2 := New(db2)

	rel2, err := app2.Relations.Get(relID)
	if err != nil {
		t.Fatalf("relation lost after restart: %v", err)
	}
	if rel2.Status != model.RelationConfirmed || rel2.MeasuredInterval == 0 {
		t.Fatalf("relation state lost: %+v", rel2)
	}
	ver2, err := app2.Versions.Get(ver.ID)
	if err != nil {
		t.Fatalf("version lost after restart: %v", err)
	}
	if ver2.Status != model.VersionFrozen || ver2.Fingerprint == "" {
		t.Fatalf("version state lost: %+v", ver2)
	}
}

// TestSealedRejectsMutation 封存批次后拒绝一切修改。
func TestSealedRejectsMutation(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := New(db)
	batchID, _ := app.Batches.Create(&model.ResearchBatch{Name: "封存测试"})
	b, _ := app.Batches.Get(batchID)
	if _, err := app.Batches.UpdateStatus(batchID, model.BatchSealed, b.Version); err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, _, err := app.Evidence.ImportSegment(evidence.ImportInput{
		BatchID: batchID, Transcript: "x",
	}); err != model.ErrFrozen {
		t.Fatalf("封存后导入应拒绝, got %v", err)
	}
	if _, _, err := app.Structure.RegisterInstrument(&model.Instrument{BatchID: batchID, Name: "y", StringCount: 1}); err != model.ErrFrozen {
		t.Fatalf("封存后录入乐器应拒绝, got %v", err)
	}
}

// TestFingerprintIdempotency 指纹幂等：同内容重复导入返回同一片段。
func TestFingerprintIdempotency(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "fp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := New(db)
	batchID, _ := app.Batches.Create(&model.ResearchBatch{Name: "幂等"})
	in := evidence.ImportInput{BatchID: batchID, Transcript: "同一句口述", Region: "西北"}
	first, created, err := app.Evidence.ImportSegment(in)
	if err != nil || !created {
		t.Fatalf("first import: %v %v", err, created)
	}
	second, created, err := app.Evidence.ImportSegment(in)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if created || second.ID != first.ID {
		t.Fatalf("幂等失败: created=%v id=%d vs %d", created, second.ID, first.ID)
	}
}
