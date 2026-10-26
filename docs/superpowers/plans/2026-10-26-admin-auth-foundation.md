# Admin Auth Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Initialize the monorepo and deliver a working Go API foundation with PostgreSQL migrations, health checks, Argon2id administrator credentials, session cookies, admin login/me/logout endpoints, and a CLI for creating the first administrator.

**Architecture:** Create a single repository with pnpm workspace metadata for future web applications and a Go service under `services/api`. PostgreSQL is the durable store, sqlc generates typed queries, goose applies embedded migrations, and Chi exposes same-origin admin APIs protected by HttpOnly session cookies.

**Tech Stack:** Go 1.27, Chi, pgx/v5, sqlc, goose, Argon2id, PostgreSQL 17, Docker Compose, pnpm 10.

## Global Constraints

- Only `ADMIN` and `STAFF` roles exist. No customer accounts or customer profile tables.
- Administrator passwords use Argon2id with memory 64 MiB, time 3, parallelism 2, 16-byte salt, and 32-byte key.
- Admin session cookie is HttpOnly, Secure in production, SameSite=Lax, and defaults to a 12-hour absolute lifetime.
- No default administrator email or password is created automatically.
- All timestamps are stored in UTC and rendered as `Asia/Shanghai` by clients.
- Upload limit remains 20 MiB, but upload endpoints are outside this milestone.
- Share tokens, PINs, content signing, H5, Next.js, and mini program code are outside this milestone.
- Environment secrets must not be committed. `.env.example` contains development-only values.

---

### Task 1: Repository Bootstrap and Local Infrastructure

**Files:**
- Create: `.gitignore`
- Create: `.editorconfig`
- Create: `.env.example`
- Create: `package.json`
- Create: `pnpm-workspace.yaml`
- Create: `Makefile`
- Create: `README.md`
- Create: `infra/compose.yaml`
- Create: `infra/Caddyfile`
- Create: `go.work`

**Interfaces:**
- Consumes: existing design document.
- Produces: `make up`, `make down`, `make psql`, `make migrate`, `make test`, and a pnpm workspace rooted at the repository.

- [ ] **Step 1: Create repository configuration files**

`.gitignore`:

```gitignore
.DS_Store
.env
.env.*
!.env.example
node_modules/
.next/
dist/
coverage/
playwright-report/
test-results/
*.log
services/api/bin/
services/api/tmp/
.codegraph/
```

`.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
indent_style = space
indent_size = 2
trim_trailing_whitespace = true

[*.go]
indent_style = tab
indent_size = 4

[Makefile]
indent_style = tab
```

`.env.example`:

```dotenv
API_ADDR=:8080
DATABASE_URL=postgres://believeboy:believeboy@localhost:5433/believeboy?sslmode=disable
ADMIN_ORIGIN=http://admin.localhost:3000
VIEW_ORIGIN=http://view.localhost:5173
CONTENT_ORIGIN=http://content.localhost:8080
API_ORIGIN=http://api.localhost:8080
ADMIN_COOKIE_NAME=bb_admin_session
ADMIN_SESSION_TTL=12h
SECURE_COOKIES=false

POSTGRES_DB=believeboy
POSTGRES_USER=believeboy
POSTGRES_PASSWORD=believeboy
POSTGRES_PORT=5433
MINIO_ROOT_USER=believeboy
MINIO_ROOT_PASSWORD=believeboy-dev-only
```

`package.json`:

```json
{
  "name": "believeboy-mini-program",
  "private": true,
  "packageManager": "pnpm@10.18.0",
  "engines": {
    "node": ">=24"
  },
  "scripts": {
    "check:web": "pnpm -r --if-present check"
  }
}
```

`pnpm-workspace.yaml`:

```yaml
packages:
  - "apps/*"
  - "packages/*"
```

`go.work`:

```text
go 1.27.0

use ./services/api
```

`infra/compose.yaml`:

```yaml
name: believeboy-mini-program

services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-believeboy}
      POSTGRES_USER: ${POSTGRES_USER:-believeboy}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-believeboy}
    ports:
      - "${POSTGRES_PORT:-5433}:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-believeboy} -d ${POSTGRES_DB:-believeboy}"]
      interval: 5s
      timeout: 3s
      retries: 20

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-believeboy}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-believeboy-dev-only}
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio-data:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 5s
      timeout: 3s
      retries: 20

volumes:
  postgres-data:
  minio-data:
```

`infra/Caddyfile`:

```caddyfile
{
  email {$ACME_EMAIL:ops@localhost}
}

{$ADMIN_HOST:admin.localhost} {
  handle /api/* {
    reverse_proxy api:8080
  }
  handle {
    reverse_proxy admin:3000
  }
}

{$VIEW_HOST:view.localhost} {
  handle /api/* {
    reverse_proxy api:8080
  }
  handle /enter* {
    reverse_proxy api:8080
  }
  handle {
    reverse_proxy viewer:4173
  }
}

{$CONTENT_HOST:content.localhost} {
  handle /c/* {
    reverse_proxy api:8080
  }
  respond 404
}

{$API_HOST:api.localhost} {
  reverse_proxy api:8080
}
```

`Makefile`:

```makefile
.PHONY: up down psql migrate sqlc test check

up:
	docker compose --env-file .env -f infra/compose.yaml up -d

down:
	docker compose --env-file .env -f infra/compose.yaml down

psql:
	docker compose --env-file .env -f infra/compose.yaml exec postgres psql -U believeboy -d believeboy

migrate:
	cd services/api && go run ./cmd/migrate up

sqlc:
	cd services/api && sqlc generate

test:
	cd services/api && go test ./...

check:
	cd services/api && gofmt -w . && go vet ./... && go test ./...
```

`README.md`:

```markdown
# BelieveBoy Mini Program

Private HTML proposal delivery system with a WeChat mini program, responsive H5 viewer, Next.js admin console, and Go API.

## Local setup

```bash
cp .env.example .env
make up
make migrate
cd services/api
go run ./cmd/admin create --email admin@example.com
go run ./cmd/server
```

The API listens on `http://localhost:8080` by default.

## Checks

```bash
make test
make check
```

The system design is documented in `docs/superpowers/specs/2026-10-26-believeboy-mini-program-design.md`.
```

- [ ] **Step 2: Validate configuration syntax**

Run:

```bash
docker compose --env-file .env.example -f infra/compose.yaml config >/dev/null
pnpm install --lockfile-only
```

Expected: both commands exit 0 and create no application source files.

- [ ] **Step 3: Commit**

```bash
git add .gitignore .editorconfig .env.example package.json pnpm-workspace.yaml Makefile README.md infra go.work pnpm-lock.yaml
git commit -m "chore: bootstrap project foundation"
```

---

### Task 2: Go Configuration and Health Endpoints

**Files:**
- Create: `services/api/go.mod`
- Create: `services/api/internal/config/config.go`
- Create: `services/api/internal/config/config_test.go`
- Create: `services/api/internal/httpapi/response.go`
- Create: `services/api/internal/httpapi/router.go`
- Create: `services/api/internal/httpapi/router_test.go`
- Create: `services/api/cmd/server/main.go`

**Interfaces:**
- Consumes: environment values from `.env.example`.
- Produces: `config.Load() (config.Config, error)`, `httpapi.NewRouter(httpapi.Dependencies) http.Handler`, `GET /health/live`, and `GET /health/ready`.

- [ ] **Step 1: Write failing configuration and health tests**

`services/api/internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ADMIN_SESSION_TTL", "")
	t.Setenv("ADMIN_COOKIE_NAME", "")
	t.Setenv("SECURE_COOKIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AdminSessionTTL.Hours() != 12 {
		t.Fatalf("AdminSessionTTL = %v, want 12h", cfg.AdminSessionTTL)
	}
	if cfg.AdminCookieName != "bb_admin_session" {
		t.Fatalf("AdminCookieName = %q", cfg.AdminCookieName)
	}
	if cfg.SecureCookies {
		t.Fatal("SecureCookies should default to false")
	}
}
```

`services/api/internal/httpapi/router_test.go`:

```go
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingerStub struct {
	err error
}

func (p pingerStub) Ping(context.Context) error {
	return p.err
}

func TestLiveness(t *testing.T) {
	router := NewRouter(Dependencies{Database: pingerStub{}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestReadinessFailsWhenDatabasePingFails(t *testing.T) {
	router := NewRouter(Dependencies{Database: pingerStub{err: errors.New("offline")}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd services/api && go test ./internal/config ./internal/httpapi
```

Expected: FAIL because `Config`, `Load`, and `NewRouter` do not exist.

- [ ] **Step 3: Implement configuration and HTTP foundation**

Create `go.mod` with:

```go
module github.com/believeboy/mini-bd-program/services/api

go 1.27.0
```

`config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	AdminOrigin      string
	AdminCookieName  string
	AdminSessionTTL  time.Duration
	SecureCookies    bool
}

func Load() (Config, error) {
	ttl, err := durationEnv("ADMIN_SESSION_TTL", 12*time.Hour)
	if err != nil {
		return Config{}, err
	}
	secure, err := boolEnv("SECURE_COOKIES", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:        stringEnv("API_ADDR", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AdminOrigin:     strings.TrimRight(stringEnv("ADMIN_ORIGIN", "http://admin.localhost:3000"), "/"),
		AdminCookieName: stringEnv("ADMIN_COOKIE_NAME", "bb_admin_session"),
		AdminSessionTTL: ttl,
		SecureCookies:   secure,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func stringEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}
```

`response.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
```

`router.go`:

```go
package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Pinger interface {
	Ping(context.Context) error
}

type Dependencies struct {
	Database Pinger
}

func NewRouter(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	router.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if deps.Database == nil || deps.Database.Ping(r.Context()) != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return router
}
```

`cmd/server/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/believeboy/mini-bd-program/services/api/internal/config"
	"github.com/believeboy/mini-bd-program/services/api/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(httpapi.Dependencies{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("serve api", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
cd services/api && go mod tidy && go test ./internal/config ./internal/httpapi
```

Expected: PASS for all tests.

- [ ] **Step 5: Commit**

```bash
git add go.work services/api
git commit -m "feat(api): add configuration and health endpoints"
```

---

### Task 3: Database Schema, Migrations, and Typed Queries

**Files:**
- Create: `services/api/db/migrations/00001_init.up.sql`
- Create: `services/api/db/migrations/00001_init.down.sql`
- Create: `services/api/db/queries/admin.sql`
- Create: `services/api/sqlc.yaml`
- Create: `services/api/internal/database/database.go`
- Create: `services/api/internal/database/migrate.go`
- Create: `services/api/cmd/migrate/main.go`
- Create: `services/api/internal/database/database_integration_test.go`
- Generated: `services/api/internal/store/*.go`

**Interfaces:**
- Consumes: `DATABASE_URL`.
- Produces: `database.Open(ctx, url) (*pgxpool.Pool, error)`, `database.Migrate(url, command) error`, and generated sqlc `store.Queries`.

- [ ] **Step 1: Write the failing database integration test**

`database_integration_test.go`:

```go
package database

import (
	"context"
	"os"
	"testing"
)

func TestMigrateAndPing(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if err := Migrate(url, "up"); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	pool, err := Open(context.Background(), url)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd services/api && TEST_DATABASE_URL="$DATABASE_URL" go test ./internal/database -run TestMigrateAndPing -v
```

Expected: FAIL because the database package and schema do not exist.

- [ ] **Step 3: Add the complete initial schema and admin queries**

`00001_init.up.sql`:

```sql
CREATE TABLE admin_users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    credential_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('ADMIN', 'STAFF')),
    status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'DISABLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX admin_users_email_unique ON admin_users (lower(email));

CREATE TABLE admin_sessions (
    id UUID PRIMARY KEY,
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    session_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    ip_hash BYTEA,
    user_agent_hash BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX admin_sessions_user_idx ON admin_sessions (admin_user_id);

CREATE TABLE contents (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'ARCHIVED')),
    created_by UUID NOT NULL REFERENCES admin_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE content_versions (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    version_no INTEGER NOT NULL,
    object_key TEXT NOT NULL,
    sha256 BYTEA NOT NULL,
    byte_size BIGINT NOT NULL,
    content_title TEXT,
    analysis_status TEXT NOT NULL CHECK (analysis_status IN ('PASSED', 'REJECTED')),
    analysis_notes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by UUID NOT NULL REFERENCES admin_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (content_id, version_no),
    UNIQUE (object_key)
);
CREATE INDEX content_versions_sha_idx ON content_versions (sha256);

CREATE TABLE shares (
    id UUID PRIMARY KEY,
    content_version_id UUID NOT NULL REFERENCES content_versions(id),
    type TEXT NOT NULL CHECK (type IN ('PUBLIC', 'PRIVATE')),
    label TEXT NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,
    token_ciphertext BYTEA NOT NULL,
    pin_hash TEXT NOT NULL,
    watermark_text TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    session_ttl_seconds INTEGER NOT NULL DEFAULT 1800,
    created_by UUID NOT NULL REFERENCES admin_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX shares_version_idx ON shares (content_version_id);
CREATE INDEX shares_expiry_idx ON shares (expires_at);

CREATE TABLE access_sessions (
    id UUID PRIMARY KEY,
    share_id UUID NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
    session_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    ip_hash BYTEA,
    user_agent_hash BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX access_sessions_share_idx ON access_sessions (share_id);

CREATE TABLE viewer_tickets (
    id UUID PRIMARY KEY,
    access_session_id UUID NOT NULL REFERENCES access_sessions(id) ON DELETE CASCADE,
    ticket_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX viewer_tickets_session_idx ON viewer_tickets (access_session_id);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('ADMIN', 'VIEWER', 'SYSTEM')),
    actor_id UUID,
    share_id UUID REFERENCES shares(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    request_id TEXT NOT NULL,
    ip_hash BYTEA,
    user_agent_hash BYTEA,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_share_idx ON audit_events (share_id, created_at DESC);
CREATE INDEX audit_events_created_idx ON audit_events (created_at DESC);
```

`00001_init.down.sql`:

```sql
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS viewer_tickets;
DROP TABLE IF EXISTS access_sessions;
DROP TABLE IF EXISTS shares;
DROP TABLE IF EXISTS content_versions;
DROP TABLE IF EXISTS contents;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_users;
```

`sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: postgresql
    schema: db/migrations
    queries: db/queries
    gen:
      go:
        package: store
        out: internal/store
        sql_package: pgx/v5
        emit_interface: true
        emit_empty_slices: true
        overrides:
          - db_type: uuid
            go_type:
              import: github.com/google/uuid
              type: UUID
```

`db/queries/admin.sql`:

```sql
-- name: CreateAdmin :one
INSERT INTO admin_users (id, email, credential_hash, role, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAdminByEmail :one
SELECT * FROM admin_users
WHERE lower(email) = lower($1)
LIMIT 1;

-- name: GetAdminByID :one
SELECT * FROM admin_users
WHERE id = $1
LIMIT 1;

-- name: CreateAdminSession :one
INSERT INTO admin_sessions (
    id, admin_user_id, session_hash, expires_at, last_seen_at, ip_hash, user_agent_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAdminSessionByHash :one
SELECT
    s.id AS session_id,
    s.expires_at,
    s.last_seen_at,
    s.revoked_at,
    u.id AS user_id,
    u.email,
    u.role,
    u.status
FROM admin_sessions s
JOIN admin_users u ON u.id = s.admin_user_id
WHERE s.session_hash = $1
LIMIT 1;

-- name: TouchAdminSession :exec
UPDATE admin_sessions
SET last_seen_at = $2
WHERE id = $1;

-- name: RevokeAdminSession :exec
UPDATE admin_sessions
SET revoked_at = $2
WHERE id = $1 AND revoked_at IS NULL;
```

- [ ] **Step 4: Implement database open and migration commands**

`database.go`:

```go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
```

`migrate.go`:

```go
package database

import (
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Migrate(databaseURL, command string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Run(command, db, "migrations"); err != nil {
		return fmt.Errorf("run migration %s: %w", command, err)
	}
	return nil
}
```

`cmd/migrate/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/believeboy/mini-bd-program/services/api/internal/database"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if err := database.Migrate(url, command); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Generate sqlc code and run the integration test**

Run:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
cd services/api && sqlc generate
docker compose --env-file ../../.env.example -f ../../infra/compose.yaml up -d postgres
cd ../.. && TEST_DATABASE_URL='postgres://believeboy:believeboy@localhost:5433/believeboy?sslmode=disable' make test
```

Expected: sqlc generates `internal/store`, migration tests PASS, and `go test ./...` exits 0.

- [ ] **Step 6: Commit**

```bash
git add services/api/db services/api/sqlc.yaml services/api/internal/database services/api/internal/store services/api/cmd/migrate services/api/go.mod services/api/go.sum
git commit -m "feat(api): add database schema and migrations"
```

---

### Task 4: Credential and Session Primitives

**Files:**
- Create: `services/api/internal/security/credential.go`
- Create: `services/api/internal/security/credential_test.go`
- Create: `services/api/internal/security/token.go`
- Create: `services/api/internal/security/token_test.go`

**Interfaces:**
- Consumes: no external services.
- Produces: `security.HashCredential(string) (string, error)`, `security.VerifyCredential(string, string) (bool, error)`, `security.NewToken() (raw string, hash []byte, err error)`, and `security.HashToken(string) []byte`.

- [ ] **Step 1: Write failing credential and token tests**

`credential_test.go`:

```go
package security

import "testing"

func TestCredentialRoundTrip(t *testing.T) {
	hash, err := HashCredential("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashCredential() error = %v", err)
	}
	ok, err := VerifyCredential(hash, "correct horse battery staple")
	if err != nil {
		t.Fatalf("VerifyCredential() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifyCredential() = false, want true")
	}
	ok, err = VerifyCredential(hash, "wrong")
	if err != nil {
		t.Fatalf("VerifyCredential(wrong) error = %v", err)
	}
	if ok {
		t.Fatal("VerifyCredential(wrong) = true, want false")
	}
}
```

`token_test.go`:

```go
package security

import "testing"

func TestNewTokenReturnsStableHash(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if raw == "" || len(hash) != 32 {
		t.Fatalf("raw length = %d, hash length = %d", len(raw), len(hash))
	}
	if string(hash) == raw {
		t.Fatal("hash must not equal raw token")
	}
	if string(HashToken(raw)) != string(hash) {
		t.Fatal("HashToken(raw) does not match NewToken hash")
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd services/api && go test ./internal/security
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement primitives**

`credential.go`:

```go
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

func HashCredential(credential string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}
	key := argon2.IDKey(
		[]byte(credential),
		salt,
		argonIterations,
		argonMemory,
		argonParallelism,
		argonKeyLength,
	)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyCredential(encoded, credential string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported credential hash")
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("parse argon parameters: %w", err)
	}
	_ = strconv.IntSize
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode key: %w", err)
	}
	actual := argon2.IDKey([]byte(credential), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
```

`token.go`:

```go
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func NewToken() (string, []byte, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", nil, fmt.Errorf("read token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(rawBytes)
	hash := HashToken(raw)
	return raw, hash, nil
}

func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
```

Remove the unused `strconv` import and `_ = strconv.IntSize` line if the compiler reports it; the final code must have no unused imports.

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
cd services/api && go mod tidy && go test ./internal/security -v
```

Expected: PASS for both tests.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/security services/api/go.mod services/api/go.sum
git commit -m "feat(api): add credential and token primitives"
```

---

### Task 5: Admin Authentication Service and HTTP API

**Files:**
- Create: `services/api/internal/adminauth/service.go`
- Create: `services/api/internal/adminauth/service_test.go`
- Create: `services/api/internal/httpapi/admin_auth.go`
- Create: `services/api/internal/httpapi/admin_auth_test.go`
- Modify: `services/api/internal/httpapi/router.go`
- Modify: `services/api/cmd/server/main.go`

**Interfaces:**
- Consumes: generated `store.Queries`, `security.HashCredential`, `security.VerifyCredential`, `security.NewToken`, and `security.HashToken`.
- Produces: `adminauth.Service.Login(...)`, `Authenticate(...)`, `Logout(...)`, plus `POST /api/v1/admin/login`, `GET /api/v1/admin/me`, and `POST /api/v1/admin/logout`.

- [ ] **Step 1: Write failing service tests**

Use an in-memory implementation of this interface:

```go
type Store interface {
	FindAdminByEmail(ctx context.Context, email string) (AdminRecord, error)
	CreateSession(ctx context.Context, input CreateSessionInput) error
	FindSession(ctx context.Context, hash []byte) (SessionRecord, error)
	TouchSession(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeSession(ctx context.Context, id uuid.UUID, at time.Time) error
}
```

Tests must cover:

- Login succeeds with the correct credential and returns a 43-character URL-safe token.
- Login fails with the same `ErrInvalidCredentials` for unknown email and wrong credential.
- Disabled admins and stale sessions return `ErrSessionInvalid`.
- Logout revokes the session.

Use exact table-driven tests in `service_test.go`; do not mock the security package.

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd services/api && go test ./internal/adminauth -v
```

Expected: FAIL because the package and service do not exist.

- [ ] **Step 3: Implement the service**

Expose:

```go
type Service struct {
	store        Store
	sessionTTL   time.Duration
	now          func() time.Time
}

func New(store Store, sessionTTL time.Duration) *Service
func (s *Service) Login(ctx context.Context, email, credential string, meta SessionMeta) (LoginResult, error)
func (s *Service) Authenticate(ctx context.Context, rawToken string, meta SessionMeta) (Principal, error)
func (s *Service) Logout(ctx context.Context, rawToken string) error
```

`Login` lowercases and trims email, retrieves the user, always verifies against a dummy Argon2id hash when the user is absent to reduce timing differences, rejects non-active users, creates a 32-byte token, stores only its SHA-256 hash, and returns `LoginResult{Token, ExpiresAt, Principal}`.

`Authenticate` hashes the cookie token, loads the session plus user, rejects revoked/expired/disabled states, and touches `last_seen_at`.

`Logout` is idempotent and revokes the matching session if present.

- [ ] **Step 4: Add the PostgreSQL adapter**

Create `services/api/internal/adminauth/postgres.go` with a concrete adapter over `*store.Queries`. Map pgx `pgx.ErrNoRows` to `adminauth.ErrNotFound`. Use `google/uuid.New()` for session IDs and UTC timestamps.

- [ ] **Step 5: Write failing HTTP tests**

Tests in `admin_auth_test.go` must verify:

- `POST /api/v1/admin/login` returns `204`, sets `bb_admin_session` with HttpOnly, SameSite=Lax, Path=/, MaxAge=43200, and no response body for valid credentials.
- Invalid credentials return `401` with `{"code":"INVALID_CREDENTIALS","message":"邮箱或密码错误","request_id":"..."}`.
- `GET /api/v1/admin/me` without a cookie returns `401`.
- `GET /api/v1/admin/me` with the login cookie returns the admin email and role.
- `POST /api/v1/admin/logout` clears the cookie.
- State-changing requests with a present but foreign `Origin` return `403`.

- [ ] **Step 6: Implement HTTP handlers and wiring**

`admin_auth.go` must:

- Accept only `application/json`.
- Limit request bodies to 16 KiB.
- Use `DisallowUnknownFields`.
- Set `Cache-Control: no-store`.
- Set cookie `Secure` from configuration.
- Read the session cookie for `/me` and `/logout`.
- Use a constant JSON error envelope with `code`, `message`, and `request_id`.

Update `router.go` so `Dependencies` includes `AdminAuth *adminauth.Service`, `AdminCookieName string`, `SecureCookies bool`, and `AllowedAdminOrigin string`.

Update `cmd/server/main.go` to open PostgreSQL, construct `store.New(pool)`, construct `adminauth.New(...)`, and set `Database: pool` for readiness.

- [ ] **Step 7: Run tests and verify pass**

Run:

```bash
cd services/api && go test ./internal/adminauth ./internal/httpapi -v
```

Expected: PASS for service and HTTP tests.

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/adminauth services/api/internal/httpapi services/api/cmd/server services/api/go.mod services/api/go.sum
git commit -m "feat(api): add admin authentication endpoints"
```

---

### Task 6: Administrator CLI and End-to-End Verification

**Files:**
- Create: `services/api/cmd/admin/main.go`
- Create: `services/api/cmd/admin/cli_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: migrated PostgreSQL schema and `security.HashCredential`.
- Produces: `go run ./cmd/admin create --email EMAIL --credential-stdin` and a non-zero exit for duplicate or invalid input.

- [ ] **Step 1: Write the failing CLI validation test**

Test parsing without a database:

- Missing `--email` returns an error.
- Role defaults to `ADMIN`.
- Unsupported role returns an error.
- Empty credential from stdin returns an error.

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd services/api && go test ./cmd/admin -v
```

Expected: FAIL because the CLI parser does not exist.

- [ ] **Step 3: Implement the CLI**

The command reads:

```text
go run ./cmd/admin create --email admin@example.com --credential-stdin
```

It reads one line from stdin, rejects whitespace-only credentials shorter than 12 characters, hashes the credential, inserts the administrator with `role=ADMIN` and `status=ACTIVE`, and never prints the credential or hash.

Add `--role ADMIN|STAFF`; default is `ADMIN`.

- [ ] **Step 4: Run unit tests**

Run:

```bash
cd services/api && go test ./cmd/admin -v
```

Expected: PASS.

- [ ] **Step 5: Verify the full flow against Docker**

Run:

```bash
docker compose --env-file .env.example -f infra/compose.yaml up -d postgres
export DATABASE_URL='postgres://believeboy:believeboy@localhost:5433/believeboy?sslmode=disable'
cd services/api
go run ./cmd/migrate up
printf 'dev-credential-1234\n' | go run ./cmd/admin create --email admin@example.com --credential-stdin
go run ./cmd/server
```

In a second terminal:

```bash
curl -i -c /tmp/bb-cookie.txt \
  -H 'Content-Type: application/json' \
  -H 'Origin: http://admin.localhost:3000' \
  -d '{"email":"admin@example.com","credential":"dev-credential-1234"}' \
  http://localhost:8080/api/v1/admin/login

curl -i -b /tmp/bb-cookie.txt http://localhost:8080/api/v1/admin/me
```

Expected: login returns 204 and `/me` returns the administrator email with role `ADMIN`.

- [ ] **Step 6: Run the complete suite**

Run:

```bash
make test
make check
```

Expected: all Go tests and vet checks exit 0.

- [ ] **Step 7: Commit**

```bash
git add services/api/cmd/admin README.md services/api/go.mod services/api/go.sum
git commit -m "feat(api): add administrator creation CLI"
```

## Self-Review

- Spec coverage for this milestone: repository foundation, PostgreSQL schema, admin roles, Argon2id credentials, admin sessions, login/me/logout, health checks, and first-admin CLI.
- Deferred by design: upload, content isolation, sharing, PIN sessions, viewer tickets, Next.js, H5, and mini program.
- Placeholder scan: no unresolved markers or unspecified implementation steps.
- Type consistency: session token is raw URL-safe string in HTTP/service layers and SHA-256 bytes in storage; generated store types use `google/uuid.UUID` through sqlc overrides.
