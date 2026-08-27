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

func TestFreezeHonorsCancelAndLatestSnapshot(t *testing.T) {
	app, _ := newProbeApp(t)
	bg := context.Background()
	batchID, err := app.Batches.Create(&model.ResearchBatch{Name: "freeze-probe"})
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
		BatchID: batchID, Transcript: "冻结探针",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for pos, hz := range map[int]float64{2: 196, 3: 261.6} {
		if _, err := app.Evidence.AttachPitch(bg, &model.PitchObservation{
			SegmentID: seg.ID, StringPos: pos, FrequencyHz: hz, Unit: "hz",
			Confidence: 0.9, RecordedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	relA, err := app.Relations.Create(&model.TuningRelation{
		BatchID: batchID, InstrumentID: insID, SegmentID: seg.ID,
		FromPosition: 3, ToPosition: 2, DescribedTerm: "纯四度", DescribedInterval: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	relB, err := app.Relations.Create(&model.TuningRelation{
		BatchID: batchID, InstrumentID: insID, SegmentID: seg.ID,
		FromPosition: 2, ToPosition: 3, DescribedTerm: "纯五度", DescribedInterval: 700,
	})
	if err != nil {
		t.Fatal(err)
	}
	gotA, err := app.Relations.Get(relA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Relations.Adjudicate(relA, model.RelationConfirmed, gotA.Version); err != nil {
		t.Fatal(err)
	}
	draft, err := app.CreateVersion(batchID, "draft-only")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(bg)
	cancel()
	if _, err := app.FreezeVersion(cancelled, draft.ID, draft.Version); err == nil {
		t.Fatal("expected freeze to fail on cancelled context")
	}
	stillDraft, err := app.Versions.Get(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stillDraft.Status != model.VersionDraft {
		t.Fatalf("cancelled freeze left status=%s want draft", stillDraft.Status)
	}
	gotB, err := app.Relations.Get(relB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Relations.Adjudicate(relB, model.RelationConfirmed, gotB.Version); err != nil {
		t.Fatal(err)
	}
	latest, err := app.CreateVersion(batchID, "latest")
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := app.FreezeVersion(bg, latest.ID, latest.Version)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot []*model.TuningRelation
	if err := json.Unmarshal([]byte(frozen.SnapshotJSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 2 {
		t.Fatalf("frozen snapshot relations=%d want=2 confirmed", len(snapshot))
	}
	for _, rel := range snapshot {
		if rel.Status != model.RelationConfirmed {
			t.Fatalf("snapshot relation %d status=%s want confirmed", rel.ID, rel.Status)
		}
	}
}
