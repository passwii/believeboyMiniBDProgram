package content

import (
	"context"
	"errors"
	"fmt"

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

func (p *PostgresRepository) Create(ctx context.Context, input PersistInput) (Record, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Record{}, fmt.Errorf("begin content transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := p.queries.WithTx(tx)
	created, err := queries.CreateContent(ctx, store.CreateContentParams{
		ID:        input.ContentID,
		Title:     input.Title,
		CreatedBy: input.AdminID,
	})
	if err != nil {
		return Record{}, fmt.Errorf("create content: %w", err)
	}
	version, err := queries.CreateContentVersion(ctx, store.CreateContentVersionParams{
		ID:            input.VersionID,
		ContentID:     input.ContentID,
		ObjectKey:     input.ObjectKey,
		Sha256:        input.SHA256,
		ByteSize:      input.ByteSize,
		ContentTitle:  pgtype.Text{String: input.Title, Valid: true},
		AnalysisNotes: input.AnalysisNotes,
		CreatedBy:     input.AdminID,
	})
	if err != nil {
		return Record{}, fmt.Errorf("create content version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Record{}, fmt.Errorf("commit content transaction: %w", err)
	}
	return Record{
		ID:            created.ID,
		Title:         created.Title,
		Status:        created.Status,
		VersionID:     version.ID,
		VersionNo:     version.VersionNo,
		ObjectKey:     version.ObjectKey,
		SHA256:        version.Sha256,
		ByteSize:      version.ByteSize,
		AnalysisNotes: version.AnalysisNotes,
		CreatedAt:     created.CreatedAt.Time,
	}, nil
}

func (p *PostgresRepository) List(ctx context.Context, limit, offset int32) ([]Record, error) {
	rows, err := p.queries.ListContents(ctx, store.ListContentsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list contents: %w", err)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		records = append(records, Record{
			ID:            row.ID,
			Title:         row.Title,
			Status:        row.Status,
			VersionID:     row.VersionID,
			VersionNo:     row.VersionNo,
			SHA256:        row.Sha256,
			ByteSize:      row.ByteSize,
			AnalysisNotes: row.AnalysisNotes,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return records, nil
}

func (p *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Record, error) {
	row, err := p.queries.GetContent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrContentNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("get content: %w", err)
	}
	return Record{
		ID:            row.ID,
		Title:         row.Title,
		Status:        row.Status,
		VersionID:     row.VersionID,
		VersionNo:     row.VersionNo,
		ObjectKey:     row.ObjectKey,
		SHA256:        row.Sha256,
		ByteSize:      row.ByteSize,
		AnalysisNotes: row.AnalysisNotes,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}
