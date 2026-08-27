package store

import (
	"testing"

	"task287-tuneproof/internal/model"
)

func newProbeDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestSegmentListResultsIndependent(t *testing.T) {
	db := newProbeDB(t)
	batches := NewBatchStore(db)
	batchID, err := batches.Create(&model.ResearchBatch{Name: "list-probe"})
	if err != nil {
		t.Fatal(err)
	}
	ss := NewSegmentStore(db)
	segA := &model.EvidenceSegment{
		BatchID: batchID, SourceType: "oral_history", Transcript: "片段甲",
		Status: model.SegmentRaw, Fingerprint: "fp-a",
	}
	segB := &model.EvidenceSegment{
		BatchID: batchID, SourceType: "oral_history", Transcript: "片段乙",
		Status: model.SegmentRaw, Fingerprint: "fp-b",
	}
	if _, err := ss.Create(segA); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.Create(segB); err != nil {
		t.Fatal(err)
	}
	first, err := ss.List(batchID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 2 {
		t.Fatalf("want 2 segments, got %d", len(first))
	}
	wantStatus := first[0].Status
	second, err := ss.List(batchID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) == 0 {
		t.Fatal("second list empty")
	}
	second[0].Status = model.SegmentAmbiguous
	if first[0].Status != wantStatus {
		t.Fatalf("first list mutated: status=%s want=%s", first[0].Status, wantStatus)
	}
}
