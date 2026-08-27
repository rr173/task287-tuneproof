package check

import (
	"errors"
	"testing"
	"time"

	"task287-tuneproof/internal/evidence"
	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/store"
)

func newProbeDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newProbeCheck(t *testing.T) (*Service, *store.DB, int64) {
	t.Helper()
	db := newProbeDB(t)
	batches := store.NewBatchStore(db)
	batchID, err := batches.Create(&model.ResearchBatch{Name: "probe-batch"})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	svc := NewService(
		store.NewRelationStore(db),
		store.NewSegmentStore(db),
		store.NewInstrumentStore(db),
		batches,
		store.NewAuditStore(db),
	)
	return svc, db, batchID
}

func TestSealedBatchCheckReturnsFrozen(t *testing.T) {
	svc, db, batchID := newProbeCheck(t)
	batches := store.NewBatchStore(db)
	instruments := store.NewInstrumentStore(db)
	segments := store.NewSegmentStore(db)
	relations := store.NewRelationStore(db)
	evidenceSvc := evidence.NewService(segments, batches, store.NewAuditStore(db))

	insID, err := instruments.Create(&model.Instrument{
		BatchID: batchID, Name: "三弦", StringCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	seg, _, err := evidenceSvc.ImportSegment(evidence.ImportInput{
		BatchID: batchID, Transcript: "封存检查探针",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for pos, hz := range map[int]float64{2: 196, 3: 261.6} {
		if _, err := segments.AddPitch(&model.PitchObservation{
			SegmentID: seg.ID, StringPos: pos, FrequencyHz: hz, Unit: "hz",
			Confidence: 0.9, RecordedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	relID, err := relations.Create(&model.TuningRelation{
		BatchID: batchID, InstrumentID: insID, SegmentID: seg.ID,
		FromPosition: 3, ToPosition: 2, DescribedTerm: "纯四度", DescribedInterval: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := batches.Get(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := batches.UpdateStatus(batchID, model.BatchSealed, batch.Version); err != nil {
		t.Fatal(err)
	}
	report, err := svc.CheckRelation(relID)
	if err == nil {
		if report != nil {
			t.Fatalf("expected ErrFrozen, got report verdict=%s", report.Verdict)
		}
		t.Fatal("expected ErrFrozen, got nil error and nil report")
	}
	if !errors.Is(err, model.ErrFrozen) {
		t.Fatalf("err=%v want ErrFrozen", err)
	}
}
