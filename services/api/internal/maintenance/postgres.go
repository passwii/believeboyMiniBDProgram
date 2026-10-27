package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (p *PostgresStore) CleanupExpired(ctx context.Context, now time.Time) (Counts, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Counts{}, fmt.Errorf("begin retention cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := store.New(p.pool).WithTx(tx)
	cutoff30Days := now.Add(-30 * 24 * time.Hour)
	counts := Counts{}

	count, err := queries.DeleteOldViewerTickets(ctx, store.DeleteOldViewerTicketsParams{
		ExpiresAt:  timestamp(now.Add(-24 * time.Hour)),
		ConsumedAt: timestamp(cutoff30Days),
	})
	if err != nil {
		return Counts{}, fmt.Errorf("delete old viewer tickets: %w", err)
	}
	counts.ViewerTickets = count

	counts.AccessSessions, err = queries.DeleteOldAccessSessions(ctx, timestamp(cutoff30Days))
	if err != nil {
		return Counts{}, fmt.Errorf("delete old access sessions: %w", err)
	}
	counts.AdminSessions, err = queries.DeleteOldAdminSessions(ctx, timestamp(cutoff30Days))
	if err != nil {
		return Counts{}, fmt.Errorf("delete old admin sessions: %w", err)
	}
	counts.AuditEvents, err = queries.DeleteOldAuditEvents(ctx, timestamp(now.Add(-180*24*time.Hour)))
	if err != nil {
		return Counts{}, fmt.Errorf("delete old audit events: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Counts{}, fmt.Errorf("commit retention cleanup: %w", err)
	}
	return counts, nil
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
