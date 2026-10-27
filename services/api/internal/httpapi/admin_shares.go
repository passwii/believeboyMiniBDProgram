package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/audit"
	"github.com/believeboy/mini-bd-program/services/api/internal/share"
)

type adminShareService interface {
	Create(ctx context.Context, input share.CreateInput) (share.Share, error)
	List(ctx context.Context, limit, offset int32) ([]share.Share, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	ResetPIN(ctx context.Context, id uuid.UUID, pin string) error
}

type adminShareHandlers struct {
	service    adminShareService
	viewOrigin string
	logger     *slog.Logger
	audit      audit.ServiceAPI
	ipHashKey  []byte
}

type createShareRequest struct {
	ContentID         uuid.UUID `json:"content_id"`
	Type              string    `json:"type"`
	Label             string    `json:"label"`
	PIN               string    `json:"pin"`
	WatermarkText     string    `json:"watermark_text"`
	ExpiresAt         time.Time `json:"expires_at"`
	SessionTTLSeconds int       `json:"session_ttl_seconds"`
}

type resetPINRequest struct {
	PIN string `json:"pin"`
}

type shareResponse struct {
	ID                uuid.UUID `json:"id"`
	ContentID         uuid.UUID `json:"content_id"`
	ContentVersionID  uuid.UUID `json:"content_version_id"`
	ContentTitle      string    `json:"content_title"`
	Type              string    `json:"type"`
	Label             string    `json:"label"`
	WatermarkText     string    `json:"watermark_text"`
	ExpiresAt         string    `json:"expires_at"`
	RevokedAt         *string   `json:"revoked_at"`
	SessionTTLSeconds int       `json:"session_ttl_seconds"`
	CreatedAt         string    `json:"created_at"`
	H5URL             string    `json:"h5_url"`
	MiniappPath       string    `json:"miniapp_path"`
}

func newAdminShareHandlers(deps Dependencies) adminShareHandlers {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return adminShareHandlers{
		service:    deps.Shares,
		viewOrigin: strings.TrimRight(deps.ViewOrigin, "/"),
		logger:     logger,
		audit:      deps.Audit,
		ipHashKey:  deps.IPHashKey,
	}
}

func (h adminShareHandlers) create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !isJSON(r) {
		writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "仅支持 JSON 请求")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBody)
	var payload createShareRequest
	if err := decodeJSON(r, &payload); err != nil || payload.ContentID == uuid.Nil || payload.ExpiresAt.IsZero() {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "分享参数不完整")
		return
	}
	principal, ok := adminPrincipal(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
		return
	}
	created, err := h.service.Create(r.Context(), share.CreateInput{
		ContentID:     payload.ContentID,
		Type:          payload.Type,
		Label:         payload.Label,
		PIN:           payload.PIN,
		WatermarkText: payload.WatermarkText,
		ExpiresAt:     payload.ExpiresAt,
		SessionTTL:    time.Duration(payload.SessionTTLSeconds) * time.Second,
		AdminID:       principal.ID,
	})
	if err != nil {
		h.writeError(w, r, err, "创建分享失败")
		return
	}
	recordAudit(r, h.audit, h.logger, h.ipHashKey, "SHARE_CREATED", audit.ActorAdmin, &principal.ID, &created.ID,
		map[string]any{"type": created.Type, "expires_at": created.ExpiresAt.UTC().Format(time.RFC3339)})
	writeJSON(w, http.StatusCreated, h.response(created))
}

func (h adminShareHandlers) list(w http.ResponseWriter, r *http.Request) {
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
		h.logger.ErrorContext(r.Context(), "list shares failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取分享列表失败")
		return
	}
	response := make([]shareResponse, 0, len(items))
	for _, item := range items {
		response = append(response, h.response(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h adminShareHandlers) revoke(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "分享 ID 不正确")
		return
	}
	if err := h.service.Revoke(r.Context(), id); err != nil {
		h.writeError(w, r, err, "撤销分享失败")
		return
	}
	if principal, ok := adminPrincipal(r.Context()); ok {
		recordAudit(r, h.audit, h.logger, h.ipHashKey, "SHARE_REVOKED", audit.ActorAdmin, &principal.ID, &id, nil)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminShareHandlers) resetPIN(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "分享 ID 不正确")
		return
	}
	if !isJSON(r) {
		writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "仅支持 JSON 请求")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBody)
	var payload resetPINRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "请求格式不正确")
		return
	}
	if err := h.service.ResetPIN(r.Context(), id, payload.PIN); err != nil {
		h.writeError(w, r, err, "重置密码失败")
		return
	}
	if principal, ok := adminPrincipal(r.Context()); ok {
		recordAudit(r, h.audit, h.logger, h.ipHashKey, "SHARE_PIN_RESET", audit.ActorAdmin, &principal.ID, &id, nil)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminShareHandlers) response(item share.Share) shareResponse {
	var revokedAt *string
	if item.RevokedAt != nil {
		value := item.RevokedAt.UTC().Format(time.RFC3339)
		revokedAt = &value
	}
	return shareResponse{
		ID: item.ID, ContentID: item.ContentID, ContentVersionID: item.ContentVersionID,
		ContentTitle: item.ContentTitle, Type: item.Type, Label: item.Label,
		WatermarkText: item.WatermarkText, ExpiresAt: item.ExpiresAt.UTC().Format(time.RFC3339),
		RevokedAt: revokedAt, SessionTTLSeconds: int(item.SessionTTL.Seconds()),
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		H5URL:       h.viewOrigin + "/s/" + url.PathEscape(item.Token),
		MiniappPath: "/pages/open/index?t=" + url.QueryEscape(item.Token),
	}
}

func (h adminShareHandlers) writeError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, share.ErrShareNotFound):
		writeError(w, r, http.StatusNotFound, "SHARE_NOT_FOUND", "分享不存在")
	case errors.Is(err, share.ErrInvalidShareType):
		writeError(w, r, http.StatusUnprocessableEntity, "INVALID_SHARE_TYPE", "分享类型不正确")
	case errors.Is(err, share.ErrInvalidPIN):
		writeError(w, r, http.StatusUnprocessableEntity, "INVALID_PIN", "密码必须是 4 位数字")
	case errors.Is(err, share.ErrInvalidExpiry):
		writeError(w, r, http.StatusUnprocessableEntity, "INVALID_EXPIRY", "过期时间必须晚于当前时间")
	case errors.Is(err, share.ErrInvalidLabel):
		writeError(w, r, http.StatusUnprocessableEntity, "INVALID_LABEL", "分享标签不能为空")
	default:
		h.logger.ErrorContext(r.Context(), "admin share operation failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}
