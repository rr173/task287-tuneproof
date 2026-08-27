package httpapi

import (
	"net/http"

	"task287-tuneproof/internal/model"
)

// createBatch 创建研究批次。
func (h *Handler) createBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Region      string `json:"region"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	if in.Name == "" {
		respondError(w, model.NewValidationError("name", "required"))
		return
	}
	batch := &model.ResearchBatch{Name: in.Name, Description: in.Description, Region: in.Region}
	id, err := h.app.Batches.Create(batch)
	if err != nil {
		respondError(w, err)
		return
	}
	_ = h.app.Audit.Log(id, "batch.create", "research_batch", id, in.Name)
	created, err := h.app.Batches.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, created)
}

// listBatches 列批次。
func (h *Handler) listBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := h.app.Batches.List()
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, batches)
}

// getBatch 批次详情（含乐器、片段、关系、版本概要）。
func (h *Handler) getBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/batches")
	if err != nil {
		respondError(w, err)
		return
	}
	batch, err := h.app.Batches.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	instruments, _ := h.app.Instruments.List(id)
	segments, _ := h.app.Segments.List(id, "")
	relations, _ := h.app.Relations.List(id, "")
	versions, _ := h.app.Versions.List(id)
	audit, _ := h.app.Audit.List(id)
	respond(w, http.StatusOK, map[string]any{
		"batch":       batch,
		"instruments": instruments,
		"segments":    segments,
		"relations":   relations,
		"versions":    versions,
		"audit_logs":  audit,
	})
}

// updateBatchStatus 推进批次状态（乐观锁）。
func (h *Handler) updateBatchStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/batches")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		Status  string `json:"status"`
		Version int64  `json:"version"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	batch, err := h.app.Batches.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	if !model.ValidBatchTransition(batch.Status, in.Status) {
		respondError(w, model.ErrInvalidState)
		return
	}
	if model.BatchClosed(in.Status) && in.Status != model.BatchSealed {
		respondError(w, model.ErrFrozen)
		return
	}
	updated, err := h.app.Batches.UpdateStatus(id, in.Status, in.Version)
	if err != nil {
		respondError(w, err)
		return
	}
	_ = h.app.Audit.Log(id, "batch.status", "research_batch", id, batch.Status+" -> "+in.Status)
	respond(w, http.StatusOK, updated)
}

// sealBatch 封存批次（终态）。
func (h *Handler) sealBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/batches")
	if err != nil {
		respondError(w, err)
		return
	}
	batch, err := h.app.Batches.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	if batch.Status == model.BatchSealed {
		respond(w, http.StatusOK, batch)
		return
	}
	if !model.ValidBatchTransition(batch.Status, model.BatchSealed) {
		respondError(w, model.ErrInvalidState)
		return
	}
	sealed, err := h.app.Batches.UpdateStatus(id, model.BatchSealed, batch.Version)
	if err != nil {
		respondError(w, err)
		return
	}
	_ = h.app.Audit.Log(id, "batch.seal", "research_batch", id, "sealed")
	respond(w, http.StatusOK, sealed)
}
