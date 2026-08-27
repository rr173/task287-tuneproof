package httpapi

import (
	"net/http"
	"strconv"

	"task287-tuneproof/internal/model"
)

// createVersion 发布调弦版本（草稿态）。
func (h *Handler) createVersion(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BatchID int64  `json:"batch_id"`
		Name    string `json:"name"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	v, err := h.app.CreateVersion(in.BatchID, in.Name)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, v)
}

// listVersions 列版本（按批次过滤）。
func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request) {
	batchID := queryID(r, "batch_id")
	if batchID <= 0 {
		respondError(w, model.NewValidationError("batch_id", "required"))
		return
	}
	versions, err := h.app.Versions.List(batchID)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, versions)
}

// getVersion 版本详情（含快照内容）。
func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/versions")
	if err != nil {
		respondError(w, err)
		return
	}
	v, err := h.app.Versions.Get(id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, v)
}

// diffVersion 对比两版本快照差异。
func (h *Handler) diffVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/versions")
	if err != nil {
		respondError(w, err)
		return
	}
	other := queryID(r, "with")
	if other <= 0 {
		respondError(w, model.NewValidationError("with", "required"))
		return
	}
	added, removed, err := h.app.VersionDiff(other, id)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"base_version":    other,
		"compare_version": id,
		"added":           added,
		"removed":         removed,
	})
}

// supersedeVersion 用新版本替代冻结版本。
func (h *Handler) supersedeVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/versions")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		NewVersionID int64 `json:"new_version_id"`
		Version      int64 `json:"version"`
	}
	if err := decode(r, &in); err != nil {
		respondError(w, err)
		return
	}
	updated, err := h.app.SupersedeVersion(id, in.NewVersionID, in.Version)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, updated)
}

// versionAction 便捷路由处理：share / freeze 通过 PATCH 语义由 batch 层调用。
// 为保持 API 完备，提供显式端点：
//   POST /api/versions/{id}/freeze
//   POST /api/versions/{id}/share
func (h *Handler) versionAction(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "/api/versions")
	if err != nil {
		respondError(w, err)
		return
	}
	var in struct {
		Version int64 `json:"version"`
	}
	_ = decode(r, &in)
	var updated *model.TuningVersion
	action := pathTail(r.URL.Path, "/api/versions/")
	switch action {
	case "freeze":
		updated, err = h.app.FreezeVersion(r.Context(), id, in.Version)
	case "share":
		updated, err = h.app.ShareVersion(id, in.Version)
	default:
		respondError(w, model.NewValidationError("action", "unknown version action"))
		return
	}
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, updated)
}

// itoa 便捷整数转字符串。
func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// pathTail 取路径最后一段（动作名）。
func pathTail(p, prefix string) string {
	tail := p[len(prefix):]
	for i := len(tail) - 1; i >= 0; i-- {
		if tail[i] == '/' {
			return tail[i+1:]
		}
	}
	return tail
}
