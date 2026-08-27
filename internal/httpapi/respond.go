// Package httpapi HTTP 层：路由统一 /api 前缀，JSON 出入参。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"task287-tuneproof/internal/model"
)

// respond 统一 JSON 响应。
func respond(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// errResponse 错误响应体。
type errResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// respondError 按领域错误映射 HTTP 状态码。
func respondError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal"
	switch {
	case errors.Is(err, model.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, model.ErrConflict):
		status, code = http.StatusConflict, "version_conflict"
	case errors.Is(err, model.ErrInvalidState):
		status, code = http.StatusUnprocessableEntity, "invalid_state"
	case errors.Is(err, model.ErrFrozen):
		status, code = http.StatusLocked, "frozen"
	case errors.Is(err, model.ErrDuplicate):
		status, code = http.StatusConflict, "duplicate"
	case errors.Is(err, model.ErrValidation):
		status, code = http.StatusBadRequest, "validation"
	case errors.Is(err, model.ErrRelation):
		status, code = http.StatusBadRequest, "invalid_relation"
	}
	respond(w, status, errResponse{Error: err.Error(), Code: code})
}

// decode 解析请求体 JSON 到目标结构。
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return model.NewValidationError("body", err.Error())
	}
	return nil
}

// pathID 解析路径中的 ID 段（/api/x/{id}/...）。
func pathID(r *http.Request, prefix string) (int64, error) {
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	seg := rest
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		seg = rest[:i]
	}
	id, err := strconv.ParseInt(seg, 10, 64)
	if err != nil || id <= 0 {
		return 0, model.NewValidationError("id", "invalid id")
	}
	return id, nil
}
