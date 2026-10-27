package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

type PostgresStore struct {
	queries *store.Queries
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{queries: store.New(pool)}
}

func (p *PostgresStore) Insert(ctx context.Context, event Event) error {
	err := p.queries.CreateAuditEvent(ctx, store.CreateAuditEventParams{
		ID:            uuid.New(),
		ActorType:     event.ActorType,
		ActorID:       nullableUUID(event.ActorID),
		ShareID:       nullableUUID(event.ShareID),
		EventType:     event.EventType,
		RequestID:     event.RequestID,
		IpHash:        event.IPHash,
		UserAgentHash: event.UserAgentHash,
		MetadataJson:  event.Metadata,
		CreatedAt:     pgtype.Timestamptz{Time: event.CreatedAt.UTC(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (p *PostgresStore) List(ctx context.Context, limit, offset int32) ([]Record, error) {
	rows, err := p.queries.ListAuditEvents(ctx, store.ListAuditEventsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		records = append(records, Record{
			ID: row.ID, ActorType: row.ActorType, ActorID: uuidPointer(row.ActorID),
			ShareID: uuidPointer(row.ShareID), EventType: row.EventType,
			RequestID: row.RequestID, Metadata: row.MetadataJson,
			CreatedAt: row.CreatedAt.Time.UTC(),
		})
	}
	return records, nil
}

func nullableUUID(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *value, Valid: true}
}

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	result := uuid.UUID(value.Bytes)
	return &result
}
