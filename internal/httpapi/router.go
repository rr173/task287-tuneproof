package httpapi

import (
	"net/http"

	"task287-tuneproof/internal/service"
)

// Handler HTTP 处理器，持有应用服务。
type Handler struct {
	app *service.App
}

// NewRouter 构造路由：全部端点以 /api 为前缀。
func NewRouter(app *service.App) http.Handler {
	h := &Handler{app: app}
	mux := http.NewServeMux()

	// 批次
	mux.HandleFunc("POST /api/batches", h.createBatch)
	mux.HandleFunc("GET /api/batches", h.listBatches)
	mux.HandleFunc("GET /api/batches/{id}", h.getBatch)
	mux.HandleFunc("PATCH /api/batches/{id}/status", h.updateBatchStatus)
	mux.HandleFunc("POST /api/batches/{id}/seal", h.sealBatch)

	// 乐器与弦位
	mux.HandleFunc("POST /api/instruments", h.createInstrument)
	mux.HandleFunc("GET /api/instruments", h.listInstruments)
	mux.HandleFunc("GET /api/instruments/{id}", h.getInstrument)
	mux.HandleFunc("POST /api/instruments/{id}/strings", h.addString)
	mux.HandleFunc("GET /api/instruments/{id}/strings", h.listStrings)

	// 证据片段与音高
	mux.HandleFunc("POST /api/segments", h.importSegment)
	mux.HandleFunc("GET /api/segments", h.listSegments)
	mux.HandleFunc("GET /api/segments/{id}", h.getSegment)
	mux.HandleFunc("POST /api/segments/{id}/pitch", h.addPitch)
	mux.HandleFunc("PATCH /api/segments/{id}/status", h.updateSegmentStatus)

	// 术语对齐
	mux.HandleFunc("POST /api/align/terms", h.alignTerms)
	mux.HandleFunc("GET /api/align/mappings", h.listMappings)
	mux.HandleFunc("POST /api/align/mappings", h.createMapping)

	// 调弦关系
	mux.HandleFunc("POST /api/relations", h.createRelation)
	mux.HandleFunc("GET /api/relations", h.listRelations)
	mux.HandleFunc("GET /api/relations/{id}", h.getRelation)
	mux.HandleFunc("POST /api/relations/{id}/check", h.checkRelation)
	mux.HandleFunc("POST /api/relations/{id}/adjudicate", h.adjudicateRelation)
	mux.HandleFunc("GET /api/relations/conflicts", h.listConflicts)

	// 变体候选
	mux.HandleFunc("GET /api/variants", h.listVariants)
	mux.HandleFunc("POST /api/variants/{id}/adjudicate", h.adjudicateVariant)

	// 调弦版本
	mux.HandleFunc("POST /api/versions", h.createVersion)
	mux.HandleFunc("GET /api/versions", h.listVersions)
	mux.HandleFunc("GET /api/versions/{id}", h.getVersion)
	mux.HandleFunc("GET /api/versions/{id}/diff", h.diffVersion)
	mux.HandleFunc("POST /api/versions/{id}/supersede", h.supersedeVersion)
	mux.HandleFunc("POST /api/versions/{id}/freeze", h.versionAction)
	mux.HandleFunc("POST /api/versions/{id}/share", h.versionAction)

	// 统计与健康
	mux.HandleFunc("GET /api/stats", h.stats)
	mux.HandleFunc("GET /api/health", h.health)

	return h.withLogging(mux)
}

// withLogging 简单访问日志中间件。
func (h *Handler) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
