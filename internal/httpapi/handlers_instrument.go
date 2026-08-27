package httpapi

import (
	"net/http"

	"task287-tuneproof/internal/model"
)

// createInstrument 录入乐器。
func (h *Handler) createInstrument(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BatchID     int64  `json:"batch_id"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Region      string `json:"region"`
		StringCount int    `json:"string_count"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	ins := &model.Instrument{
		BatchID:     in.BatchID,
		Name:        in.Name,
		Category:    in.Category,
		Region:      in.Region,
		StringCount: in.StringCount,
	}
	created, _, err := h.app.Structure.RegisterInstrument(ins)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, created)
}

// listInstruments 列乐器（按批次过滤）。
func (h *Handler) listInstruments(w http.ResponseWriter, r *http.Request) {
	batchID := queryID(r, "batch_id")
	if batchID <= 0 {
		respondError(w, model.NewValidationError("batch_id", "required"))
		return
	}
	instruments, err := h.app.Structure.List(batchID)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, instruments)
}

// getInstrument 乐器详情（含弦位）。
func (h *Handler) getInstrument(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/instruments")
	if err != nil {
		respondError(w, err)
		return
	}
	ins, positions, err := h.app.Structure.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"instrument": ins,
		"positions":  positions,
	})
}

// addString 添加弦位。
func (h *Handler) addString(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/instruments")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		Position   int     `json:"position"`
		Name       string  `json:"name"`
		MinFreqHz  float64 `json:"min_freq_hz"`
		MaxFreqHz  float64 `json:"max_freq_hz"`
		StandardHz float64 `json:"standard_hz"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	p := &model.StringPosition{
		InstrumentID: id,
		Position:     in.Position,
		Name:         in.Name,
		MinFreqHz:    in.MinFreqHz,
		MaxFreqHz:    in.MaxFreqHz,
		StandardHz:   in.StandardHz,
	}
	positions, err := h.app.Structure.AddStringPosition(p)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, positions)
}

// listStrings 列弦位。
func (h *Handler) listStrings(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/instruments")
	if err != nil {
		respondError(w, err)
		return
	}
	positions, err := h.app.Structure.ListPositions(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, positions)
}
