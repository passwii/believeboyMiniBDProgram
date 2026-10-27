package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/believeboy/mini-bd-program/services/api/internal/audit"
)

type adminAuditHandlers struct {
	service audit.ServiceAPI
	logger  *slog.Logger
}

func newAdminAuditHandlers(deps Dependencies) adminAuditHandlers {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return adminAuditHandlers{service: deps.Audit, logger: logger}
}

func (h adminAuditHandlers) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	limit, err := intQuery(r, "limit", 50)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "limit 参数不正确")
		return
	}
	offset, err := intQuery(r, "offset", 0)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "offset 参数不正确")
		return
	}
	items, err := h.service.List(r.Context(), int32(limit), int32(offset))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "list audit events failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取审计记录失败")
		return
	}
	if items == nil {
		items = []audit.Record{}
	}
	writeJSON(w, http.StatusOK, items)
}
