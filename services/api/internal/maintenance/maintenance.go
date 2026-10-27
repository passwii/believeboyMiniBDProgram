package maintenance

import (
	"context"
	"log/slog"
	"time"
)

type Counts struct {
	AdminSessions  int64
	AccessSessions int64
	ViewerTickets  int64
	AuditEvents    int64
}

type Store interface {
	CleanupExpired(context.Context, time.Time) (Counts, error)
}

func Run(ctx context.Context, store Store, logger *slog.Logger, interval time.Duration) {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Hour
	}
	cleanup := func() {
		counts, err := store.CleanupExpired(ctx, time.Now().UTC())
		if err != nil {
			logger.ErrorContext(ctx, "security retention cleanup failed", "error", err)
			return
		}
		logger.InfoContext(ctx, "security retention cleanup completed",
			"admin_sessions", counts.AdminSessions,
			"access_sessions", counts.AccessSessions,
			"viewer_tickets", counts.ViewerTickets,
			"audit_events", counts.AuditEvents,
		)
	}

	cleanup()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
