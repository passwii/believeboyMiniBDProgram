# Content Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add authenticated single-file HTML upload, strict self-contained validation, S3-compatible object storage, immutable content versions, and admin content list/detail endpoints.

**Architecture:** The Go API validates the uploaded file before writing it to object storage, then creates content metadata and version rows in a PostgreSQL transaction. MinIO provides the local S3 implementation. Unit tests use an in-memory storage implementation; the service and database integration tests use real PostgreSQL.

**Tech Stack:** Go 1.27, Chi, pgx/v5, sqlc, PostgreSQL 17, AWS SDK for Go v2 S3 client, MinIO.

## Global Constraints

- Only one `.html` file per upload; maximum size is 20 MiB.
- The HTML must be self-contained: external `http://`, `https://`, and protocol-relative resources are rejected.
- Inline scripts, inline styles, `data:` and `blob:` resources remain allowed.
- A missing `<title>` falls back to the original filename and emits a warning.
- A missing viewport declaration is allowed and emits a warning.
- Content versions are immutable.
- Failed uploads must not leave orphaned database rows or object-storage objects.
- All content admin APIs require an active administrator session.
- Object storage credentials and signing keys never reach the browser.
- Upload responses never expose object keys.

---

### Task 1: Extend Configuration for Uploads and Object Storage

**Files:**
- Modify: `.env.example`
- Modify: `services/api/internal/config/config.go`
- Modify: `services/api/internal/config/config_test.go`

**Interfaces:**
- Consumes: environment variables.
- Produces: `Config.MaxUploadBytes`, `Config.S3Endpoint`, `Config.S3Region`, `Config.S3Bucket`, `Config.S3AccessKey`, `Config.S3SecretKey`, `Config.S3UsePathStyle`.

- [ ] **Step 1: Add failing configuration tests**

Add tests for:

- `MAX_UPLOAD_BYTES` defaults to `20971520`.
- `MAX_UPLOAD_BYTES` rejects values above `20971520`.
- Missing `S3_BUCKET`, `S3_ACCESS_KEY`, or `S3_SECRET_KEY` returns an error.
- `S3_USE_PATH_STYLE` defaults to `true`.

Use `t.Setenv` for every variable so the test is hermetic.

- [ ] **Step 2: Run tests and verify failure**

```bash
cd services/api && go test ./internal/config -v
```

Expected: FAIL because the new fields and validation do not exist.

- [ ] **Step 3: Add environment values**

Append to `.env.example`:

```dotenv
MAX_UPLOAD_BYTES=20971520
S3_ENDPOINT=http://localhost:9000
S3_REGION=us-east-1
S3_BUCKET=content
S3_ACCESS_KEY=believeboy
S3_SECRET_KEY=believeboy-dev-only
S3_USE_PATH_STYLE=true
```

- [ ] **Step 4: Implement configuration**

Add `int64Env` and required-string validation. Reject a configured upload limit above 20 MiB. Keep the other existing defaults unchanged.

- [ ] **Step 5: Run tests and verify pass**

```bash
cd services/api && go test ./internal/config -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add .env.example services/api/internal/config
git commit -m "feat(api): configure content storage"
```

---

### Task 2: Analyze and Validate Self-Contained HTML

**Files:**
- Create: `services/api/internal/content/analyze.go`
- Create: `services/api/internal/content/analyze_test.go`

**Interfaces:**
- Consumes: raw HTML bytes and original filename.
- Produces:

```go
type Analysis struct {
    Title              string
    ViewportPresent    bool
    ScriptCount        int
    ExternalResources  []string
    Warnings           []string
}

func Analyze(data []byte, filename string) (Analysis, error)
```

- [ ] **Step 1: Write failing tests**

Cover:

1. A valid self-contained document extracts its `<title>`.
2. Missing title falls back to the filename stem and adds a warning.
3. Missing viewport adds a warning.
4. Inline scripts are counted.
5. External `<script src="https://...">`, `<img src="//cdn...">`, and CSS `url(https://...)` are collected and return `ErrExternalResources`.
6. `data:image/png;base64,...` is not considered external.
7. Invalid HTML input returns an error instead of panicking.

- [ ] **Step 2: Run tests and verify failure**

```bash
cd services/api && go test ./internal/content -v
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the analyzer**

Use `golang.org/x/net/html`. Walk nodes and collect:

- first non-empty `<title>` text
- `<meta name="viewport">`
- `<script>` count
- `src`, `href`, `poster`, and `srcset` attributes
- CSS `url(...)` values in `<style>` nodes

Use `net/url` to classify protocol-relative and absolute HTTP(S) references. Deduplicate references while preserving document order.

Return `ErrExternalResources` when at least one is found. Do not reject `mailto:`, `tel:`, fragment links, `data:`, or `blob:`.

- [ ] **Step 4: Run tests and verify pass**

```bash
cd services/api && go mod tidy && go test ./internal/content -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/content services/api/go.mod services/api/go.sum
git commit -m "feat(content): validate self-contained HTML"
```

---

### Task 3: Add Object Storage Interface and S3 Implementation

**Files:**
- Create: `services/api/internal/storage/storage.go`
- Create: `services/api/internal/storage/memory.go`
- Create: `services/api/internal/storage/memory_test.go`
- Create: `services/api/internal/storage/s3.go`
- Modify: `services/api/internal/config/config.go`

**Interfaces:**
- Consumes: `config.Config` S3 fields.
- Produces:

```go
type Store interface {
    Put(ctx context.Context, key string, contentType string, data []byte) error
    Get(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}

func NewS3Store(cfg config.Config) (*S3Store, error)
func NewMemoryStore() *MemoryStore
```

- [ ] **Step 1: Write failing memory-store tests**

Test Put/Get/Delete, missing-key `ErrNotFound`, defensive copies, and concurrent access under `go test -race`.

- [ ] **Step 2: Run tests and verify failure**

```bash
cd services/api && go test -race ./internal/storage -v
```

Expected: FAIL because the storage package does not exist.

- [ ] **Step 3: Implement the memory store**

Use a mutex-protected map. Copy input and output byte slices. Return `ErrNotFound` for missing keys.

- [ ] **Step 4: Implement the S3 store**

Use `github.com/aws/aws-sdk-go-v2` and its S3 client. Configure:

- custom endpoint resolver when `S3_ENDPOINT` is set
- path-style addressing from `S3_USE_PATH_STYLE`
- static credentials
- `ContentType`
- `Bucket`
- `Key`

`Get` returns `ErrNotFound` for `NoSuchKey`.

- [ ] **Step 5: Run tests and verify pass**

```bash
cd services/api && go mod tidy && go test -race ./internal/storage -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/storage services/api/internal/config services/api/go.mod services/api/go.sum
git commit -m "feat(storage): add S3-compatible content storage"
```

---

### Task 4: Persist Content and Versions

**Files:**
- Modify: `services/api/db/queries/admin.sql`
- Create: `services/api/db/queries/content.sql`
- Regenerate: `services/api/internal/store/*.go`
- Create: `services/api/internal/content/service.go`
- Create: `services/api/internal/content/service_test.go`

**Interfaces:**
- Consumes: `storage.Store`.
- Produces:

```go
type CreateInput struct {
    Filename string
    Data     []byte
    AdminID  uuid.UUID
}

type Content struct {
    ID          uuid.UUID
    Title       string
    Status      string
    VersionID   uuid.UUID
    VersionNo   int32
    SHA256      []byte
    ByteSize    int64
    Analysis    Analysis
    CreatedAt   time.Time
}

type Service interface {
    Create(ctx context.Context, input CreateInput) (Content, error)
    List(ctx context.Context, limit, offset int32) ([]Content, error)
}
```

- [ ] **Step 1: Write failing service tests**

Cover:

1. Rejects files over 20 MiB before parsing.
2. Rejects non-HTML filenames.
3. Rejects external resources.
4. Stores the object before committing database rows.
5. Deletes the stored object if the database transaction fails.
6. Returns analysis warnings and the original title.
7. Never returns the object key in the public `Content` type.

- [ ] **Step 2: Run tests and verify failure**

```bash
cd services/api && go test ./internal/content -v
```

Expected: FAIL because the service does not exist.

- [ ] **Step 3: Add queries**

Create:

```sql
-- name: CreateContent :one
INSERT INTO contents (id, title, status, created_by)
VALUES ($1, $2, 'ACTIVE', $3)
RETURNING *;

-- name: CreateContentVersion :one
INSERT INTO content_versions (
    id, content_id, version_no, object_key, sha256, byte_size,
    content_title, analysis_status, analysis_notes, created_by
)
VALUES ($1, $2, 1, $3, $4, $5, $6, 'PASSED', $7, $8)
RETURNING *;

-- name: ListContents :many
SELECT
    c.id,
    c.title,
    c.status,
    c.created_at,
    v.id AS version_id,
    v.version_no,
    v.byte_size
FROM contents c
JOIN LATERAL (
    SELECT * FROM content_versions
    WHERE content_id = c.id
    ORDER BY version_no DESC
    LIMIT 1
) v ON true
WHERE c.status = 'ACTIVE'
ORDER BY c.created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetContent :one
SELECT
    c.id,
    c.title,
    c.status,
    c.created_at,
    v.id AS version_id,
    v.version_no,
    v.object_key,
    v.sha256,
    v.byte_size,
    v.content_title,
    v.analysis_notes
FROM contents c
JOIN LATERAL (
    SELECT * FROM content_versions
    WHERE content_id = c.id
    ORDER BY version_no DESC
    LIMIT 1
) v ON true
WHERE c.id = $1;
```

- [ ] **Step 4: Generate typed queries**

```bash
cd services/api && $(go env GOPATH)/bin/sqlc generate
```

Expected: generated code compiles.

- [ ] **Step 5: Implement the service**

Generate `contentID`, `versionID`, and an object key such as
`content/<contentID>/<versionID>.html`.

On database failure, call `storage.Delete`. Use a database transaction through `store.Queries.WithTx`.

- [ ] **Step 6: Run tests and verify pass**

```bash
cd services/api && go test ./internal/content -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add services/api/db/queries services/api/internal/store services/api/internal/content
git commit -m "feat(content): persist immutable content versions"
```

---

### Task 5: Add Authenticated Content HTTP APIs

**Files:**
- Create: `services/api/internal/httpapi/admin_middleware.go`
- Create: `services/api/internal/httpapi/admin_contents.go`
- Create: `services/api/internal/httpapi/admin_contents_test.go`
- Modify: `services/api/internal/httpapi/router.go`
- Modify: `services/api/cmd/server/main.go`

**Interfaces:**
- Consumes: `content.Service`, `adminauth.Service`.
- Produces:

```text
GET  /api/v1/admin/contents
POST /api/v1/admin/contents
GET  /api/v1/admin/contents/{id}
```

- [ ] **Step 1: Write failing HTTP tests**

Cover:

- Unauthenticated upload and list requests return `401`.
- A valid session cookie permits upload.
- Multipart requests without exactly one `file` part return `400`.
- Files over 20 MiB return `413`.
- External resources return `422`.
- Successful upload returns `201` with title, version, SHA-256 hex, size, and warnings.
- List returns newest content first.
- Detail returns metadata without `object_key`.

- [ ] **Step 2: Run tests and verify failure**

```bash
cd services/api && go test ./internal/httpapi -v
```

Expected: FAIL because the routes and handlers do not exist.

- [ ] **Step 3: Implement admin session middleware**

Read `AdminCookieName`, authenticate, place `adminauth.Principal` in request context, and return `401` on missing/invalid sessions.

- [ ] **Step 4: Implement handlers**

Use `http.MaxBytesReader` with `MaxUploadBytes + 1 MiB` multipart overhead. Reject more than one file part. Pass bytes to the content service. Use existing error envelopes.

- [ ] **Step 5: Wire dependencies**

Add `Content *content.Service` and `MaxUploadBytes int64` to HTTP dependencies. Construct S3 storage and content service in `cmd/server`.

- [ ] **Step 6: Run tests and verify pass**

```bash
cd services/api && go test ./internal/httpapi ./internal/content -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/httpapi services/api/cmd/server
git commit -m "feat(api): add authenticated content upload endpoints"
```

---

### Task 6: Verify MinIO and PostgreSQL End-to-End

**Files:**
- Modify: `README.md`
- Create: `scripts/dev/seed-minio.sh`

**Interfaces:**
- Consumes: Docker Compose services, administrator session, valid self-contained HTML.
- Produces: a stored HTML object and a content version visible through the API.

- [ ] **Step 1: Ensure the MinIO bucket exists**

Create `scripts/dev/seed-minio.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

docker compose --env-file .env -f infra/compose.yaml exec -T minio sh -lc '
  mc alias set local http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
  mc mb --ignore-existing local/content >/dev/null
'
```

- [ ] **Step 2: Run local dependencies**

```bash
docker compose --env-file .env.example -f infra/compose.yaml up -d postgres minio
chmod +x scripts/dev/seed-minio.sh
./scripts/dev/seed-minio.sh
```

Expected: PostgreSQL and MinIO are healthy, bucket `content` exists.

- [ ] **Step 3: Run migration and server**

```bash
set -a; source .env.example; set +a
cd services/api
go run ./cmd/migrate up
go run ./cmd/server
```

Expected: API starts and `/health/ready` returns `200`.

- [ ] **Step 4: Upload sample HTML**

Create a temporary self-contained document with `<title>`, viewport, inline CSS, and inline JavaScript. Log in with the development administrator, then:

```bash
curl -i -b /tmp/bb-cookie.txt \
  -F 'file=@/tmp/content-upload-sample.html;type=text/html' \
  http://localhost:8080/api/v1/admin/contents
```

Expected: `201` with `title`, `version_no: 1`, a 64-character SHA-256 hex string, byte size, and warnings.

- [ ] **Step 5: Verify object and database**

```bash
docker compose --env-file .env.example -f infra/compose.yaml exec -T minio sh -lc \
  'mc ls --recursive local/content'
docker compose --env-file .env.example -f infra/compose.yaml exec -T postgres \
  psql -U believeboy -d believeboy -c \
  'SELECT c.title, v.version_no, v.byte_size FROM contents c JOIN content_versions v ON v.content_id = c.id;'
```

Expected: one object and one matching content version.

- [ ] **Step 6: Run the full suite**

```bash
set -a; source .env.example; set +a
TEST_DATABASE_URL="$DATABASE_URL" make test
make check
```

Expected: all tests and vet checks pass.

- [ ] **Step 7: Commit**

```bash
git add README.md scripts/dev/seed-minio.sh
git commit -m "chore: document content storage setup"
```

## Self-Review

- Spec coverage for this milestone: 20 MiB limit, single self-contained HTML, title and viewport analysis, external-resource rejection, object storage, immutable version rows, authenticated upload/list/detail APIs.
- Deferred by design: content preview, share creation, viewer sessions, content signing, H5, Next.js, and mini program.
- Placeholder scan: no unresolved markers or unspecified implementation steps.
- Type consistency: content IDs and version IDs use `google/uuid.UUID`; SHA-256 values remain raw 32-byte arrays internally and are rendered as lowercase hex only at the HTTP boundary.
