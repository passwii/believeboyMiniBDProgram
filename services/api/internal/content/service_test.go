package content

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
)

const testAdminID = "44444444-4444-4444-8444-444444444444"

type fakeRepository struct {
	created   []PersistInput
	lastInput PersistInput
	createErr error
}

func (f *fakeRepository) Create(_ context.Context, input PersistInput) (Record, error) {
	f.lastInput = input
	if f.createErr != nil {
		return Record{}, f.createErr
	}
	f.created = append(f.created, input)
	return Record{
		ID:        input.ContentID,
		Title:     input.Title,
		Status:    "ACTIVE",
		VersionID: input.VersionID,
		VersionNo: 1,
		SHA256:    input.SHA256,
		ByteSize:  input.ByteSize,
		CreatedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeRepository) List(context.Context, int32, int32) ([]Record, error) {
	return nil, nil
}

func (f *fakeRepository) Get(context.Context, uuid.UUID) (Record, error) {
	return Record{}, ErrContentNotFound
}

type spyStore struct {
	inner   *storage.MemoryStore
	puts    []string
	deletes []string
}

func newSpyStore() *spyStore {
	return &spyStore{inner: storage.NewMemoryStore()}
}

func (s *spyStore) Put(ctx context.Context, key, contentType string, data []byte) error {
	s.puts = append(s.puts, key)
	return s.inner.Put(ctx, key, contentType, data)
}

func (s *spyStore) Get(ctx context.Context, key string) ([]byte, error) {
	return s.inner.Get(ctx, key)
}

func (s *spyStore) Delete(ctx context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	return s.inner.Delete(ctx, key)
}

func newTestService(t *testing.T) (*Service, *fakeRepository, *spyStore) {
	t.Helper()
	repository := &fakeRepository{}
	objectStore := newSpyStore()
	return NewService(repository, objectStore, 20<<20), repository, objectStore
}

func validHTML(title string) []byte {
	return []byte(`<!doctype html><html><head><title>` + title + `</title><meta name="viewport" content="width=device-width"><style>.x{color:red}</style></head><body><script>1+1</script></body></html>`)
}

func TestCreateContentPersistsObjectAndVersion(t *testing.T) {
	service, repository, objectStore := newTestService(t)
	data := validHTML("方案 A")

	result, err := service.Create(context.Background(), CreateInput{
		Filename: "proposal.html",
		Data:     data,
		AdminID:  uuid.MustParse(testAdminID),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Title != "方案 A" || result.VersionNo != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(repository.created) != 1 {
		t.Fatalf("repository calls = %d", len(repository.created))
	}
	input := repository.created[0]
	if !strings.HasPrefix(input.ObjectKey, "content/") || !strings.HasSuffix(input.ObjectKey, ".html") {
		t.Fatalf("ObjectKey = %q", input.ObjectKey)
	}
	stored, err := objectStore.Get(context.Background(), input.ObjectKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(stored) != string(data) {
		t.Fatal("stored object does not match upload")
	}
	expectedHash := sha256.Sum256(data)
	if string(result.SHA256) != string(expectedHash[:]) {
		t.Fatal("SHA256 mismatch")
	}
}

func TestCreateContentRejectsInvalidInputs(t *testing.T) {
	service, repository, _ := newTestService(t)
	adminID := uuid.MustParse(testAdminID)

	t.Run("too large", func(t *testing.T) {
		_, err := service.Create(context.Background(), CreateInput{
			Filename: "large.html",
			Data:     make([]byte, (20<<20)+1),
			AdminID:  adminID,
		})
		if !errors.Is(err, ErrFileTooLarge) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("wrong extension", func(t *testing.T) {
		_, err := service.Create(context.Background(), CreateInput{
			Filename: "proposal.zip",
			Data:     validHTML("bad"),
			AdminID:  adminID,
		})
		if !errors.Is(err, ErrUnsupportedFile) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("external resources", func(t *testing.T) {
		data := []byte(`<html><head><title>bad</title><script src="https://cdn.example/app.js"></script></head></html>`)
		_, err := service.Create(context.Background(), CreateInput{
			Filename: "proposal.html",
			Data:     data,
			AdminID:  adminID,
		})
		if !errors.Is(err, ErrExternalResources) {
			t.Fatalf("error = %v", err)
		}
	})

	if len(repository.created) != 0 {
		t.Fatalf("repository calls = %d", len(repository.created))
	}
}

func TestCreateContentDeletesObjectWhenDatabaseFails(t *testing.T) {
	service, repository, objectStore := newTestService(t)
	repository.createErr = errors.New("database offline")

	_, err := service.Create(context.Background(), CreateInput{
		Filename: "proposal.html",
		Data:     validHTML("cleanup"),
		AdminID:  uuid.MustParse(testAdminID),
	})
	if err == nil {
		t.Fatal("expected database error")
	}
	if len(repository.created) != 0 {
		t.Fatalf("repository calls = %d", len(repository.created))
	}
	if len(objectStore.puts) != 1 || len(objectStore.deletes) != 1 {
		t.Fatalf("puts/deletes = %v/%v", objectStore.puts, objectStore.deletes)
	}
	if objectStore.puts[0] != objectStore.deletes[0] {
		t.Fatalf("deleted %q, put %q", objectStore.deletes[0], objectStore.puts[0])
	}
	if _, err := objectStore.Get(context.Background(), objectStore.puts[0]); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("object remains after cleanup: %v", err)
	}
}

func TestCreateContentUsesFilenameWhenTitleMissing(t *testing.T) {
	service, repository, _ := newTestService(t)
	result, err := service.Create(context.Background(), CreateInput{
		Filename: "Fallback Name.html",
		Data:     []byte(`<html><body>content</body></html>`),
		AdminID:  uuid.MustParse(testAdminID),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Title != "Fallback Name" {
		t.Fatalf("Title = %q", result.Title)
	}
	if len(result.Analysis.Warnings) != 2 {
		t.Fatalf("Warnings = %v", result.Analysis.Warnings)
	}
	if len(repository.created) != 1 {
		t.Fatalf("repository calls = %d", len(repository.created))
	}
}
