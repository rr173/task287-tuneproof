package structure

import (
	"fmt"
	"sync"
	"testing"

	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/store"
)

func newProbeStructure(t *testing.T) (*Service, int64) {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/probe-structure.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	batches := store.NewBatchStore(db)
	batchID, err := batches.Create(&model.ResearchBatch{Name: "probe-batch"})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	svc := NewService(store.NewInstrumentStore(db), batches, store.NewAuditStore(db))
	return svc, batchID
}

func TestConcurrentInstrumentCreate(t *testing.T) {
	svc, batchID := newProbeStructure(t)
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _, err := svc.RegisterInstrument(&model.Instrument{
				BatchID: batchID, Name: fmt.Sprintf("instrument-%d", n), StringCount: 3,
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	list, err := svc.List(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != workers {
		t.Fatalf("instrument count=%d want=%d", len(list), workers)
	}
}
