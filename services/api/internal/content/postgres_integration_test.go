package content

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/database"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

func TestPostgresRepositoryRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if err := database.Migrate(databaseURL, "up"); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	adminID := uuid.New()
	hash, err := security.HashCredential("integration-test-credential")
	if err != nil {
		t.Fatalf("HashCredential() error = %v", err)
	}
	if _, err := store.New(pool).CreateAdmin(ctx, store.CreateAdminParams{
		ID:             adminID,
		Email:          adminID.String() + "@example.com",
		CredentialHash: hash,
		Role:           "ADMIN",
		Status:         "ACTIVE",
	}); err != nil {
		t.Fatalf("CreateAdmin() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM contents WHERE created_by = $1", adminID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM admin_users WHERE id = $1", adminID)
	})

	repository := NewPostgresRepository(pool)
	service := NewService(repository, storage.NewMemoryStore(), 20<<20)
	created, err := service.Create(ctx, CreateInput{
		Filename: "integration.html",
		Data:     validHTML("Integration"),
		AdminID:  adminID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	listed, err := service.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) == 0 {
		t.Fatal("List() returned no content")
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Title != "Integration" || got.VersionNo != 1 {
		t.Fatalf("Get() = %+v", got)
	}
	if got.Analysis.ScriptCount != 1 {
		t.Fatalf("ScriptCount = %d", got.Analysis.ScriptCount)
	}
}
