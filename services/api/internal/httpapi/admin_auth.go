package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
)

const maxAdminRequestBody = 16 << 10

type adminAuthHandlers struct {
	service    *adminauth.Service
	cookieName string
	sessionTTL time.Duration
	secure     bool
	logger     *slog.Logger
}

type adminLoginRequest struct {
	Email      string `json:"email"`
	Credential string `json:"credential"`
}

func newAdminAuthHandlers(deps Dependencies) adminAuthHandlers {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	ttl := deps.AdminSessionTTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return adminAuthHandlers{
		service:    deps.AdminAuth,
		cookieName: deps.AdminCookieName,
		sessionTTL: ttl,
		secure:     deps.SecureCookies,
		logger:     logger,
	}
}

func (h adminAuthHandlers) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !isJSON(r) {
		writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "仅支持 JSON 请求")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBody)
	var payload adminLoginRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "请求格式不正确")
		return
	}
	if strings.TrimSpace(payload.Email) == "" || strings.TrimSpace(payload.Credential) == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "邮箱和密码不能为空")
		return
	}

	result, err := h.service.Login(r.Context(), payload.Email, payload.Credential, adminauth.SessionMeta{})
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		writeError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "邮箱或密码错误")
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "admin login failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  result.ExpiresAt,
		MaxAge:   int(h.sessionTTL.Seconds()),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h adminAuthHandlers) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cookie, err := r.Cookie(h.cookieName)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
		return
	}
	principal, err := h.service.Authenticate(r.Context(), cookie.Value, adminauth.SessionMeta{})
	if errors.Is(err, adminauth.ErrSessionInvalid) {
		h.clearCookie(w)
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "admin authenticate failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取登录状态失败")
		return
	}
	writeJSON(w, http.StatusOK, principal)
}

func (h adminAuthHandlers) logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if cookie, err := r.Cookie(h.cookieName); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			h.logger.ErrorContext(r.Context(), "admin logout failed", "error", err)
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "退出登录失败")
			return
		}
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h adminAuthHandlers) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}

func isJSON(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func originGuard(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" && origin != allowedOrigin {
				writeError(w, r, http.StatusForbidden, "FORBIDDEN_ORIGIN", "请求来源不受信任")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
