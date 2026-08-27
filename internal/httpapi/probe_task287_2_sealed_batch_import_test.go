package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/service"
	"task287-tuneproof/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/probe-http.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewRouter(service.New(db))
}

func TestSealedBatchImportMapsLocked(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"name":"封存批次","region":"闽台"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create batch=%d body=%s", rec.Code, rec.Body.String())
	}
	var batch model.ResearchBatch
	if err := json.Unmarshal(rec.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/batches/%d/seal", batch.ID), nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seal=%d body=%s", rec.Code, rec.Body.String())
	}
	segBody := fmt.Sprintf(`{"batch_id":%d,"transcript":"封存后导入应拒绝"}`, batch.ID)
	req = httptest.NewRequest(http.MethodPost, "/api/segments", strings.NewReader(segBody))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusLocked {
		t.Fatalf("sealed import status=%d body=%s", rec.Code, rec.Body.String())
	}
}
