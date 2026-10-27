package maintenance

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/database"
)

func TestPostgresCleanupUsesRetentionCutoffs(t *testing.T) {
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
	var redundantRetentionIndex int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE indexname = 'audit_events_retention_idx'`).Scan(&redundantRetentionIndex); err != nil {
		t.Fatalf("check audit retention index: %v", err)
	}
	if redundantRetentionIndex != 0 {
		t.Fatalf("redundant audit retention indexes = %d, want 0", redundantRetentionIndex)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	adminID, contentID, versionID, shareID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	oldAdminSessionID, newAdminSessionID := uuid.New(), uuid.New()
	oldAccessID, newAccessID := uuid.New(), uuid.New()
	oldTicketID, consumedTicketID, recentConsumedTicketID, newTicketID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	oldAuditID, newAuditID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_events WHERE id IN ($1,$2)", oldAuditID, newAuditID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM admin_users WHERE id = $1", adminID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM contents WHERE id = $1", contentID)
	})

	if _, err := pool.Exec(ctx, `INSERT INTO admin_users (id,email,credential_hash,role,status)
		VALUES ($1,$2,'test','ADMIN','ACTIVE')`, adminID, adminID.String()+"@cleanup.test"); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO contents (id,title,status,created_by)
		VALUES ($1,'cleanup fixture','ACTIVE',$2)`, contentID, adminID); err != nil {
		t.Fatalf("insert content: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO content_versions
		(id,content_id,version_no,object_key,sha256,byte_size,analysis_status,created_by)
		VALUES ($1,$2,1,$3,$4,1,'PASSED',$5)`,
		versionID, contentID, "cleanup/"+versionID.String()+".html", make([]byte, 32), adminID); err != nil {
		t.Fatalf("insert version: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO shares
		(id,content_version_id,type,label,token_hash,token_ciphertext,pin_hash,watermark_text,expires_at,created_by)
		VALUES ($1,$2,'PRIVATE','cleanup',$3,$4,'test','cleanup',$5,$6)`,
		shareID, versionID, shareID[:], []byte("cipher"), now.Add(time.Hour), adminID); err != nil {
		t.Fatalf("insert share: %v", err)
	}

	oldCutoff := now.Add(-31 * 24 * time.Hour)
	if _, err := pool.Exec(ctx, `INSERT INTO admin_sessions (id,admin_user_id,session_hash,expires_at,last_seen_at)
		VALUES ($1,$2,$3,$4,$4)`, oldAdminSessionID, adminID, oldAdminSessionID[:], oldCutoff); err != nil {
		t.Fatalf("insert expired admin session: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO admin_sessions (id,admin_user_id,session_hash,expires_at,last_seen_at)
		VALUES ($1,$2,$3,$4,$4)`, newAdminSessionID, adminID, newAdminSessionID[:], now.Add(time.Hour)); err != nil {
		t.Fatalf("insert current admin session: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO access_sessions (id,share_id,session_hash,expires_at,last_seen_at)
		VALUES ($1,$2,$3,$4,$4)`, oldAccessID, shareID, oldAccessID[:], oldCutoff); err != nil {
		t.Fatalf("insert expired access session: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO access_sessions (id,share_id,session_hash,expires_at,last_seen_at)
		VALUES ($1,$2,$3,$4,$4)`, newAccessID, shareID, newAccessID[:], now.Add(time.Hour)); err != nil {
		t.Fatalf("insert current access session: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO viewer_tickets (id,access_session_id,ticket_hash,session_ciphertext,expires_at)
		VALUES ($1,$2,$3,$4,$5)`, oldTicketID, oldAccessID, oldTicketID[:], []byte("cipher"), now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("insert expired ticket: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO viewer_tickets (id,access_session_id,ticket_hash,session_ciphertext,expires_at,consumed_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, consumedTicketID, newAccessID, consumedTicketID[:], []byte("cipher"), now.Add(time.Hour), now.Add(-31*24*time.Hour)); err != nil {
		t.Fatalf("insert consumed ticket: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO viewer_tickets (id,access_session_id,ticket_hash,session_ciphertext,expires_at,consumed_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, recentConsumedTicketID, newAccessID, recentConsumedTicketID[:], []byte("cipher"), now.Add(-48*time.Hour), now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("insert recently consumed expired ticket: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO viewer_tickets (id,access_session_id,ticket_hash,session_ciphertext,expires_at)
		VALUES ($1,$2,$3,$4,$5)`, newTicketID, newAccessID, newTicketID[:], []byte("cipher"), now.Add(time.Hour)); err != nil {
		t.Fatalf("insert current ticket: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO audit_events (id,actor_type,event_type,request_id,metadata_json,created_at)
		VALUES ($1,'SYSTEM','OLD',$2,'{}',$3)`, oldAuditID, "old-"+oldAuditID.String(), now.Add(-181*24*time.Hour)); err != nil {
		t.Fatalf("insert old audit event: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO audit_events (id,actor_type,event_type,request_id,metadata_json,created_at)
		VALUES ($1,'SYSTEM','NEW',$2,'{}',$3)`, newAuditID, "new-"+newAuditID.String(), now); err != nil {
		t.Fatalf("insert current audit event: %v", err)
	}
	counts, err := NewPostgresStore(pool).CleanupExpired(ctx, now)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if counts.AdminSessions < 1 || counts.AccessSessions < 1 || counts.ViewerTickets < 2 || counts.AuditEvents < 1 {
		t.Fatalf("cleanup counts = %+v", counts)
	}
	var oldAdmin, recentAdmin, oldAccess, recentAccess, oldTicket, consumedTicket, recentConsumedTicket, recentTicket, oldAudit, recentAudit int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admin_sessions WHERE id=$1`, oldAdminSessionID).Scan(&oldAdmin); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admin_sessions WHERE id=$1`, newAdminSessionID).Scan(&recentAdmin); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM access_sessions WHERE id=$1`, oldAccessID).Scan(&oldAccess); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM access_sessions WHERE id=$1`, newAccessID).Scan(&recentAccess); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM viewer_tickets WHERE id=$1`, oldTicketID).Scan(&oldTicket); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM viewer_tickets WHERE id=$1`, consumedTicketID).Scan(&consumedTicket); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM viewer_tickets WHERE id=$1`, recentConsumedTicketID).Scan(&recentConsumedTicket); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM viewer_tickets WHERE id=$1`, newTicketID).Scan(&recentTicket); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE id=$1`, oldAuditID).Scan(&oldAudit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE id=$1`, newAuditID).Scan(&recentAudit); err != nil {
		t.Fatal(err)
	}
	if oldAdmin != 0 || recentAdmin != 1 || oldAccess != 0 || recentAccess != 1 ||
		oldTicket != 0 || consumedTicket != 0 || recentConsumedTicket != 1 || recentTicket != 1 || oldAudit != 0 || recentAudit != 1 {
		t.Fatalf("rows remaining = old-admin:%d admin:%d old-access:%d access:%d old-ticket:%d old-consumed-ticket:%d recent-consumed-ticket:%d ticket:%d old-audit:%d audit:%d",
			oldAdmin, recentAdmin, oldAccess, recentAccess, oldTicket, consumedTicket, recentConsumedTicket, recentTicket, oldAudit, recentAudit)
	}
}
