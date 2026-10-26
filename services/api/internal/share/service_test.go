package share

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/content"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
)

type fakeRepository struct {
	created      PersistShareInput
	record       ShareRecord
	list         []ShareRecord
	revoked      uuid.UUID
	reset        uuid.UUID
	resetPINHash string
}

func (f *fakeRepository) CreateShare(_ context.Context, input PersistShareInput) (ShareRecord, error) {
	f.created = input
	record := f.record
	record.ID = input.ID
	record.ContentVersionID = input.ContentVersionID
	record.Type = input.Type
	record.Label = input.Label
	record.TokenHash = input.TokenHash
	record.TokenCiphertext = input.TokenCiphertext
	record.PINHash = input.PINHash
	record.WatermarkText = input.WatermarkText
	record.ExpiresAt = input.ExpiresAt
	record.SessionTTL = input.SessionTTL
	return record, nil
}

func (f *fakeRepository) ListShares(context.Context, int32, int32) ([]ShareRecord, error) {
	return f.list, nil
}

func (f *fakeRepository) GetShareByTokenHash(_ context.Context, hash []byte) (ShareRecord, error) {
	for _, record := range f.list {
		if bytes.Equal(record.TokenHash, hash) {
			return record, nil
		}
	}
	return ShareRecord{}, ErrShareNotFound
}

func (f *fakeRepository) GetShareByID(_ context.Context, id uuid.UUID) (ShareRecord, error) {
	if id == f.record.ID {
		return f.record, nil
	}
	return ShareRecord{}, ErrShareNotFound
}

func (f *fakeRepository) RevokeShareAndSessions(_ context.Context, id uuid.UUID, _ time.Time) error {
	f.revoked = id
	return nil
}

func (f *fakeRepository) ResetPINAndRevokeSessions(_ context.Context, id uuid.UUID, hash string, _ time.Time) error {
	f.reset = id
	f.resetPINHash = hash
	return nil
}

func (f *fakeRepository) CreateSession(context.Context, SessionInput) (SessionRecord, error) {
	return SessionRecord{}, nil
}

func (f *fakeRepository) GetSessionByHash(context.Context, []byte) (SessionRecord, error) {
	return SessionRecord{}, ErrShareNotFound
}

func (f *fakeRepository) GetSessionByID(context.Context, uuid.UUID) (SessionRecord, error) {
	return SessionRecord{}, ErrShareNotFound
}

func (f *fakeRepository) TouchSession(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (f *fakeRepository) CreateTicket(context.Context, TicketInput) error {
	return nil
}

func (f *fakeRepository) ConsumeTicket(context.Context, []byte, time.Time) (TicketRecord, error) {
	return TicketRecord{}, ErrShareNotFound
}

type fakeContentLookup struct {
	item content.Content
	err  error
}

func (f fakeContentLookup) Get(context.Context, uuid.UUID) (content.Content, error) {
	return f.item, f.err
}

func newTestShareService(t *testing.T) (*Service, *fakeRepository) {
	t.Helper()
	cipher, err := security.NewCipher(bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	contentID := uuid.New()
	versionID := uuid.New()
	record := ShareRecord{
		ID:               uuid.New(),
		ContentID:        contentID,
		ContentVersionID: versionID,
		ContentTitle:     "Proposal",
		CreatedAt:        time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC),
	}
	repository := &fakeRepository{record: record}
	lookup := fakeContentLookup{item: content.Content{
		ID: contentID, VersionID: versionID, Title: "Proposal", Status: "ACTIVE",
	}}
	service := NewService(repository, lookup, cipher)
	service.now = func() time.Time { return time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC) }
	return service, repository
}

func TestCreateShareEncryptsTokenAndHashesPIN(t *testing.T) {
	service, repository := newTestShareService(t)
	expiresAt := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)

	created, err := service.Create(context.Background(), CreateInput{
		ContentID:  repository.record.ContentID,
		Type:       "PUBLIC",
		Label:      "群分享",
		PIN:        "1234",
		ExpiresAt:  expiresAt,
		SessionTTL: 30 * time.Minute,
		AdminID:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Token) != 43 {
		t.Fatalf("token length = %d", len(created.Token))
	}
	if repository.created.PINHash == "1234" {
		t.Fatal("PIN was stored in plaintext")
	}
	valid, err := security.VerifyCredential(repository.created.PINHash, "1234")
	if err != nil || !valid {
		t.Fatalf("PIN hash invalid: %v", err)
	}
	decrypted, err := service.cipher.Decrypt(repository.created.TokenCiphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(decrypted) != created.Token {
		t.Fatal("encrypted token does not match response token")
	}
	if created.WatermarkText != "群分享" {
		t.Fatalf("WatermarkText = %q", created.WatermarkText)
	}
}

func TestCreateShareRejectsInvalidInput(t *testing.T) {
	service, repository := newTestShareService(t)
	base := CreateInput{
		ContentID: repository.record.ContentID,
		Type:      "PUBLIC",
		Label:     "test",
		PIN:       "1234",
		ExpiresAt: time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC),
		AdminID:   uuid.New(),
	}

	tests := []struct {
		name   string
		mutate func(*CreateInput)
		want   error
	}{
		{name: "type", mutate: func(in *CreateInput) { in.Type = "OTHER" }, want: ErrInvalidShareType},
		{name: "pin", mutate: func(in *CreateInput) { in.PIN = "12a4" }, want: ErrInvalidPIN},
		{name: "label", mutate: func(in *CreateInput) { in.Label = "" }, want: ErrInvalidLabel},
		{name: "expiry", mutate: func(in *CreateInput) { in.ExpiresAt = time.Now().Add(-time.Hour) }, want: ErrInvalidExpiry},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.mutate(&input)
			_, err := service.Create(context.Background(), input)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestListSharesDecryptsTokens(t *testing.T) {
	service, repository := newTestShareService(t)
	raw := "known-token"
	encrypted, err := service.cipher.Encrypt([]byte(raw))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	repository.list = []ShareRecord{{
		ID: uuid.New(), ContentID: repository.record.ContentID,
		ContentVersionID: repository.record.ContentVersionID,
		ContentTitle:     "Proposal", TokenCiphertext: encrypted,
		Type: "PRIVATE", Label: "one", ExpiresAt: time.Now().Add(time.Hour),
		SessionTTL: 30 * time.Minute,
	}}

	items, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Token != raw {
		t.Fatalf("items = %+v", items)
	}
}

func TestRevokeAndResetPIN(t *testing.T) {
	service, repository := newTestShareService(t)
	id := repository.record.ID

	if err := service.Revoke(context.Background(), id); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if repository.revoked != id {
		t.Fatalf("revoked = %v", repository.revoked)
	}
	if err := service.ResetPIN(context.Background(), id, "5678"); err != nil {
		t.Fatalf("ResetPIN() error = %v", err)
	}
	if repository.reset != id {
		t.Fatalf("reset = %v", repository.reset)
	}
	valid, err := security.VerifyCredential(repository.resetPINHash, "5678")
	if err != nil || !valid {
		t.Fatalf("new PIN hash invalid: %v", err)
	}
}
