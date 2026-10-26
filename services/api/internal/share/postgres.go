package share

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

type PostgresRepository struct {
	pool    *pgxpool.Pool
	queries *store.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, queries: store.New(pool)}
}

func (p *PostgresRepository) CreateShare(ctx context.Context, input PersistShareInput) (ShareRecord, error) {
	row, err := p.queries.CreateShare(ctx, store.CreateShareParams{
		ID:                input.ID,
		ContentVersionID:  input.ContentVersionID,
		Type:              input.Type,
		Label:             input.Label,
		TokenHash:         input.TokenHash,
		TokenCiphertext:   input.TokenCiphertext,
		PinHash:           input.PINHash,
		WatermarkText:     input.WatermarkText,
		ExpiresAt:         timestamp(input.ExpiresAt),
		SessionTtlSeconds: int32(input.SessionTTL.Seconds()),
		CreatedBy:         input.AdminID,
	})
	if err != nil {
		return ShareRecord{}, fmt.Errorf("create share: %w", err)
	}
	return recordFromShare(row), nil
}

func (p *PostgresRepository) ListShares(ctx context.Context, limit, offset int32) ([]ShareRecord, error) {
	rows, err := p.queries.ListShares(ctx, store.ListSharesParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	records := make([]ShareRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, recordFromListRow(row))
	}
	return records, nil
}

func (p *PostgresRepository) GetShareByTokenHash(ctx context.Context, tokenHash []byte) (ShareRecord, error) {
	row, err := p.queries.GetShareByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShareRecord{}, ErrShareNotFound
	}
	if err != nil {
		return ShareRecord{}, fmt.Errorf("get share by token: %w", err)
	}
	return recordFromTokenRow(row), nil
}

func (p *PostgresRepository) GetShareByID(ctx context.Context, id uuid.UUID) (ShareRecord, error) {
	row, err := p.queries.GetShareByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShareRecord{}, ErrShareNotFound
	}
	if err != nil {
		return ShareRecord{}, fmt.Errorf("get share: %w", err)
	}
	return recordFromIDRow(row), nil
}

func (p *PostgresRepository) RevokeShareAndSessions(ctx context.Context, id uuid.UUID, at time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revoke transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := p.queries.WithTx(tx)
	if err := queries.RevokeShare(ctx, store.RevokeShareParams{ID: id, RevokedAt: timestamp(at)}); err != nil {
		return fmt.Errorf("revoke share: %w", err)
	}
	if err := queries.RevokeAccessSessionsByShare(ctx, store.RevokeAccessSessionsByShareParams{
		ShareID: id, RevokedAt: timestamp(at),
	}); err != nil {
		return fmt.Errorf("revoke access sessions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit revoke transaction: %w", err)
	}
	return nil
}

func (p *PostgresRepository) ResetPINAndRevokeSessions(ctx context.Context, id uuid.UUID, pinHash string, at time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin PIN reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := p.queries.WithTx(tx)
	if err := queries.ResetSharePIN(ctx, store.ResetSharePINParams{
		ID: id, PinHash: pinHash, UpdatedAt: timestamp(at),
	}); err != nil {
		return fmt.Errorf("reset share PIN: %w", err)
	}
	if err := queries.RevokeAccessSessionsByShare(ctx, store.RevokeAccessSessionsByShareParams{
		ShareID: id, RevokedAt: timestamp(at),
	}); err != nil {
		return fmt.Errorf("revoke access sessions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit PIN reset transaction: %w", err)
	}
	return nil
}

func (p *PostgresRepository) CreateSession(ctx context.Context, input SessionInput) (SessionRecord, error) {
	row, err := p.queries.CreateAccessSession(ctx, store.CreateAccessSessionParams{
		ID: input.ID, ShareID: input.ShareID, SessionHash: input.SessionHash,
		ExpiresAt: timestamp(input.ExpiresAt), LastSeenAt: timestamp(input.LastSeenAt),
		IpHash: input.IPHash, UserAgentHash: input.UserAgentHash,
	})
	if err != nil {
		return SessionRecord{}, fmt.Errorf("create access session: %w", err)
	}
	return SessionRecord{
		ID: row.ID, ShareID: row.ShareID, ExpiresAt: row.ExpiresAt.Time,
		LastSeenAt: row.LastSeenAt.Time,
	}, nil
}

func (p *PostgresRepository) GetSessionByHash(ctx context.Context, sessionHash []byte) (SessionRecord, error) {
	row, err := p.queries.GetAccessSessionByHash(ctx, sessionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRecord{}, ErrShareNotFound
	}
	if err != nil {
		return SessionRecord{}, fmt.Errorf("get access session: %w", err)
	}
	return sessionFromHashRow(row), nil
}

func (p *PostgresRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (SessionRecord, error) {
	row, err := p.queries.GetAccessSessionByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRecord{}, ErrShareNotFound
	}
	if err != nil {
		return SessionRecord{}, fmt.Errorf("get access session by id: %w", err)
	}
	return sessionFromIDRow(row), nil
}

func (p *PostgresRepository) TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	return p.queries.TouchAccessSession(ctx, store.TouchAccessSessionParams{
		ID: id, LastSeenAt: timestamp(at),
	})
}

func (p *PostgresRepository) CreateTicket(ctx context.Context, input TicketInput) error {
	_, err := p.queries.CreateViewerTicket(ctx, store.CreateViewerTicketParams{
		ID: input.ID, AccessSessionID: input.AccessSessionID, TicketHash: input.TicketHash,
		SessionCiphertext: input.SessionCiphertext, ExpiresAt: timestamp(input.ExpiresAt),
	})
	if err != nil {
		return fmt.Errorf("create viewer ticket: %w", err)
	}
	return nil
}

func (p *PostgresRepository) ConsumeTicket(ctx context.Context, ticketHash []byte, at time.Time) (TicketRecord, error) {
	row, err := p.queries.ConsumeViewerTicket(ctx, store.ConsumeViewerTicketParams{
		TicketHash: ticketHash, ConsumedAt: timestamp(at),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketRecord{}, ErrShareNotFound
	}
	if err != nil {
		return TicketRecord{}, fmt.Errorf("consume viewer ticket: %w", err)
	}
	return TicketRecord{
		AccessSessionID: row.AccessSessionID, SessionCiphertext: row.SessionCiphertext,
		ShareID: row.ShareID, ShareTokenCiphertext: row.TokenCiphertext,
	}, nil
}

func recordFromShare(row store.Share) ShareRecord {
	return ShareRecord{
		ID:               row.ID,
		ContentVersionID: row.ContentVersionID,
		Type:             row.Type,
		Label:            row.Label,
		TokenHash:        row.TokenHash,
		TokenCiphertext:  row.TokenCiphertext,
		PINHash:          row.PinHash,
		WatermarkText:    row.WatermarkText,
		ExpiresAt:        row.ExpiresAt.Time,
		RevokedAt:        timePointer(row.RevokedAt),
		SessionTTL:       time.Duration(row.SessionTtlSeconds) * time.Second,
		CreatedAt:        row.CreatedAt.Time,
	}
}

func recordFromTokenRow(row store.GetShareByTokenHashRow) ShareRecord {
	return ShareRecord{
		ID: row.ID, ContentID: row.ContentID, ContentVersionID: row.ContentVersionID,
		ContentTitle: row.ContentTitle, Type: row.Type, Label: row.Label,
		TokenHash: row.TokenHash, TokenCiphertext: row.TokenCiphertext, PINHash: row.PinHash,
		WatermarkText: row.WatermarkText, ExpiresAt: row.ExpiresAt.Time,
		RevokedAt: timePointer(row.RevokedAt), SessionTTL: time.Duration(row.SessionTtlSeconds) * time.Second,
		CreatedAt: row.CreatedAt.Time,
	}
}

func recordFromIDRow(row store.GetShareByIDRow) ShareRecord {
	return ShareRecord{
		ID: row.ID, ContentID: row.ContentID, ContentVersionID: row.ContentVersionID,
		ContentTitle: row.ContentTitle, Type: row.Type, Label: row.Label,
		TokenHash: row.TokenHash, TokenCiphertext: row.TokenCiphertext, PINHash: row.PinHash,
		WatermarkText: row.WatermarkText, ExpiresAt: row.ExpiresAt.Time,
		RevokedAt: timePointer(row.RevokedAt), SessionTTL: time.Duration(row.SessionTtlSeconds) * time.Second,
		CreatedAt: row.CreatedAt.Time,
	}
}

func recordFromListRow(row store.ListSharesRow) ShareRecord {
	return ShareRecord{
		ID: row.ID, ContentID: row.ContentID, ContentVersionID: row.ContentVersionID,
		ContentTitle: row.ContentTitle, Type: row.Type, Label: row.Label,
		TokenHash: row.TokenHash, TokenCiphertext: row.TokenCiphertext, PINHash: row.PinHash,
		WatermarkText: row.WatermarkText, ExpiresAt: row.ExpiresAt.Time,
		RevokedAt: timePointer(row.RevokedAt), SessionTTL: time.Duration(row.SessionTtlSeconds) * time.Second,
		CreatedAt: row.CreatedAt.Time,
	}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func sessionFromHashRow(row store.GetAccessSessionByHashRow) SessionRecord {
	return SessionRecord{
		ID: row.SessionID, ShareID: row.ShareID, ExpiresAt: row.SessionExpiresAt.Time,
		RevokedAt: timePointer(row.SessionRevokedAt), LastSeenAt: row.LastSeenAt.Time,
		ShareExpiresAt: row.ShareExpiresAt.Time, ShareRevokedAt: timePointer(row.ShareRevokedAt),
		Type: row.Type, Label: row.Label, WatermarkText: row.WatermarkText,
		ContentVersionID: row.ContentVersionID, ObjectKey: row.ObjectKey, ContentTitle: row.ContentTitle,
	}
}

func sessionFromIDRow(row store.GetAccessSessionByIDRow) SessionRecord {
	return SessionRecord{
		ID: row.SessionID, ShareID: row.ShareID, ExpiresAt: row.SessionExpiresAt.Time,
		RevokedAt: timePointer(row.SessionRevokedAt), LastSeenAt: row.LastSeenAt.Time,
		ShareExpiresAt: row.ShareExpiresAt.Time, ShareRevokedAt: timePointer(row.ShareRevokedAt),
		Type: row.Type, Label: row.Label, WatermarkText: row.WatermarkText,
		ContentVersionID: row.ContentVersionID, ObjectKey: row.ObjectKey, ContentTitle: row.ContentTitle,
	}
}
