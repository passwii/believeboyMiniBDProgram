package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ActorAdmin  = "ADMIN"
	ActorViewer = "VIEWER"
	ActorSystem = "SYSTEM"
)

var ErrInvalidEvent = errors.New("invalid audit event")

type Event struct {
	ActorType     string
	ActorID       *uuid.UUID
	ShareID       *uuid.UUID
	EventType     string
	RequestID     string
	IPHash        []byte
	UserAgentHash []byte
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

type Record struct {
	ID        uuid.UUID       `json:"id"`
	ActorType string          `json:"actor_type"`
	ActorID   *uuid.UUID      `json:"actor_id,omitempty"`
	ShareID   *uuid.UUID      `json:"share_id,omitempty"`
	EventType string          `json:"event_type"`
	RequestID string          `json:"request_id"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

type Store interface {
	Insert(context.Context, Event) error
	List(context.Context, int32, int32) ([]Record, error)
}

type ServiceAPI interface {
	Record(context.Context, Event) error
	List(context.Context, int32, int32) ([]Record, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Record(ctx context.Context, event Event) error {
	event.ActorType = strings.ToUpper(strings.TrimSpace(event.ActorType))
	event.EventType = strings.ToUpper(strings.TrimSpace(event.EventType))
	event.RequestID = strings.TrimSpace(event.RequestID)
	if event.ActorType != ActorAdmin && event.ActorType != ActorViewer && event.ActorType != ActorSystem {
		return fmt.Errorf("%w: actor type", ErrInvalidEvent)
	}
	if event.EventType == "" || len(event.EventType) > 80 || event.RequestID == "" || len(event.RequestID) > 200 {
		return fmt.Errorf("%w: event type or request ID", ErrInvalidEvent)
	}
	if len(event.Metadata) == 0 {
		event.Metadata = json.RawMessage(`{}`)
	} else if !json.Valid(event.Metadata) {
		return fmt.Errorf("%w: metadata must be valid JSON", ErrInvalidEvent)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = s.now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	if err := s.store.Insert(ctx, event); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, limit, offset int32) ([]Record, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.store.List(ctx, limit, offset)
}
