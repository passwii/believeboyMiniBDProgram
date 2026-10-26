package share

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
)

type accessRepo struct {
	*fakeRepository
	sessions map[string]SessionRecord
	tickets  map[string]TicketRecord
}

func (a *accessRepo) CreateSession(_ context.Context, input SessionInput) (SessionRecord, error) {
	record := SessionRecord{
		ID: input.ID, ShareID: input.ShareID, ExpiresAt: input.ExpiresAt,
		LastSeenAt: input.LastSeenAt, ShareExpiresAt: a.list[0].ExpiresAt,
		Type: a.list[0].Type, Label: a.list[0].Label,
		WatermarkText:    a.list[0].WatermarkText,
		ContentVersionID: a.list[0].ContentVersionID,
		ObjectKey:        "content/one.html", ContentTitle: a.list[0].ContentTitle,
	}
	a.sessions[string(input.SessionHash)] = record
	return record, nil
}

func (a *accessRepo) GetSessionByHash(_ context.Context, hash []byte) (SessionRecord, error) {
	record, ok := a.sessions[string(hash)]
	if !ok {
		return SessionRecord{}, ErrShareNotFound
	}
	return record, nil
}

func (a *accessRepo) GetSessionByID(_ context.Context, id uuid.UUID) (SessionRecord, error) {
	for _, record := range a.sessions {
		if record.ID == id {
			return record, nil
		}
	}
	return SessionRecord{}, ErrShareNotFound
}

func (a *accessRepo) TouchSession(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (a *accessRepo) CreateTicket(_ context.Context, input TicketInput) error {
	a.tickets[string(input.TicketHash)] = TicketRecord{
		AccessSessionID:      input.AccessSessionID,
		SessionCiphertext:    input.SessionCiphertext,
		ShareID:              a.list[0].ID,
		ShareTokenCiphertext: a.list[0].TokenCiphertext,
	}
	return nil
}

func (a *accessRepo) ConsumeTicket(_ context.Context, hash []byte, _ time.Time) (TicketRecord, error) {
	record, ok := a.tickets[string(hash)]
	if !ok {
		return TicketRecord{}, ErrShareNotFound
	}
	delete(a.tickets, string(hash))
	return record, nil
}

func newAccessFixture(t *testing.T) (*AccessService, *accessRepo, string) {
	t.Helper()
	cipher, err := security.NewCipher(bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	pinHash, err := security.HashCredential("1234")
	if err != nil {
		t.Fatalf("HashCredential() error = %v", err)
	}
	now := time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)
	shareToken := "share-token"
	tokenCiphertext, err := cipher.Encrypt([]byte(shareToken))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	base := &fakeRepository{list: []ShareRecord{{
		ID: uuid.New(), ContentID: uuid.New(), ContentVersionID: uuid.New(),
		ContentTitle: "Proposal", Type: "PUBLIC", Label: "group",
		TokenHash: security.HashToken(shareToken), TokenCiphertext: tokenCiphertext,
		PINHash: pinHash, WatermarkText: "group watermark",
		ExpiresAt: now.Add(24 * time.Hour), SessionTTL: 30 * time.Minute,
	}}}
	repo := &accessRepo{
		fakeRepository: base,
		sessions:       make(map[string]SessionRecord),
		tickets:        make(map[string]TicketRecord),
	}
	service := NewAccessService(
		repo, cipher, NewLimiter(), bytes.Repeat([]byte{0x72}, 32),
		"https://content.example.test", 30*time.Minute, time.Minute, 10*time.Minute,
		storage.NewMemoryStore(),
	)
	service.now = func() time.Time { return now }
	return service, repo, shareToken
}

func TestAccessUnlockAndContentURL(t *testing.T) {
	service, _, shareToken := newAccessFixture(t)
	result, err := service.Unlock(context.Background(), shareToken, "1234", AccessMeta{}, false)
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if len(result.SessionToken) != 43 || result.Ticket != "" {
		t.Fatalf("result = %+v", result)
	}
	session, err := service.Authenticate(context.Background(), result.SessionToken)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if session.ContentVersionID == uuid.Nil {
		t.Fatal("session missing version")
	}
	content, err := service.ContentURL(context.Background(), result.SessionToken)
	if err != nil {
		t.Fatalf("ContentURL() error = %v", err)
	}
	encoded := strings.TrimPrefix(content.URL, "https://content.example.test/c/")
	claims, err := VerifyClaims(encoded, bytes.Repeat([]byte{0x72}, 32))
	if err != nil {
		t.Fatalf("VerifyClaims() error = %v", err)
	}
	if claims.VersionID != session.ContentVersionID {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestAccessWrongPINIsRateLimited(t *testing.T) {
	service, _, shareToken := newAccessFixture(t)
	for i := 0; i < 5; i++ {
		_, err := service.Unlock(context.Background(), shareToken, "0000", AccessMeta{}, false)
		if !errors.Is(err, ErrPINRejected) {
			t.Fatalf("attempt %d error = %v", i, err)
		}
	}
	_, err := service.Unlock(context.Background(), shareToken, "1234", AccessMeta{}, false)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limited error = %v", err)
	}
}

func TestAccessMiniTicketIsSingleUse(t *testing.T) {
	service, _, shareToken := newAccessFixture(t)
	result, err := service.Unlock(context.Background(), shareToken, "1234", AccessMeta{}, true)
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	login, err := service.EnterTicket(context.Background(), result.Ticket)
	if err != nil {
		t.Fatalf("EnterTicket() error = %v", err)
	}
	if login.ShareToken != shareToken || login.SessionToken == "" {
		t.Fatalf("login = %+v", login)
	}
	if _, err := service.EnterTicket(context.Background(), result.Ticket); !errors.Is(err, ErrTicketInvalid) {
		t.Fatalf("second EnterTicket() error = %v", err)
	}
}
