package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"task287-tuneproof/internal/model"
)

// alignTerms 对齐一段口述转录，返回结构化调弦声明。
func (h *Handler) alignTerms(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Transcript string `json:"transcript"`
		Region     string `json:"region"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	claim, ok, err := h.app.Align.AlignTranscript(in.Transcript, in.Region)
	if err != nil {
		respondError(w, err)
		return
	}
	if !ok {
		respond(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "无法从转录中解析出音程描述",
			"code":   "ambiguous_transcript",
			"claim":  claim,
		})
		return
	}
	respond(w, http.StatusOK, claim)
}

// createMapping 录入术语映射（地区特化）。
func (h *Handler) createMapping(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SourceTerm    string  `json:"source_term"`
		Region        string  `json:"region"`
		Normalized    string  `json:"normalized"`
		IntervalCents int     `json:"interval_cents"`
		Unit          string  `json:"unit"`
		Confidence    float64 `json:"confidence"`
		Notes         string  `json:"notes"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	m := &model.TermMapping{
		SourceTerm:    in.SourceTerm,
		Region:        in.Region,
		Normalized:    in.Normalized,
		IntervalCents: in.IntervalCents,
		Unit:          orDefault(in.Unit, "cents"),
		Confidence:    in.Confidence,
		Notes:         in.Notes,
	}
	if m.Confidence == 0 {
		m.Confidence = 1
	}
	id, err := h.app.Align.UpsertMapping(m)
	if err != nil {
		respondError(w, err)
		return
	}
	created, err := h.app.Mappings.Get(m.SourceTerm, m.Region)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{"id": id, "mapping": created})
}

// listMappings 列术语映射（按地区过滤）。
func (h *Handler) listMappings(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	mappings, err := h.app.Align.ListMappings(region)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, mappings)
}

// parseRecordedAt 解析观测时间（空则当前时间）。
func parseRecordedAt(s string) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}

// orDefault 空值回退。
func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// queryID 解析查询参数为 ID。
func queryID(r *http.Request, key string) int64 {
	v := r.URL.Query().Get(key)
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}
