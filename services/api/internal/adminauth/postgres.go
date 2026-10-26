package adminauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

type PostgresStore struct {
	queries *store.Queries
}

func NewPostgresStore(queries *store.Queries) *PostgresStore {
	return &PostgresStore{queries: queries}
}

func (p *PostgresStore) FindAdminByEmail(ctx context.Context, email string) (AdminRecord, error) {
	admin, err := p.queries.GetAdminByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminRecord{}, ErrNotFound
	}
	if err != nil {
		return AdminRecord{}, fmt.Errorf("get admin by email: %w", err)
	}
	return AdminRecord{
		ID:             admin.ID,
		Email:          admin.Email,
		CredentialHash: admin.CredentialHash,
		Role:           admin.Role,
		Status:         admin.Status,
	}, nil
}

func (p *PostgresStore) CreateSession(ctx context.Context, input CreateSessionInput) error {
	_, err := p.queries.CreateAdminSession(ctx, store.CreateAdminSessionParams{
		ID:            input.ID,
		AdminUserID:   input.AdminUserID,
		SessionHash:   input.SessionHash,
		ExpiresAt:     timestamptz(input.ExpiresAt),
		LastSeenAt:    timestamptz(input.LastSeenAt),
		IpHash:        input.IPHash,
		UserAgentHash: input.UserAgentHash,
	})
	if err != nil {
		return fmt.Errorf("create admin session: %w", err)
	}
	return nil
}

func (p *PostgresStore) FindSession(ctx context.Context, hash []byte) (SessionRecord, error) {
	row, err := p.queries.GetAdminSessionByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRecord{}, ErrNotFound
	}
	if err != nil {
		return SessionRecord{}, fmt.Errorf("get admin session: %w", err)
	}
	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		value := row.RevokedAt.Time
		revokedAt = &value
	}
	return SessionRecord{
		ID:          row.SessionID,
		AdminUserID: row.UserID,
		ExpiresAt:   row.ExpiresAt.Time,
		LastSeenAt:  row.LastSeenAt.Time,
		RevokedAt:   revokedAt,
		Email:       row.Email,
		Role:        row.Role,
		Status:      row.Status,
	}, nil
}

func (p *PostgresStore) TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	return p.queries.TouchAdminSession(ctx, store.TouchAdminSessionParams{
		ID:         id,
		LastSeenAt: timestamptz(at),
	})
}

func (p *PostgresStore) RevokeSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	return p.queries.RevokeAdminSession(ctx, store.RevokeAdminSessionParams{
		ID:        id,
		RevokedAt: timestamptz(at),
	})
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
