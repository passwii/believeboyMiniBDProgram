package share

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/content"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
)

var (
	ErrShareNotFound    = errors.New("share not found")
	ErrInvalidShareType = errors.New("invalid share type")
	ErrInvalidPIN       = errors.New("PIN must contain exactly four digits")
	ErrInvalidExpiry    = errors.New("share expiry must be in the future")
	ErrInvalidLabel     = errors.New("share label is required")
)

var pinPattern = regexp.MustCompile(`^[0-9]{4}$`)

type Share struct {
	ID               uuid.UUID
	ContentID        uuid.UUID
	ContentVersionID uuid.UUID
	ContentTitle     string
	Type             string
	Label            string
	Token            string
	WatermarkText    string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	SessionTTL       time.Duration
	CreatedAt        time.Time
}

type CreateInput struct {
	ContentID     uuid.UUID
	Type          string
	Label         string
	PIN           string
	WatermarkText string
	ExpiresAt     time.Time
	SessionTTL    time.Duration
	AdminID       uuid.UUID
}

type ShareRecord struct {
	ID               uuid.UUID
	ContentID        uuid.UUID
	ContentVersionID uuid.UUID
	ContentTitle     string
	Type             string
	Label            string
	TokenHash        []byte
	TokenCiphertext  []byte
	PINHash          string
	WatermarkText    string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	SessionTTL       time.Duration
	CreatedAt        time.Time
}

type PersistShareInput struct {
	ID               uuid.UUID
	ContentVersionID uuid.UUID
	Type             string
	Label            string
	TokenHash        []byte
	TokenCiphertext  []byte
	PINHash          string
	WatermarkText    string
	ExpiresAt        time.Time
	SessionTTL       time.Duration
	AdminID          uuid.UUID
}

type SessionInput struct {
	ID            uuid.UUID
	ShareID       uuid.UUID
	SessionHash   []byte
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	IPHash        []byte
	UserAgentHash []byte
}

type SessionRecord struct {
	ID               uuid.UUID
	ShareID          uuid.UUID
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	LastSeenAt       time.Time
	ShareExpiresAt   time.Time
	ShareRevokedAt   *time.Time
	Type             string
	Label            string
	WatermarkText    string
	ContentVersionID uuid.UUID
	ObjectKey        string
	ContentTitle     string
}

type TicketInput struct {
	ID                uuid.UUID
	AccessSessionID   uuid.UUID
	TicketHash        []byte
	SessionCiphertext []byte
	ExpiresAt         time.Time
}

type TicketRecord struct {
	AccessSessionID      uuid.UUID
	SessionCiphertext    []byte
	ShareID              uuid.UUID
	ShareTokenCiphertext []byte
}

type Repository interface {
	CreateShare(ctx context.Context, input PersistShareInput) (ShareRecord, error)
	ListShares(ctx context.Context, limit, offset int32) ([]ShareRecord, error)
	GetShareByTokenHash(ctx context.Context, tokenHash []byte) (ShareRecord, error)
	GetShareByID(ctx context.Context, id uuid.UUID) (ShareRecord, error)
	RevokeShareAndSessions(ctx context.Context, id uuid.UUID, at time.Time) error
	ResetPINAndRevokeSessions(ctx context.Context, id uuid.UUID, pinHash string, at time.Time) error
	CreateSession(ctx context.Context, input SessionInput) (SessionRecord, error)
	GetSessionByHash(ctx context.Context, sessionHash []byte) (SessionRecord, error)
	GetSessionByID(ctx context.Context, id uuid.UUID) (SessionRecord, error)
	TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error
	CreateTicket(ctx context.Context, input TicketInput) error
	ConsumeTicket(ctx context.Context, ticketHash []byte, at time.Time) (TicketRecord, error)
}

type ContentLookup interface {
	Get(ctx context.Context, id uuid.UUID) (content.Content, error)
}

type Service struct {
	repository Repository
	content    ContentLookup
	cipher     *security.Cipher
	now        func() time.Time
	newID      func() uuid.UUID
}

func NewService(repository Repository, contentLookup ContentLookup, cipher *security.Cipher) *Service {
	return &Service{
		repository: repository,
		content:    contentLookup,
		cipher:     cipher,
		now:        time.Now,
		newID:      uuid.New,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Share, error) {
	shareType := strings.ToUpper(strings.TrimSpace(input.Type))
	if shareType != "PUBLIC" && shareType != "PRIVATE" {
		return Share{}, ErrInvalidShareType
	}
	if !pinPattern.MatchString(input.PIN) {
		return Share{}, ErrInvalidPIN
	}
	label := strings.TrimSpace(input.Label)
	if label == "" || len(label) > 120 {
		return Share{}, ErrInvalidLabel
	}
	expiresAt := input.ExpiresAt.UTC()
	if !expiresAt.After(s.now().UTC()) {
		return Share{}, ErrInvalidExpiry
	}
	sessionTTL := input.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = 30 * time.Minute
	}
	if sessionTTL < time.Minute || sessionTTL > 2*time.Hour {
		return Share{}, errors.New("session TTL must be between 1 minute and 2 hours")
	}

	item, err := s.content.Get(ctx, input.ContentID)
	if errors.Is(err, content.ErrContentNotFound) {
		return Share{}, ErrShareNotFound
	}
	if err != nil {
		return Share{}, fmt.Errorf("load content: %w", err)
	}

	rawToken, tokenHash, err := security.NewToken()
	if err != nil {
		return Share{}, fmt.Errorf("create share token: %w", err)
	}
	tokenCiphertext, err := s.cipher.Encrypt([]byte(rawToken))
	if err != nil {
		return Share{}, fmt.Errorf("encrypt share token: %w", err)
	}
	pinHash, err := security.HashCredential(input.PIN)
	if err != nil {
		return Share{}, fmt.Errorf("hash PIN: %w", err)
	}
	watermark := strings.TrimSpace(input.WatermarkText)
	if watermark == "" {
		watermark = label
	}
	if len(watermark) > 200 {
		return Share{}, errors.New("watermark text is too long")
	}

	record, err := s.repository.CreateShare(ctx, PersistShareInput{
		ID:               s.newID(),
		ContentVersionID: item.VersionID,
		Type:             shareType,
		Label:            label,
		TokenHash:        tokenHash,
		TokenCiphertext:  tokenCiphertext,
		PINHash:          pinHash,
		WatermarkText:    watermark,
		ExpiresAt:        expiresAt,
		SessionTTL:       sessionTTL,
		AdminID:          input.AdminID,
	})
	if err != nil {
		return Share{}, fmt.Errorf("persist share: %w", err)
	}
	record.ContentID = item.ID
	record.ContentTitle = item.Title
	return s.toShare(record, rawToken), nil
}

func (s *Service) List(ctx context.Context, limit, offset int32) ([]Share, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	records, err := s.repository.ListShares(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	shares := make([]Share, 0, len(records))
	for _, record := range records {
		raw, err := s.cipher.Decrypt(record.TokenCiphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt share token: %w", err)
		}
		shares = append(shares, s.toShare(record, string(raw)))
	}
	return shares, nil
}

func (s *Service) Revoke(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repository.GetShareByID(ctx, id); err != nil {
		return err
	}
	return s.repository.RevokeShareAndSessions(ctx, id, s.now().UTC())
}

func (s *Service) ResetPIN(ctx context.Context, id uuid.UUID, pin string) error {
	if !pinPattern.MatchString(pin) {
		return ErrInvalidPIN
	}
	if _, err := s.repository.GetShareByID(ctx, id); err != nil {
		return err
	}
	pinHash, err := security.HashCredential(pin)
	if err != nil {
		return fmt.Errorf("hash PIN: %w", err)
	}
	return s.repository.ResetPINAndRevokeSessions(ctx, id, pinHash, s.now().UTC())
}

func (s *Service) toShare(record ShareRecord, rawToken string) Share {
	return Share{
		ID:               record.ID,
		ContentID:        record.ContentID,
		ContentVersionID: record.ContentVersionID,
		ContentTitle:     record.ContentTitle,
		Type:             record.Type,
		Label:            record.Label,
		Token:            rawToken,
		WatermarkText:    record.WatermarkText,
		ExpiresAt:        record.ExpiresAt,
		RevokedAt:        record.RevokedAt,
		SessionTTL:       record.SessionTTL,
		CreatedAt:        record.CreatedAt,
	}
}
