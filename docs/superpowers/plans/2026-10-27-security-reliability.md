# Security and Reliability Implementation Plan

Status: Implemented and verified on 2026-10-27.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Persist security-relevant audit events, rate-limit administrator logins, and periodically remove expired sessions, viewer tickets, and old audit rows.

**Architecture:** Add a small `audit` package that writes and lists audit records through sqlc. HTTP handlers record events using request IDs and keyed IP/User-Agent hashes. Add a process-local administrator login limiter and a PostgreSQL maintenance runner that executes bounded-retention cleanup once per hour.

**Tech Stack:** Go 1.27, Chi, pgx/v5, sqlc, PostgreSQL 17.

## Global Constraints

- Administrator login allows 5 failed attempts per IP in a rolling 10-minute window, then blocks that IP for 15 minutes.
- A successful login clears that IP's failed-attempt counter.
- The login limiter is in memory because the agreed first deployment runs a single API instance.
- Login IPs use the TCP peer by default; forwarded IPs are trusted only for configured `TRUSTED_PROXY_CIDRS`.
- Login attempts from the same IP are admitted one at a time; limiter state is periodically swept and capped.
- Viewer PIN limits, session IP hashes, and admin login limits use the same resolved client IP.
- Audit rows include request ID and keyed HMAC hashes for IP and User-Agent.
- Audit rows never contain credentials, PINs, raw share tokens, viewer cookies, or object keys.
- Audit writes are best effort: failures are logged without changing a completed operation's response.
- Audit retention is 180 days.
- Expired viewer tickets are removed 24 hours after expiry; consumed tickets are removed after 30 days.
- Expired or revoked admin and viewer sessions are removed after 30 days.
- Cleanup runs once per hour and also once when the service starts.
- Database cleanup uses PostgreSQL and preserves referential integrity.

---

### Task 1: Persist and Expose Audit Events

**Files:**
- Create: `services/api/db/queries/audit.sql`
- Regenerate: `services/api/internal/store/*.go`
- Create: `services/api/internal/audit/audit.go`
- Create: `services/api/internal/audit/postgres.go`
- Create: `services/api/internal/audit/audit_integration_test.go`
- Create: `services/api/internal/httpapi/audit.go`
- Create: `services/api/internal/httpapi/audit_test.go`
- Modify: `services/api/internal/httpapi/router.go`
- Modify: `services/api/cmd/server/main.go`

**Interfaces:**

```go
type Event struct {
    ActorType string
    ActorID   *uuid.UUID
    ShareID   *uuid.UUID
    EventType string
    RequestID string
    IPHash    []byte
    UserAgentHash []byte
    Metadata  json.RawMessage
    CreatedAt time.Time
}

type Recorder interface {
    Record(context.Context, Event) error
}
```

Routes:

```text
GET /api/v1/admin/audit-events?limit=50&offset=0
```

- [x] Test audit metadata serialization and insertion against the PostgreSQL migration.
- [x] Test the authenticated list endpoint and verify it never returns hashes or secret fields.
- [x] Implement sqlc insert/list queries, Postgres adapter, and the authenticated read route.
- [x] Wire one recorder into admin, content, share, and viewer handlers.
- [x] Record login success/failure, logout, upload accepted/rejected, share created/revoked/PIN reset, PIN success/failure/limited, ticket consumed, content URL issued, and content served.
- [x] Run `go test ./internal/audit ./internal/httpapi` and commit `feat(audit): persist security events`.

---

### Task 2: Rate-Limit Administrator Login

**Files:**
- Create: `services/api/internal/httpapi/admin_login_limiter.go`
- Create: `services/api/internal/httpapi/admin_login_limiter_test.go`
- Modify: `services/api/internal/httpapi/admin_auth.go`
- Modify: `services/api/internal/httpapi/admin_auth_test.go`
- Modify: `services/api/internal/httpapi/router.go`

**Interfaces:**

```go
type AdminLoginLimiter interface {
    Check(key string, now time.Time) time.Duration
    RecordFailure(key string, now time.Time)
    RecordSuccess(key string)
}
```

- [x] Test allowed initial attempts, blocking after five failures, cooldown expiry, and success reset.
- [x] Test HTTP `429`, `Retry-After`, and unchanged generic credential errors.
- [x] Hash the normalized remote IP with configured `IPHashKey` before using it as the limiter key.
- [x] Check the limiter after request-shape validation and before Argon2id verification.
- [x] Record failed attempts only for invalid credentials and clear failures on successful login.
- [x] Run `go test ./internal/httpapi` and commit `feat(security): rate limit admin login`.

---

### Task 3: Add Periodic Retention Cleanup

**Files:**
- Create: `services/api/db/queries/maintenance.sql`
- Regenerate: `services/api/internal/store/*.go`
- Create: `services/api/internal/maintenance/maintenance.go`
- Create: `services/api/internal/maintenance/postgres.go`
- Create: `services/api/internal/maintenance/maintenance_test.go`
- Create: `services/api/internal/maintenance/postgres_integration_test.go`
- Modify: `services/api/cmd/server/main.go`

**Interfaces:**

```go
type Store interface {
    CleanupExpired(context.Context, time.Time) (Counts, error)
}

func Run(ctx context.Context, store Store, logger *slog.Logger, interval time.Duration)
```

- [x] Test the cleanup runner's initial run, periodic runs, cancellation, and logged row counts with a real in-memory store.
- [x] Test SQL retention cleanup on PostgreSQL using expired and recent rows for each retained table.
- [x] Implement deletion rules: expired tickets after 24 hours, consumed tickets after 30 days, expired/revoked sessions after 30 days, audit rows after 180 days.
- [x] Run cleanup at process start and hourly; stop it with the server context.
- [x] Run `TEST_DATABASE_URL="$DATABASE_URL" go test -race ./...`, `go vet ./...`, `pnpm --dir apps/admin check`, and `pnpm --dir apps/viewer check`.
- [x] Commit `feat(maintenance): clean expired security records`.

## Self-Review

- Audit coverage includes all security and content operations listed above; no secrets are included in event payloads.
- Administrator login limiting is process-local and assumes the single-instance first deployment.
- Forwarded client IPs are accepted only from explicitly configured trusted proxy CIDRs.
- Per-IP login state is bounded and stale entries are removed by periodic sweeps.
- Viewer unlock metadata uses the same trusted client IP resolver as administrator login.
- Cleanup covers `admin_sessions`, `viewer_tickets`, `access_sessions`, and `audit_events` with explicit retention cutoffs.
- The admin audit API is included; a visual audit-log page is deferred to the next admin workflow pass.
