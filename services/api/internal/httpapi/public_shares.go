package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/share"
)

type viewerAccessService interface {
	Status(ctx context.Context, token string) (share.ShareStatus, error)
	Unlock(ctx context.Context, token, pin string, meta share.AccessMeta, issueTicket bool) (share.UnlockResult, error)
	Authenticate(ctx context.Context, session string) (share.SessionRecord, error)
	ContentURL(ctx context.Context, session string) (share.ContentURLResult, error)
	EnterTicket(ctx context.Context, ticket string) (share.TicketLoginResult, error)
	ReadContent(ctx context.Context, signedToken string) ([]byte, error)
}

type publicShareHandlers struct {
	access     viewerAccessService
	cookieName string
	secure     bool
	ipHashKey  []byte
	viewOrigin string
	logger     *slog.Logger
}

type unlockRequest struct {
	PIN string `json:"pin"`
}

func newPublicShareHandlers(deps Dependencies) publicShareHandlers {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return publicShareHandlers{
		access: deps.Access, cookieName: deps.ViewerCookieName, secure: deps.SecureCookies,
		ipHashKey: deps.IPHashKey, viewOrigin: strings.TrimRight(deps.ViewOrigin, "/"), logger: logger,
	}
}

func (h publicShareHandlers) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	status, err := h.access.Status(r.Context(), chi.URLParam(r, "token"))
	if errors.Is(err, share.ErrShareNotFound) {
		writeError(w, r, http.StatusNotFound, "SHARE_NOT_FOUND", "分享不存在")
		return
	}
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"share_id": status.ShareID, "state": status.State, "type": status.Type,
		"label":      status.Label,
		"expires_at": status.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h publicShareHandlers) unlockH5(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var payload unlockRequest
	if !decodePINRequest(w, r, &payload) {
		return
	}
	result, err := h.access.Unlock(
		r.Context(), chi.URLParam(r, "token"), payload.PIN, h.meta(r), false,
	)
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	h.setSessionCookie(w, result.SessionToken, result.SessionExpiresAt)
	w.WriteHeader(http.StatusNoContent)
}

func (h publicShareHandlers) unlockMini(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var payload unlockRequest
	if !decodePINRequest(w, r, &payload) {
		return
	}
	result, err := h.access.Unlock(
		r.Context(), chi.URLParam(r, "token"), payload.PIN, h.meta(r), true,
	)
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket":     result.Ticket,
		"expires_at": result.SessionExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h publicShareHandlers) session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	session, err := h.authenticatedSession(r)
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"share_id":      session.ShareID,
		"content_title": session.ContentTitle, "label": session.Label,
		"watermark_text": session.WatermarkText,
		"expires_at":     session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h publicShareHandlers) heartbeat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, err := h.authenticatedSession(r); err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h publicShareHandlers) contentURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	session, err := h.authenticatedSession(r)
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	result, err := h.access.ContentURL(r.Context(), session.SessionToken)
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url": result.URL, "watermark_text": result.WatermarkText,
		"expires_at": result.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h publicShareHandlers) enter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.access.EnterTicket(r.Context(), r.URL.Query().Get("ticket"))
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	h.setSessionCookie(w, result.SessionToken, result.SessionExpiresAt)
	http.Redirect(w, r, h.viewOrigin+"/s/"+url.PathEscape(result.ShareToken), http.StatusSeeOther)
}

func (h publicShareHandlers) content(w http.ResponseWriter, r *http.Request) {
	data, err := h.access.ReadContent(r.Context(), chi.URLParam(r, "signedToken"))
	if err != nil {
		h.writeAccessError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy(h.viewOrigin))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h publicShareHandlers) authenticatedSession(r *http.Request) (sessionWithToken, error) {
	cookie, err := r.Cookie(h.cookieName)
	if err != nil || cookie.Value == "" {
		return sessionWithToken{}, share.ErrSessionInvalid
	}
	record, err := h.access.Authenticate(r.Context(), cookie.Value)
	return sessionWithToken{SessionRecord: record, SessionToken: cookie.Value}, err
}

type sessionWithToken struct {
	share.SessionRecord
	SessionToken string
}

func (h publicShareHandlers) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name: h.cookieName, Value: token, Path: "/", HttpOnly: true, Secure: h.secure,
		SameSite: http.SameSiteLaxMode, Expires: expiresAt, MaxAge: maxAge,
	})
}

func (h publicShareHandlers) meta(r *http.Request) share.AccessMeta {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	return share.AccessMeta{
		IPHash:        security.HMAC(h.ipHashKey, ip),
		UserAgentHash: security.HMAC(h.ipHashKey, r.UserAgent()),
	}
}

func (h publicShareHandlers) writeAccessError(w http.ResponseWriter, r *http.Request, err error) {
	var rateLimit *share.RateLimitError
	switch {
	case errors.Is(err, share.ErrShareNotFound):
		writeError(w, r, http.StatusNotFound, "SHARE_NOT_FOUND", "分享不存在")
	case errors.Is(err, share.ErrShareExpired):
		writeError(w, r, http.StatusGone, "SHARE_EXPIRED", "分享已过期")
	case errors.Is(err, share.ErrShareRevoked):
		writeError(w, r, http.StatusGone, "SHARE_REVOKED", "分享已撤销")
	case errors.Is(err, share.ErrPINRejected):
		writeError(w, r, http.StatusUnauthorized, "PIN_INVALID", "密码错误")
	case errors.Is(err, share.ErrRateLimited) && errors.As(err, &rateLimit):
		w.Header().Set("Retry-After", strconv.Itoa(int(rateLimit.RetryAfter.Seconds())))
		writeError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "尝试次数过多，请稍后再试")
	case errors.Is(err, share.ErrSessionInvalid):
		writeError(w, r, http.StatusUnauthorized, "SESSION_INVALID", "查看会话已失效")
	case errors.Is(err, share.ErrTicketInvalid):
		writeError(w, r, http.StatusGone, "TICKET_INVALID", "入场票据已失效")
	case errors.Is(err, share.ErrContentUnavailable):
		writeError(w, r, http.StatusNotFound, "CONTENT_UNAVAILABLE", "内容暂不可用")
	default:
		h.internalError(w, r, err)
	}
}

func (h publicShareHandlers) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "viewer access failed", "error", err)
	writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
}

func decodePINRequest(w http.ResponseWriter, r *http.Request, payload *unlockRequest) bool {
	if !isJSON(r) {
		writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "仅支持 JSON 请求")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := decodeJSON(r, payload); err != nil || payload.PIN == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "密码不能为空")
		return false
	}
	return true
}

func contentSecurityPolicy(viewOrigin string) string {
	return "default-src 'none'; script-src 'unsafe-inline' blob:; " +
		"style-src 'unsafe-inline'; img-src data: blob:; font-src data:; " +
		"media-src data: blob:; connect-src 'none'; frame-src 'none'; " +
		"worker-src blob:; object-src 'none'; base-uri 'none'; " +
		"form-action 'none'; frame-ancestors " + viewOrigin + ";"
}
