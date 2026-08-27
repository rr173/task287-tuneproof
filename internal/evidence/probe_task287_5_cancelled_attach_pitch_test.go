package evidence

import (
	"context"
	"testing"
	"time"

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

func newProbeEvidence(t *testing.T) (*Service, int64, *store.SegmentStore) {
	t.Helper()
	db := newProbeDB(t)
	batches := store.NewBatchStore(db)
	batchID, err := batches.Create(&model.ResearchBatch{Name: "probe-batch"})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	segments := store.NewSegmentStore(db)
	svc := NewService(segments, batches, store.NewAuditStore(db))
	return svc, batchID, segments
}

func TestCancelledAttachPitchDoesNotWrite(t *testing.T) {
	svc, batchID, segments := newProbeEvidence(t)
	seg, created, err := svc.ImportSegment(ImportInput{
		BatchID: batchID, Transcript: "取消写入探针",
	})
	if err != nil || !created {
		t.Fatalf("import segment: %v created=%v", err, created)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Now().UTC()
	_, err = svc.AttachPitch(ctx, &model.PitchObservation{
		SegmentID: seg.ID, StringPos: 1, FrequencyHz: 220, Unit: "hz",
		Confidence: 0.9, RecordedAt: now,
	})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	pitches, err := segments.ListPitches(seg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pitches) != 0 {
		t.Fatalf("cancelled attach wrote %d pitch rows, want 0", len(pitches))
	}
}
