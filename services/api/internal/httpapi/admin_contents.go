package httpapi

import (
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
	"github.com/believeboy/mini-bd-program/services/api/internal/audit"
	"github.com/believeboy/mini-bd-program/services/api/internal/content"
)

type adminContentHandlers struct {
	service        *content.Service
	maxUploadBytes int64
	logger         *slog.Logger
	audit          audit.ServiceAPI
	ipHashKey      []byte
}

type analysisResponse struct {
	ViewportPresent   bool     `json:"viewport_present"`
	ScriptCount       int      `json:"script_count"`
	ExternalResources []string `json:"external_resources"`
	Warnings          []string `json:"warnings"`
}

type contentResponse struct {
	ID        uuid.UUID        `json:"id"`
	Title     string           `json:"title"`
	Status    string           `json:"status"`
	VersionID uuid.UUID        `json:"version_id"`
	VersionNo int32            `json:"version_no"`
	SHA256    string           `json:"sha256"`
	ByteSize  int64            `json:"byte_size"`
	Analysis  analysisResponse `json:"analysis"`
	CreatedAt string           `json:"created_at"`
}

func newAdminContentHandlers(deps Dependencies) adminContentHandlers {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxUploadBytes := deps.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = 20 << 20
	}
	return adminContentHandlers{
		service:        deps.Content,
		maxUploadBytes: maxUploadBytes,
		logger:         logger,
		audit:          deps.Audit,
		ipHashKey:      deps.IPHashKey,
	}
}

func (h adminContentHandlers) upload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := adminPrincipal(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		h.rejectUpload(w, r, principal, http.StatusBadRequest, "INVALID_REQUEST", "上传请求必须使用 multipart/form-data")
		return
	}

	var (
		filename  string
		data      []byte
		fileCount int
	)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if isRequestTooLarge(err) {
				h.rejectUpload(w, r, principal, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "HTML 文件不能超过 20 MiB")
				return
			}
			h.rejectUpload(w, r, principal, http.StatusBadRequest, "INVALID_REQUEST", "上传内容无法读取")
			return
		}
		if part.FormName() != "file" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}
		fileCount++
		if fileCount > 1 {
			_ = part.Close()
			h.rejectUpload(w, r, principal, http.StatusBadRequest, "INVALID_REQUEST", "每次只能上传一个 HTML 文件")
			return
		}
		filename = part.FileName()
		data, err = io.ReadAll(io.LimitReader(part, h.maxUploadBytes+1))
		_ = part.Close()
		if err != nil {
			if isRequestTooLarge(err) {
				h.rejectUpload(w, r, principal, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "HTML 文件不能超过 20 MiB")
				return
			}
			h.rejectUpload(w, r, principal, http.StatusBadRequest, "INVALID_REQUEST", "上传内容无法读取")
			return
		}
		if int64(len(data)) > h.maxUploadBytes {
			h.rejectUpload(w, r, principal, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "HTML 文件不能超过 20 MiB")
			return
		}
	}
	if fileCount != 1 || filename == "" {
		h.rejectUpload(w, r, principal, http.StatusBadRequest, "INVALID_REQUEST", "请上传一个 HTML 文件")
		return
	}

	created, err := h.service.Create(r.Context(), content.CreateInput{
		Filename: filename,
		Data:     data,
		AdminID:  principal.ID,
	})
	if err != nil {
		h.writeCreateError(w, r, err)
		return
	}
	recordAudit(r, h.audit, h.logger, h.ipHashKey, "CONTENT_UPLOAD_ACCEPTED", audit.ActorAdmin, &principal.ID, nil,
		map[string]any{"content_id": created.ID, "version_id": created.VersionID, "byte_size": created.ByteSize})
	writeJSON(w, http.StatusCreated, toContentResponse(created))
}

func (h adminContentHandlers) rejectUpload(
	w http.ResponseWriter,
	r *http.Request,
	principal adminauth.Principal,
	status int,
	code, message string,
) {
	recordAudit(r, h.audit, h.logger, h.ipHashKey, "CONTENT_UPLOAD_REJECTED", audit.ActorAdmin,
		&principal.ID, nil, map[string]any{"reason": code})
	writeError(w, r, status, code, message)
}

func (h adminContentHandlers) list(w http.ResponseWriter, r *http.Request) {
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
		h.logger.ErrorContext(r.Context(), "list content failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取方案列表失败")
		return
	}
	response := make([]contentResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toContentResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h adminContentHandlers) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "方案 ID 不正确")
		return
	}
	item, err := h.service.Get(r.Context(), id)
	if errors.Is(err, content.ErrContentNotFound) {
		writeError(w, r, http.StatusNotFound, "CONTENT_NOT_FOUND", "方案不存在")
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "get content failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取方案失败")
		return
	}
	writeJSON(w, http.StatusOK, toContentResponse(item))
}

func (h adminContentHandlers) writeCreateError(w http.ResponseWriter, r *http.Request, err error) {
	principal, _ := adminPrincipal(r.Context())
	auditCode := "INTERNAL_ERROR"
	switch {
	case errors.Is(err, content.ErrFileTooLarge):
		auditCode = "FILE_TOO_LARGE"
		writeError(w, r, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "HTML 文件不能超过 20 MiB")
	case errors.Is(err, content.ErrUnsupportedFile):
		auditCode = "UNSUPPORTED_FILE"
		writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_FILE", "仅支持单个 .html 文件")
	case errors.Is(err, content.ErrExternalResources):
		auditCode = "EXTERNAL_RESOURCES"
		writeError(w, r, http.StatusUnprocessableEntity, "EXTERNAL_RESOURCES", "HTML 不允许引用外部资源")
	case errors.Is(err, content.ErrInvalidHTML):
		auditCode = "INVALID_HTML"
		writeError(w, r, http.StatusUnprocessableEntity, "INVALID_HTML", "HTML 内容无法解析")
	default:
		h.logger.ErrorContext(r.Context(), "create content failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "保存方案失败")
	}
	if principal.ID != uuid.Nil {
		recordAudit(r, h.audit, h.logger, h.ipHashKey, "CONTENT_UPLOAD_REJECTED", audit.ActorAdmin, &principal.ID, nil,
			map[string]any{"reason": auditCode})
	}
}

func toContentResponse(item content.Content) contentResponse {
	externalResources := item.Analysis.ExternalResources
	if externalResources == nil {
		externalResources = []string{}
	}
	warnings := item.Analysis.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return contentResponse{
		ID:        item.ID,
		Title:     item.Title,
		Status:    item.Status,
		VersionID: item.VersionID,
		VersionNo: item.VersionNo,
		SHA256:    hex.EncodeToString(item.SHA256),
		ByteSize:  item.ByteSize,
		Analysis: analysisResponse{
			ViewportPresent:   item.Analysis.ViewportPresent,
			ScriptCount:       item.Analysis.ScriptCount,
			ExternalResources: externalResources,
			Warnings:          warnings,
		},
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func intQuery(r *http.Request, key string, fallback int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}
