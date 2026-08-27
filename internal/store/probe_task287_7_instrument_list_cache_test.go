package store

import (
	"fmt"
	"sync"
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

func TestInstrumentListSeesNewCreate(t *testing.T) {
	db := newProbeDB(t)
	batches := NewBatchStore(db)
	batchID, err := batches.Create(&model.ResearchBatch{Name: "list-cache-probe"})
	if err != nil {
		t.Fatal(err)
	}
	is := NewInstrumentStore(db)
	id, err := is.Create(&model.Instrument{
		BatchID: batchID, Name: "新录入琵琶", StringCount: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := is.List(batchID)
			if err != nil {
				errCh <- err
				return
			}
			found := false
			for _, ins := range list {
				if ins.ID == id {
					found = true
					break
				}
			}
			if !found {
				errCh <- fmt.Errorf("missing instrument id=%d in list len=%d", id, len(list))
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}
