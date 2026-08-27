package httpapi

import (
	"net/http"

	"task287-tuneproof/internal/model"
)

// createRelation 创建调弦关系（候选态）。
func (h *Handler) createRelation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BatchID      int64  `json:"batch_id"`
		InstrumentID int64  `json:"instrument_id"`
		SegmentID    int64  `json:"segment_id"`
		FromPosition int    `json:"from_position"`
		ToPosition   int    `json:"to_position"`
		DescribedTerm string `json:"described_term"`
		DescribedInterval int `json:"described_interval"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	// 音程未显式给出时，尝试从描述词解析
	described := in.DescribedInterval
	if described <= 0 && in.DescribedTerm != "" {
		if it, ok, _ := model.FuzzyResolveIntervalTerm(in.DescribedTerm); ok {
			described = it.Cents
		}
	}
	if described <= 0 {
		respondError(w, model.NewValidationError("described_interval", "cannot resolve interval"))
		return
	}
	rel := &model.TuningRelation{
		BatchID:          in.BatchID,
		InstrumentID:     in.InstrumentID,
		SegmentID:        in.SegmentID,
		FromPosition:     in.FromPosition,
		ToPosition:       in.ToPosition,
		DescribedTerm:    in.DescribedTerm,
		DescribedInterval: described,
	}
	id, err := h.app.Relations.Create(rel)
	if err != nil {
		respondError(w, err)
		return
	}
	_ = h.app.Audit.Log(in.BatchID, "relation.create", "tuning_relation", id, in.DescribedTerm)
	created, err := h.app.Relations.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, created)
}

// listRelations 列关系（按批次/状态过滤）。
func (h *Handler) listRelations(w http.ResponseWriter, r *http.Request) {
	batchID := queryID(r, "batch_id")
	status := r.URL.Query().Get("status")
	if batchID <= 0 {
		respondError(w, model.NewValidationError("batch_id", "required"))
		return
	}
	relations, err := h.app.Relations.List(batchID, status)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, relations)
}

// getRelation 关系详情。
func (h *Handler) getRelation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/relations")
	if err != nil {
		respondError(w, err)
		return
	}
	rel, err := h.app.Relations.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	variants, _ := h.app.Relations.ListVariants(id)
	respond(w, http.StatusOK, map[string]any{
		"relation": rel,
		"variants": variants,
	})
}

// checkRelation 执行可行性检查（音程比对 + 弦位约束）。
func (h *Handler) checkRelation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/relations")
	if err != nil {
		respondError(w, err)
		return
	}
	report, err := h.app.Check.CheckRelation(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, report)
}

// adjudicateRelation 研究者裁决（确认/否决），乐观锁。
func (h *Handler) adjudicateRelation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/relations")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		Verdict string `json:"verdict"` // confirmed / rejected
		Version int64  `json:"version"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	if in.Verdict != model.RelationConfirmed && in.Verdict != model.RelationRejected {
		respondError(w, model.NewValidationError("verdict", "must be confirmed or rejected"))
		return
	}
	rel, err := h.app.Relations.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	if !model.ValidRelationTransition(rel.Status, in.Verdict) {
		respondError(w, model.ErrInvalidState)
		return
	}
	updated, err := h.app.Relations.Adjudicate(id, in.Verdict, in.Version)
	if err != nil {
		respondError(w, err)
		return
	}
	_ = h.app.Audit.Log(rel.BatchID, "relation.adjudicate", "tuning_relation", id, in.Verdict)
	respond(w, http.StatusOK, updated)
}

// listConflicts 冲突关系列表（音程/弦位冲突，按批次过滤）。
func (h *Handler) listConflicts(w http.ResponseWriter, r *http.Request) {
	batchID := queryID(r, "batch_id")
	if batchID <= 0 {
		respondError(w, model.NewValidationError("batch_id", "required"))
		return
	}
	conflicts, err := h.app.Relations.List(batchID, model.RelationIntervalConflict)
	if err != nil {
		respondError(w, err)
		return
	}
	stringConflicts, err := h.app.Relations.List(batchID, model.RelationStringConflict)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"interval_conflicts": conflicts,
		"string_conflicts":   stringConflicts,
		"total":              len(conflicts) + len(stringConflicts),
	})
}

// listVariants 变体候选列表（按关系过滤）。
func (h *Handler) listVariants(w http.ResponseWriter, r *http.Request) {
	relationID := queryID(r, "relation_id")
	variants, err := h.app.Relations.ListVariants(relationID)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, variants)
}

// adjudicateVariant 裁决变体候选（采纳/否决）。
func (h *Handler) adjudicateVariant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/variants")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		Verdict string `json:"verdict"` // accepted / declined
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	if in.Verdict != model.VariantAccepted && in.Verdict != model.VariantDeclined {
		respondError(w, model.NewValidationError("verdict", "must be accepted or declined"))
		return
	}
	if err := h.app.Relations.AdjudicateVariant(id, in.Verdict); err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": in.Verdict})
}
