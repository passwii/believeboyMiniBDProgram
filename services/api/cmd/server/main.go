package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
	"github.com/believeboy/mini-bd-program/services/api/internal/config"
	"github.com/believeboy/mini-bd-program/services/api/internal/content"
	"github.com/believeboy/mini-bd-program/services/api/internal/database"
	"github.com/believeboy/mini-bd-program/services/api/internal/httpapi"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/share"
	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := store.New(pool)
	adminAuth := adminauth.New(adminauth.NewPostgresStore(queries), cfg.AdminSessionTTL)
	objectStore, err := storage.NewS3Store(cfg)
	if err != nil {
		logger.Error("configure object storage", "error", err)
		os.Exit(1)
	}
	contentService := content.NewService(content.NewPostgresRepository(pool), objectStore, cfg.MaxUploadBytes)
	tokenCipher, err := security.NewCipher(cfg.TokenEncryptionKey)
	if err != nil {
		logger.Error("configure token cipher", "error", err)
		os.Exit(1)
	}
	shareService := share.NewService(share.NewPostgresRepository(pool), contentService, tokenCipher)
	accessService := share.NewAccessService(
		share.NewPostgresRepository(pool),
		tokenCipher,
		share.NewLimiter(),
		cfg.ContentSigningKey,
		cfg.ContentOrigin,
		cfg.ViewerSessionTTL,
		cfg.ViewerTicketTTL,
		cfg.ContentURLTTL,
		objectStore,
	)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Database:           pool,
			AdminAuth:          adminAuth,
			AdminCookieName:    cfg.AdminCookieName,
			AdminSessionTTL:    cfg.AdminSessionTTL,
			SecureCookies:      cfg.SecureCookies,
			AllowedAdminOrigin: cfg.AdminOrigin,
			Logger:             logger,
			Content:            contentService,
			MaxUploadBytes:     cfg.MaxUploadBytes,
			Shares:             shareService,
			ViewOrigin:         cfg.ViewOrigin,
			Access:             accessService,
			ViewerCookieName:   cfg.ViewerCookieName,
			IPHashKey:          cfg.IPHashKey,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve api", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown api", "error", err)
	}
}
