package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
)

type adminPrincipalKey struct{}

func adminSessionMiddleware(deps Dependencies) func(http.Handler) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(deps.AdminCookieName)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
				return
			}
			principal, err := deps.AdminAuth.Authenticate(r.Context(), cookie.Value, adminauth.SessionMeta{})
			if errors.Is(err, adminauth.ErrSessionInvalid) {
				clearAdminCookie(w, deps.AdminCookieName, deps.SecureCookies)
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
				return
			}
			if err != nil {
				logger.ErrorContext(r.Context(), "admin session authentication failed", "error", err)
				writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取登录状态失败")
				return
			}
			ctx := context.WithValue(r.Context(), adminPrincipalKey{}, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func adminPrincipal(ctx context.Context) (adminauth.Principal, bool) {
	principal, ok := ctx.Value(adminPrincipalKey{}).(adminauth.Principal)
	return principal, ok
}

func clearAdminCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}
