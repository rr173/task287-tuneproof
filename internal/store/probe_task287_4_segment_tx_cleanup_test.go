package store

import (
	"testing"
	"time"

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

func TestFailedSegmentInsertDoesNotBlock(t *testing.T) {
	db := newProbeDB(t)
	batches := NewBatchStore(db)
	batchID, err := batches.Create(&model.ResearchBatch{Name: "tx-probe"})
	if err != nil {
		t.Fatal(err)
	}
	ss := NewSegmentStore(db)
	base := &model.EvidenceSegment{
		BatchID: batchID, SourceType: "oral_history", Transcript: "重复指纹",
		Status: model.SegmentRaw, Fingerprint: "dup-fp",
	}
	if _, err := ss.Create(base); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.Create(base); err != model.ErrDuplicate {
		t.Fatalf("duplicate create err=%v want ErrDuplicate", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ss.Create(&model.EvidenceSegment{
			BatchID: batchID, SourceType: "oral_history", Transcript: "合法片段",
			Status: model.SegmentRaw, Fingerprint: "ok-fp",
		})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("valid insert after duplicate failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("valid insert blocked for more than 2s after duplicate failure")
	}
}
