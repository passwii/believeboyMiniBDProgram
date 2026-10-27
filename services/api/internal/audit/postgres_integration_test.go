package audit

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/database"
)

func TestPostgresRecordAndList(t *testing.T) {
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

	requestID := "audit-test-" + uuid.NewString()
	actorID := uuid.New()
	shareID := uuid.New()
	contentID := uuid.New()
	versionID := uuid.New()
	now := time.Now().UTC()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_events WHERE request_id = $1", requestID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM shares WHERE id = $1", shareID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM contents WHERE id = $1", contentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM admin_users WHERE id = $1", actorID)
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_users (id, email, credential_hash, role, status)
		VALUES ($1, $2, 'test-hash', 'ADMIN', 'ACTIVE')`,
		actorID, actorID.String()+"@audit.test"); err != nil {
		t.Fatalf("insert test admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contents (id, title, status, created_by)
		VALUES ($1, 'Audit test', 'ACTIVE', $2)`, contentID, actorID); err != nil {
		t.Fatalf("insert test content: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO content_versions (
			id, content_id, version_no, object_key, sha256, byte_size,
			analysis_status, created_by
		) VALUES ($1, $2, 1, $3, $4, 1, 'PASSED', $5)`,
		versionID, contentID, "audit-test/"+versionID.String()+".html", make([]byte, 32), actorID); err != nil {
		t.Fatalf("insert test content version: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO shares (
			id, content_version_id, type, label, token_hash, token_ciphertext,
			pin_hash, watermark_text, expires_at, created_by
		) VALUES ($1, $2, 'PRIVATE', 'Audit test', $3, $4, 'test-pin', 'test', $5, $6)`,
		shareID, versionID, shareID[:], []byte("encrypted"), now.Add(time.Hour), actorID); err != nil {
		t.Fatalf("insert test share: %v", err)
	}
	service := NewService(NewPostgresStore(pool))
	err = service.Record(ctx, Event{
		ActorType: ActorAdmin, ActorID: &actorID, ShareID: &shareID,
		EventType: "TEST_EVENT", RequestID: requestID,
		IPHash: []byte{1, 2, 3}, UserAgentHash: []byte{4, 5, 6},
		Metadata: json.RawMessage(`{"result":"ok"}`),
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	rows, err := service.List(ctx, 20, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var found *Record
	for index := range rows {
		if rows[index].RequestID == requestID {
			found = &rows[index]
			break
		}
	}
	if found == nil {
		t.Fatal("recorded audit event was not returned")
	}
	if found.ActorID == nil || *found.ActorID != actorID || found.ShareID == nil || *found.ShareID != shareID {
		t.Fatalf("record = %+v", found)
	}
	if string(found.Metadata) != `{"result": "ok"}` && string(found.Metadata) != `{"result":"ok"}` {
		t.Fatalf("metadata = %s", found.Metadata)
	}
	if found.CreatedAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("CreatedAt = %v", found.CreatedAt)
	}
}
