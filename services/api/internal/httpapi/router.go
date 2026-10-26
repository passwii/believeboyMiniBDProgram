package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
	"github.com/believeboy/mini-bd-program/services/api/internal/content"
)

type Pinger interface {
	Ping(context.Context) error
}

type Dependencies struct {
	Database           Pinger
	AdminAuth          *adminauth.Service
	AdminCookieName    string
	AdminSessionTTL    time.Duration
	SecureCookies      bool
	AllowedAdminOrigin string
	Logger             *slog.Logger
	Content            *content.Service
	MaxUploadBytes     int64
	Shares             adminShareService
	ViewOrigin         string
}

func NewRouter(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	router.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if deps.Database == nil || deps.Database.Ping(r.Context()) != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if deps.AdminAuth != nil {
		handlers := newAdminAuthHandlers(deps)
		router.Route("/api/v1/admin", func(r chi.Router) {
			r.Use(originGuard(deps.AllowedAdminOrigin))
			r.Post("/login", handlers.login)
			r.Get("/me", handlers.me)
			r.Post("/logout", handlers.logout)

			if deps.Content != nil {
				contentHandlers := newAdminContentHandlers(deps)
				r.Group(func(r chi.Router) {
					r.Use(adminSessionMiddleware(deps))
					r.Get("/contents", contentHandlers.list)
					r.Post("/contents", contentHandlers.upload)
					r.Get("/contents/{id}", contentHandlers.get)
				})
			}
			if deps.Shares != nil {
				shareHandlers := newAdminShareHandlers(deps)
				r.Group(func(r chi.Router) {
					r.Use(adminSessionMiddleware(deps))
					r.Get("/shares", shareHandlers.list)
					r.Post("/shares", shareHandlers.create)
					r.Post("/shares/{id}/revoke", shareHandlers.revoke)
					r.Post("/shares/{id}/reset-pin", shareHandlers.resetPIN)
				})
			}
		})
	}

	return router
}
