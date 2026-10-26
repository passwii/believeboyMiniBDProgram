package content

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
)

var (
	ErrFileTooLarge    = errors.New("content file is too large")
	ErrUnsupportedFile = errors.New("only a single .html file is supported")
	ErrContentNotFound = errors.New("content not found")
)

type CreateInput struct {
	Filename string
	Data     []byte
	AdminID  uuid.UUID
}

type Content struct {
	ID        uuid.UUID
	Title     string
	Status    string
	VersionID uuid.UUID
	VersionNo int32
	SHA256    []byte
	ByteSize  int64
	Analysis  Analysis
	CreatedAt time.Time
}

type PersistInput struct {
	ContentID     uuid.UUID
	VersionID     uuid.UUID
	Title         string
	ObjectKey     string
	SHA256        []byte
	ByteSize      int64
	AnalysisNotes []byte
	AdminID       uuid.UUID
}

type Record struct {
	ID            uuid.UUID
	Title         string
	Status        string
	VersionID     uuid.UUID
	VersionNo     int32
	ObjectKey     string
	SHA256        []byte
	ByteSize      int64
	AnalysisNotes []byte
	CreatedAt     time.Time
}

type Repository interface {
	Create(ctx context.Context, input PersistInput) (Record, error)
	List(ctx context.Context, limit, offset int32) ([]Record, error)
	Get(ctx context.Context, id uuid.UUID) (Record, error)
}

type Service struct {
	repository     Repository
	objectStore    storage.Store
	maxUploadBytes int64
}

func NewService(repository Repository, objectStore storage.Store, maxUploadBytes int64) *Service {
	return &Service{
		repository:     repository,
		objectStore:    objectStore,
		maxUploadBytes: maxUploadBytes,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Content, error) {
	if int64(len(input.Data)) > s.maxUploadBytes {
		return Content{}, ErrFileTooLarge
	}
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(input.Filename)), ".html") {
		return Content{}, ErrUnsupportedFile
	}

	analysis, err := Analyze(input.Data, input.Filename)
	if err != nil {
		return Content{}, err
	}
	contentID := uuid.New()
	versionID := uuid.New()
	objectKey := fmt.Sprintf("content/%s/%s.html", contentID, versionID)
	digest := sha256.Sum256(input.Data)
	notes, err := json.Marshal(struct {
		ViewportPresent   bool     `json:"viewport_present"`
		ScriptCount       int      `json:"script_count"`
		ExternalResources []string `json:"external_resources"`
		Warnings          []string `json:"warnings"`
	}{
		ViewportPresent:   analysis.ViewportPresent,
		ScriptCount:       analysis.ScriptCount,
		ExternalResources: analysis.ExternalResources,
		Warnings:          analysis.Warnings,
	})
	if err != nil {
		return Content{}, fmt.Errorf("encode analysis notes: %w", err)
	}

	if err := s.objectStore.Put(ctx, objectKey, "text/html; charset=utf-8", input.Data); err != nil {
		return Content{}, fmt.Errorf("store content object: %w", err)
	}
	record, err := s.repository.Create(ctx, PersistInput{
		ContentID:     contentID,
		VersionID:     versionID,
		Title:         analysis.Title,
		ObjectKey:     objectKey,
		SHA256:        digest[:],
		ByteSize:      int64(len(input.Data)),
		AnalysisNotes: notes,
		AdminID:       input.AdminID,
	})
	if err != nil {
		cleanupErr := s.objectStore.Delete(ctx, objectKey)
		if cleanupErr != nil {
			return Content{}, errors.Join(fmt.Errorf("persist content: %w", err), fmt.Errorf("cleanup object: %w", cleanupErr))
		}
		return Content{}, fmt.Errorf("persist content: %w", err)
	}

	return Content{
		ID:        record.ID,
		Title:     record.Title,
		Status:    record.Status,
		VersionID: record.VersionID,
		VersionNo: record.VersionNo,
		SHA256:    record.SHA256,
		ByteSize:  record.ByteSize,
		Analysis:  analysis,
		CreatedAt: record.CreatedAt,
	}, nil
}

func (s *Service) List(ctx context.Context, limit, offset int32) ([]Content, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	records, err := s.repository.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list content: %w", err)
	}
	items := make([]Content, 0, len(records))
	for _, record := range records {
		analysis, err := decodeAnalysis(record.Title, record.AnalysisNotes)
		if err != nil {
			return nil, err
		}
		items = append(items, Content{
			ID:        record.ID,
			Title:     record.Title,
			Status:    record.Status,
			VersionID: record.VersionID,
			VersionNo: record.VersionNo,
			SHA256:    record.SHA256,
			ByteSize:  record.ByteSize,
			Analysis:  analysis,
			CreatedAt: record.CreatedAt,
		})
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Content, error) {
	record, err := s.repository.Get(ctx, id)
	if errors.Is(err, ErrContentNotFound) {
		return Content{}, ErrContentNotFound
	}
	if err != nil {
		return Content{}, fmt.Errorf("get content: %w", err)
	}
	analysis, err := decodeAnalysis(record.Title, record.AnalysisNotes)
	if err != nil {
		return Content{}, err
	}
	return Content{
		ID:        record.ID,
		Title:     record.Title,
		Status:    record.Status,
		VersionID: record.VersionID,
		VersionNo: record.VersionNo,
		SHA256:    record.SHA256,
		ByteSize:  record.ByteSize,
		Analysis:  analysis,
		CreatedAt: record.CreatedAt,
	}, nil
}

func decodeAnalysis(title string, notesJSON []byte) (Analysis, error) {
	analysis := Analysis{
		Title:             title,
		ExternalResources: []string{},
		Warnings:          []string{},
	}
	if len(notesJSON) == 0 {
		return analysis, nil
	}
	var notes struct {
		ViewportPresent   bool     `json:"viewport_present"`
		ScriptCount       int      `json:"script_count"`
		ExternalResources []string `json:"external_resources"`
		Warnings          []string `json:"warnings"`
	}
	if err := json.Unmarshal(notesJSON, &notes); err != nil {
		return Analysis{}, fmt.Errorf("decode analysis notes: %w", err)
	}
	analysis.ViewportPresent = notes.ViewportPresent
	analysis.ScriptCount = notes.ScriptCount
	if notes.ExternalResources != nil {
		analysis.ExternalResources = notes.ExternalResources
	}
	if notes.Warnings != nil {
		analysis.Warnings = notes.Warnings
	}
	return analysis, nil
}
