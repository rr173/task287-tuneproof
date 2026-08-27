package httpapi

import (
	"net/http"

	"task287-tuneproof/internal/evidence"
	"task287-tuneproof/internal/model"
)

// importSegment 导入口述片段（指纹幂等）。
func (h *Handler) importSegment(w http.ResponseWriter, r *http.Request) {
	var in evidence.ImportInput
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	seg, created, err := h.app.Evidence.ImportSegment(in)
	if err != nil {
		respondError(w, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	respond(w, status, seg)
}

// listSegments 列片段（按批次/状态过滤）。
func (h *Handler) listSegments(w http.ResponseWriter, r *http.Request) {
	batchID := queryID(r, "batch_id")
	status := r.URL.Query().Get("status")
	if batchID <= 0 {
		respondError(w, model.NewValidationError("batch_id", "required"))
		return
	}
	segments, err := h.app.Evidence.List(batchID, status)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, segments)
}

// getSegment 片段详情（含音高观测）。
func (h *Handler) getSegment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/segments")
	if err != nil {
		respondError(w, err)
		return
	}
	seg, pitches, err := h.app.Evidence.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"segment": seg,
		"pitches": pitches,
	})
}

// addPitch 附加音高观测。
func (h *Handler) addPitch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/segments")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		StringPos   int     `json:"string_pos"`
		FrequencyHz float64 `json:"frequency_hz"`
		Unit        string  `json:"unit"`
		Confidence  float64 `json:"confidence"`
		RecordedAt  string  `json:"recorded_at"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	recorded, err := parseRecordedAt(in.RecordedAt)
	if err != nil {
		respondError(w, err)
		return
	}
	p := &model.PitchObservation{
		SegmentID:   id,
		StringPos:   in.StringPos,
		FrequencyHz: in.FrequencyHz,
		Unit:        orDefault(in.Unit, "hz"),
		Confidence:  in.Confidence,
		RecordedAt:  recorded,
	}
	if p.Confidence == 0 {
		p.Confidence = 1
	}
	pitchID, err := h.app.Evidence.AttachPitch(r.Context(), p)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]int64{"id": pitchID})
}

// updateSegmentStatus 变更片段状态（歧义/排除等）。
func (h *Handler) updateSegmentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/segments")
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
	updated, err := h.app.Evidence.SetStatus(id, in.Status, in.Version)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, updated)
}
