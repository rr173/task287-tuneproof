package httpapi

import "net/http"

// stats 全库统计总览。
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.app.Stats()
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, stats)
}
