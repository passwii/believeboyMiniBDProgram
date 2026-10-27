package share

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
)

var (
	ErrPINRejected        = errors.New("PIN rejected")
	ErrShareExpired       = errors.New("share expired")
	ErrShareRevoked       = errors.New("share revoked")
	ErrRateLimited        = errors.New("rate limited")
	ErrSessionInvalid     = errors.New("viewer session invalid")
	ErrTicketInvalid      = errors.New("viewer ticket invalid")
	ErrContentUnavailable = errors.New("content unavailable")
)

type AccessMeta struct {
	IPHash        []byte
	UserAgentHash []byte
}

type ShareStatus struct {
	ShareID   uuid.UUID
	State     string
	Type      string
	Label     string
	ExpiresAt time.Time
}

type UnlockResult struct {
	SessionToken     string
	SessionExpiresAt time.Time
	Ticket           string
	Share            ShareRecord
}

type ContentURLResult struct {
	URL           string
	WatermarkText string
	ExpiresAt     time.Time
}

type TicketLoginResult struct {
	SessionToken     string
	ShareToken       string
	SessionExpiresAt time.Time
}

type AccessService struct {
	repository    Repository
	cipher        *security.Cipher
	limiter       *Limiter
	signingKey    []byte
	contentOrigin string
	sessionTTL    time.Duration
	ticketTTL     time.Duration
	contentTTL    time.Duration
	contentStore  storage.Store
	now           func() time.Time
}

func NewAccessService(
	repository Repository,
	cipher *security.Cipher,
	limiter *Limiter,
	signingKey []byte,
	contentOrigin string,
	sessionTTL time.Duration,
	ticketTTL time.Duration,
	contentTTL time.Duration,
	contentStore storage.Store,
) *AccessService {
	return &AccessService{
		repository: repository, cipher: cipher, limiter: limiter, signingKey: signingKey,
		contentOrigin: contentOrigin, sessionTTL: sessionTTL, ticketTTL: ticketTTL,
		contentTTL: contentTTL, contentStore: contentStore, now: time.Now,
	}
}

func (s *AccessService) Status(ctx context.Context, rawToken string) (ShareStatus, error) {
	record, err := s.repository.GetShareByTokenHash(ctx, security.HashToken(rawToken))
	if err != nil {
		return ShareStatus{}, err
	}
	now := s.now().UTC()
	state := "ACTIVE"
	if record.RevokedAt != nil {
		state = "REVOKED"
	} else if !now.Before(record.ExpiresAt) {
		state = "EXPIRED"
	}
	return ShareStatus{
		ShareID: record.ID, State: state, Type: record.Type,
		Label: record.Label, ExpiresAt: record.ExpiresAt,
	}, nil
}

func (s *AccessService) Unlock(
	ctx context.Context,
	rawToken, pin string,
	meta AccessMeta,
	issueTicket bool,
) (UnlockResult, error) {
	record, err := s.repository.GetShareByTokenHash(ctx, security.HashToken(rawToken))
	if err != nil {
		return UnlockResult{}, err
	}
	now := s.now().UTC()
	if record.RevokedAt != nil {
		return UnlockResult{}, ErrShareRevoked
	}
	if !now.Before(record.ExpiresAt) {
		return UnlockResult{}, ErrShareExpired
	}
	ipID := hex.EncodeToString(meta.IPHash)
	if decision := s.limiter.Check(record.ID, ipID, now); !decision.Allowed {
		return UnlockResult{}, &RateLimitError{RetryAfter: decision.RetryAfter}
	}
	valid, err := security.VerifyCredential(record.PINHash, pin)
	if err != nil || !valid {
		s.limiter.RecordFailure(record.ID, ipID, now)
		return UnlockResult{}, ErrPINRejected
	}
	s.limiter.RecordSuccess(record.ID, ipID)

	sessionTTL := record.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = s.sessionTTL
	}
	expiresAt := now.Add(sessionTTL)
	if record.ExpiresAt.Before(expiresAt) {
		expiresAt = record.ExpiresAt
	}
	sessionRaw, sessionHash, err := security.NewToken()
	if err != nil {
		return UnlockResult{}, fmt.Errorf("create session token: %w", err)
	}
	session, err := s.repository.CreateSession(ctx, SessionInput{
		ID: uuid.New(), ShareID: record.ID, SessionHash: sessionHash,
		ExpiresAt: expiresAt, LastSeenAt: now,
		IPHash: meta.IPHash, UserAgentHash: meta.UserAgentHash,
	})
	if err != nil {
		return UnlockResult{}, err
	}

	result := UnlockResult{
		SessionToken: sessionRaw, SessionExpiresAt: expiresAt, Share: record,
	}
	if !issueTicket {
		return result, nil
	}
	ticketRaw, ticketHash, err := security.NewToken()
	if err != nil {
		return UnlockResult{}, fmt.Errorf("create viewer ticket: %w", err)
	}
	sessionCiphertext, err := s.cipher.Encrypt([]byte(sessionRaw))
	if err != nil {
		return UnlockResult{}, fmt.Errorf("encrypt session token: %w", err)
	}
	if err := s.repository.CreateTicket(ctx, TicketInput{
		ID: uuid.New(), AccessSessionID: session.ID, TicketHash: ticketHash,
		SessionCiphertext: sessionCiphertext, ExpiresAt: now.Add(s.ticketTTL),
	}); err != nil {
		return UnlockResult{}, err
	}
	result.Ticket = ticketRaw
	return result, nil
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s: %s", ErrRateLimited, e.RetryAfter)
}

func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

func (s *AccessService) Authenticate(ctx context.Context, rawSession string) (SessionRecord, error) {
	record, err := s.repository.GetSessionByHash(ctx, security.HashToken(rawSession))
	if err != nil {
		return SessionRecord{}, ErrSessionInvalid
	}
	now := s.now().UTC()
	if record.RevokedAt != nil || record.ShareRevokedAt != nil ||
		!now.Before(record.ExpiresAt) || !now.Before(record.ShareExpiresAt) {
		return SessionRecord{}, ErrSessionInvalid
	}
	if err := s.repository.TouchSession(ctx, record.ID, now); err != nil {
		return SessionRecord{}, fmt.Errorf("touch session: %w", err)
	}
	return record, nil
}

func (s *AccessService) ContentURL(ctx context.Context, rawSession string) (ContentURLResult, error) {
	session, err := s.Authenticate(ctx, rawSession)
	if err != nil {
		return ContentURLResult{}, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.contentTTL)
	if session.ExpiresAt.Before(expiresAt) {
		expiresAt = session.ExpiresAt
	}
	token, err := SignClaims(ContentClaims{
		ShareID: session.ShareID, VersionID: session.ContentVersionID,
		SessionID: session.ID, ExpiresAt: expiresAt.Unix(),
	}, s.signingKey)
	if err != nil {
		return ContentURLResult{}, err
	}
	return ContentURLResult{
		URL:           s.contentOrigin + "/c/" + url.PathEscape(token),
		WatermarkText: session.WatermarkText, ExpiresAt: expiresAt,
	}, nil
}

func (s *AccessService) EnterTicket(ctx context.Context, rawTicket string) (TicketLoginResult, error) {
	now := s.now().UTC()
	ticket, err := s.repository.ConsumeTicket(ctx, security.HashToken(rawTicket), now)
	if err != nil {
		return TicketLoginResult{}, ErrTicketInvalid
	}
	sessionRaw, err := s.cipher.Decrypt(ticket.SessionCiphertext)
	if err != nil {
		return TicketLoginResult{}, ErrTicketInvalid
	}
	shareRaw, err := s.cipher.Decrypt(ticket.ShareTokenCiphertext)
	if err != nil {
		return TicketLoginResult{}, ErrTicketInvalid
	}
	session, err := s.repository.GetSessionByID(ctx, ticket.AccessSessionID)
	if err != nil || session.RevokedAt != nil || !now.Before(session.ExpiresAt) ||
		session.ShareRevokedAt != nil || !now.Before(session.ShareExpiresAt) {
		return TicketLoginResult{}, ErrTicketInvalid
	}
	return TicketLoginResult{
		SessionToken: string(sessionRaw), ShareToken: string(shareRaw),
		SessionExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *AccessService) ReadContent(ctx context.Context, signedToken string) ([]byte, error) {
	claims, err := VerifyClaims(signedToken, s.signingKey)
	if err != nil {
		return nil, ErrSessionInvalid
	}
	session, err := s.repository.GetSessionByID(ctx, claims.SessionID)
	if err != nil || session.ShareID != claims.ShareID || session.ContentVersionID != claims.VersionID {
		return nil, ErrSessionInvalid
	}
	now := s.now().UTC()
	if session.RevokedAt != nil || session.ShareRevokedAt != nil ||
		!now.Before(session.ExpiresAt) || !now.Before(session.ShareExpiresAt) {
		return nil, ErrSessionInvalid
	}
	data, err := s.contentStore.Get(ctx, session.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrContentUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("read content object: %w", err)
	}
	return data, nil
}
