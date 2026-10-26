package adminauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/security"
)

const (
	testAdminID    = "11111111-1111-4111-8111-111111111111"
	testSessionID  = "22222222-2222-4222-8222-222222222222"
	testCredential = "correct horse battery staple"
	testAdminEmail = "Admin@Example.com"
	testSessionTTL = 12 * time.Hour
)

type fakeStore struct {
	admins   map[string]AdminRecord
	sessions map[string]SessionRecord
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		admins:   make(map[string]AdminRecord),
		sessions: make(map[string]SessionRecord),
	}
}

func (f *fakeStore) FindAdminByEmail(_ context.Context, email string) (AdminRecord, error) {
	admin, ok := f.admins[email]
	if !ok {
		return AdminRecord{}, ErrNotFound
	}
	return admin, nil
}

func (f *fakeStore) CreateSession(_ context.Context, input CreateSessionInput) error {
	f.sessions[string(input.SessionHash)] = SessionRecord{
		ID:          input.ID,
		AdminUserID: input.AdminUserID,
		ExpiresAt:   input.ExpiresAt,
		Email:       f.adminEmail(input.AdminUserID),
		Role:        f.adminRole(input.AdminUserID),
		Status:      f.adminStatus(input.AdminUserID),
	}
	return nil
}

func (f *fakeStore) FindSession(_ context.Context, hash []byte) (SessionRecord, error) {
	session, ok := f.sessions[string(hash)]
	if !ok {
		return SessionRecord{}, ErrNotFound
	}
	return session, nil
}

func (f *fakeStore) TouchSession(_ context.Context, id uuid.UUID, at time.Time) error {
	for hash, session := range f.sessions {
		if session.ID == id {
			session.LastSeenAt = at
			f.sessions[hash] = session
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeStore) RevokeSession(_ context.Context, id uuid.UUID, at time.Time) error {
	for hash, session := range f.sessions {
		if session.ID == id {
			session.RevokedAt = &at
			f.sessions[hash] = session
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeStore) adminEmail(id uuid.UUID) string {
	for _, admin := range f.admins {
		if admin.ID == id {
			return admin.Email
		}
	}
	return ""
}

func (f *fakeStore) adminRole(id uuid.UUID) string {
	for _, admin := range f.admins {
		if admin.ID == id {
			return admin.Role
		}
	}
	return ""
}

func (f *fakeStore) adminStatus(id uuid.UUID) string {
	for _, admin := range f.admins {
		if admin.ID == id {
			return admin.Status
		}
	}
	return ""
}

func newTestService(t *testing.T, now time.Time) (*Service, *fakeStore) {
	t.Helper()
	hash, err := security.HashCredential(testCredential)
	if err != nil {
		t.Fatalf("HashCredential() error = %v", err)
	}

	store := newFakeStore()
	adminID := uuid.MustParse(testAdminID)
	store.admins["admin@example.com"] = AdminRecord{
		ID:             adminID,
		Email:          testAdminEmail,
		CredentialHash: hash,
		Role:           "ADMIN",
		Status:         "ACTIVE",
	}

	service := New(store, testSessionTTL)
	service.now = func() time.Time { return now }
	service.newSessionID = func() uuid.UUID { return uuid.MustParse(testSessionID) }
	return service, store
}

func TestLoginCreatesSession(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	service, store := newTestService(t, now)

	result, err := service.Login(context.Background(), testAdminEmail, testCredential, SessionMeta{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if len(result.Token) != 43 {
		t.Fatalf("token length = %d, want 43", len(result.Token))
	}
	if !result.ExpiresAt.Equal(now.Add(testSessionTTL)) {
		t.Fatalf("ExpiresAt = %v", result.ExpiresAt)
	}
	if result.Principal.Email != testAdminEmail || result.Principal.Role != "ADMIN" {
		t.Fatalf("Principal = %+v", result.Principal)
	}
	session, ok := store.sessions[string(security.HashToken(result.Token))]
	if !ok {
		t.Fatal("session was not stored under token hash")
	}
	if session.ID != uuid.MustParse(testSessionID) {
		t.Fatalf("session ID = %v", session.ID)
	}
}

func TestLoginUsesUniformErrorForUnknownEmailAndWrongCredential(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	service, _ := newTestService(t, now)

	_, unknownErr := service.Login(context.Background(), "missing@example.com", testCredential, SessionMeta{})
	_, wrongErr := service.Login(context.Background(), testAdminEmail, "wrong", SessionMeta{})

	if !errors.Is(unknownErr, ErrInvalidCredentials) {
		t.Fatalf("unknown email error = %v", unknownErr)
	}
	if !errors.Is(wrongErr, ErrInvalidCredentials) {
		t.Fatalf("wrong credential error = %v", wrongErr)
	}
}

func TestLoginRejectsDisabledAdmin(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	service, store := newTestService(t, now)
	admin := store.admins["admin@example.com"]
	admin.Status = "DISABLED"
	store.admins["admin@example.com"] = admin

	_, err := service.Login(context.Background(), testAdminEmail, testCredential, SessionMeta{})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v", err)
	}
}

func TestAuthenticateTouchesValidSession(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	service, store := newTestService(t, now)
	login, err := service.Login(context.Background(), testAdminEmail, testCredential, SessionMeta{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	principal, err := service.Authenticate(context.Background(), login.Token, SessionMeta{})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.Email != testAdminEmail {
		t.Fatalf("principal = %+v", principal)
	}
	session := store.sessions[string(security.HashToken(login.Token))]
	if !session.LastSeenAt.Equal(now) {
		t.Fatalf("LastSeenAt = %v", session.LastSeenAt)
	}
}

func TestAuthenticateRejectsExpiredRevokedAndDisabled(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	service, store := newTestService(t, now)
	login, err := service.Login(context.Background(), testAdminEmail, testCredential, SessionMeta{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	hash := string(security.HashToken(login.Token))

	t.Run("expired", func(t *testing.T) {
		session := store.sessions[hash]
		session.ExpiresAt = now.Add(-time.Second)
		store.sessions[hash] = session
		_, err := service.Authenticate(context.Background(), login.Token, SessionMeta{})
		if !errors.Is(err, ErrSessionInvalid) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		session := store.sessions[hash]
		session.ExpiresAt = now.Add(time.Hour)
		session.RevokedAt = &now
		store.sessions[hash] = session
		_, err := service.Authenticate(context.Background(), login.Token, SessionMeta{})
		if !errors.Is(err, ErrSessionInvalid) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		session := store.sessions[hash]
		session.RevokedAt = nil
		session.Status = "DISABLED"
		store.sessions[hash] = session
		_, err := service.Authenticate(context.Background(), login.Token, SessionMeta{})
		if !errors.Is(err, ErrSessionInvalid) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestLogoutRevokesSession(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	service, store := newTestService(t, now)
	login, err := service.Login(context.Background(), testAdminEmail, testCredential, SessionMeta{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := service.Logout(context.Background(), login.Token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	session := store.sessions[string(security.HashToken(login.Token))]
	if session.RevokedAt == nil || !session.RevokedAt.Equal(now) {
		t.Fatalf("RevokedAt = %v", session.RevokedAt)
	}
}
