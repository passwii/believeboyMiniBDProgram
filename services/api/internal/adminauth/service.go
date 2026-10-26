package adminauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/security"
)

var (
	ErrNotFound           = errors.New("admin auth record not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionInvalid     = errors.New("session invalid")
)

type AdminRecord struct {
	ID             uuid.UUID
	Email          string
	CredentialHash string
	Role           string
	Status         string
}

type SessionRecord struct {
	ID          uuid.UUID
	AdminUserID uuid.UUID
	ExpiresAt   time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
	Email       string
	Role        string
	Status      string
}

type CreateSessionInput struct {
	ID            uuid.UUID
	AdminUserID   uuid.UUID
	SessionHash   []byte
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	IPHash        []byte
	UserAgentHash []byte
}

type SessionMeta struct {
	IPHash        []byte
	UserAgentHash []byte
}

type Store interface {
	FindAdminByEmail(ctx context.Context, email string) (AdminRecord, error)
	CreateSession(ctx context.Context, input CreateSessionInput) error
	FindSession(ctx context.Context, hash []byte) (SessionRecord, error)
	TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeSession(ctx context.Context, id uuid.UUID, at time.Time) error
}

type Principal struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	Principal Principal
}

type Service struct {
	store        Store
	sessionTTL   time.Duration
	now          func() time.Time
	newSessionID func() uuid.UUID
}

func New(store Store, sessionTTL time.Duration) *Service {
	return &Service{
		store:        store,
		sessionTTL:   sessionTTL,
		now:          time.Now,
		newSessionID: uuid.New,
	}
}

func (s *Service) Login(ctx context.Context, email, credential string, meta SessionMeta) (LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	admin, err := s.store.FindAdminByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_, _ = security.VerifyCredential(dummyCredentialHash(), credential)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("find admin: %w", err)
	}

	valid, err := security.VerifyCredential(admin.CredentialHash, credential)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify credential: %w", err)
	}
	if !valid || admin.Status != "ACTIVE" {
		return LoginResult{}, ErrInvalidCredentials
	}

	rawToken, tokenHash, err := security.NewToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("create session token: %w", err)
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.sessionTTL)
	if err := s.store.CreateSession(ctx, CreateSessionInput{
		ID:            s.newSessionID(),
		AdminUserID:   admin.ID,
		SessionHash:   tokenHash,
		ExpiresAt:     expiresAt,
		LastSeenAt:    now,
		IPHash:        meta.IPHash,
		UserAgentHash: meta.UserAgentHash,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("create admin session: %w", err)
	}

	return LoginResult{
		Token:     rawToken,
		ExpiresAt: expiresAt,
		Principal: Principal{ID: admin.ID, Email: admin.Email, Role: admin.Role},
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, rawToken string, meta SessionMeta) (Principal, error) {
	if strings.TrimSpace(rawToken) == "" {
		return Principal{}, ErrSessionInvalid
	}
	session, err := s.store.FindSession(ctx, security.HashToken(rawToken))
	if errors.Is(err, ErrNotFound) {
		return Principal{}, ErrSessionInvalid
	}
	if err != nil {
		return Principal{}, fmt.Errorf("find admin session: %w", err)
	}

	now := s.now().UTC()
	if session.RevokedAt != nil || !now.Before(session.ExpiresAt) || session.Status != "ACTIVE" {
		return Principal{}, ErrSessionInvalid
	}
	if err := s.store.TouchSession(ctx, session.ID, now); err != nil {
		return Principal{}, fmt.Errorf("touch admin session: %w", err)
	}
	return Principal{ID: session.AdminUserID, Email: session.Email, Role: session.Role}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	session, err := s.store.FindSession(ctx, security.HashToken(rawToken))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find admin session: %w", err)
	}
	if err := s.store.RevokeSession(ctx, session.ID, s.now().UTC()); errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	return nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

func dummyCredentialHash() string {
	dummyHashOnce.Do(func() {
		dummyHash, _ = security.HashCredential("dummy credential used only for timing")
	})
	return dummyHash
}
