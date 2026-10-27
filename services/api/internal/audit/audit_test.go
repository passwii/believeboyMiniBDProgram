package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryStore struct {
	events []Event
	err    error
}

func (m *memoryStore) Insert(_ context.Context, event Event) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func (m *memoryStore) List(context.Context, int32, int32) ([]Record, error) {
	return nil, nil
}

func TestRecordRequiresRequestIDAndValidActor(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	tests := []struct {
		name  string
		event Event
	}{
		{name: "missing request ID", event: Event{ActorType: "SYSTEM", EventType: "TEST"}},
		{name: "invalid actor", event: Event{ActorType: "CUSTOMER", EventType: "TEST", RequestID: "req-1"}},
		{name: "missing event", event: Event{ActorType: "SYSTEM", RequestID: "req-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := service.Record(context.Background(), test.event); err == nil {
				t.Fatal("expected invalid event to fail")
			}
		})
	}
}

func TestRecordDefaultsMetadataAndTimestamp(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	now := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	event := Event{ActorType: ActorSystem, EventType: "CLEANUP", RequestID: "req-1"}

	if err := service.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d", len(store.events))
	}
	if string(store.events[0].Metadata) != "{}" {
		t.Fatalf("Metadata = %s", store.events[0].Metadata)
	}
	if !store.events[0].CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %v, want %v", store.events[0].CreatedAt, now)
	}
}

func TestRecordRejectsInvalidMetadataAndPropagatesStorageError(t *testing.T) {
	service := NewService(&memoryStore{})
	if err := service.Record(context.Background(), Event{
		ActorType: ActorViewer, EventType: "UNLOCK", RequestID: "req-1", Metadata: json.RawMessage("{"),
	}); err == nil {
		t.Fatal("expected invalid JSON metadata to fail")
	}

	want := errors.New("database unavailable")
	service = NewService(&memoryStore{err: want})
	err := service.Record(context.Background(), Event{
		ActorType: ActorAdmin, ActorID: testUUIDPointer(uuid.New()),
		EventType: "LOGIN", RequestID: "req-2",
	})
	if !errors.Is(err, want) {
		t.Fatalf("Record() error = %v, want wrapped storage error", err)
	}
}

func testUUIDPointer(id uuid.UUID) *uuid.UUID { return &id }
