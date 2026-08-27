package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"task287-tuneproof/internal/evidence"
	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/store"
)

func newProbeApp(t *testing.T) (*App, *store.DB) {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db), db
}

func TestFrozenVersionKeepsSnapshot(t *testing.T) {
	app, _ := newProbeApp(t)
	ctx := context.Background()
	batchID, err := app.Batches.Create(&model.ResearchBatch{Name: "snapshot-probe"})
	if err != nil {
		t.Fatal(err)
	}
	insID, err := app.Instruments.Create(&model.Instrument{
		BatchID: batchID, Name: "三弦", StringCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	seg, _, err := app.Evidence.ImportSegment(evidence.ImportInput{
		BatchID: batchID, Transcript: "中弦定调",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for pos, hz := range map[int]float64{2: 196, 3: 261.6} {
		if _, err := app.Evidence.AttachPitch(ctx, &model.PitchObservation{
			SegmentID: seg.ID, StringPos: pos, FrequencyHz: hz, Unit: "hz",
			Confidence: 0.9, RecordedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	relID, err := app.Relations.Create(&model.TuningRelation{
		BatchID: batchID, InstrumentID: insID, SegmentID: seg.ID,
		FromPosition: 3, ToPosition: 2, DescribedTerm: "纯四度", DescribedInterval: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	rel, err := app.Relations.Get(relID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Relations.Adjudicate(relID, model.RelationConfirmed, rel.Version); err != nil {
		t.Fatal(err)
	}
	ver, err := app.CreateVersion(batchID, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.FreezeVersion(ctx, ver.ID, ver.Version); err != nil {
		t.Fatal(err)
	}
	rel, err = app.Relations.Get(relID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Relations.Adjudicate(relID, model.RelationRejected, rel.Version); err != nil {
		t.Fatal(err)
	}
	frozen, err := app.Versions.Get(ver.ID)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot []*model.TuningRelation
	if err := json.Unmarshal([]byte(frozen.SnapshotJSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("snapshot relations=%d want=1", len(snapshot))
	}
	if snapshot[0].Status != model.RelationConfirmed {
		t.Fatalf("frozen snapshot status=%s want confirmed", snapshot[0].Status)
	}
}
