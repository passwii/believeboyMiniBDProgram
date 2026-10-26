# Share and Viewer Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the complete backend share/access loop: admins create public or private shares, viewers unlock with a 4-digit PIN, sessions are rate-limited and revocable, the mini program exchanges a one-time ticket for a viewer session, and signed HTML content is served with strict isolation headers.

**Architecture:** A new `share` package owns token encryption, PIN hashing, share persistence, access sessions, viewer tickets, rate limiting, and content claims. Admin routes use the existing administrator session middleware. Public routes issue viewer cookies or mini-program tickets, while the content route verifies a signed claim plus the current share/session state before reading the HTML object.

**Tech Stack:** Go 1.27, Chi, pgx/v5, sqlc, PostgreSQL 17, AES-256-GCM, Argon2id, HMAC-SHA256.

## Global Constraints

- PIN is exactly four ASCII digits.
- Share types are `PUBLIC` and `PRIVATE`; customer accounts are never created.
- Every share has a required future expiration time and defaults to a 30-minute viewer session.
- Viewer tickets are one-time and expire after 60 seconds.
- Signed content URLs expire after 10 minutes.
- PIN failures are limited per `share + IP` and per share globally.
- Share token plaintext is stored only as AES-GCM ciphertext; lookup uses SHA-256.
- PIN plaintext is never stored and cannot be retrieved.
- Revoking a share invalidates all its access sessions and future content URL issuance.
- HTML content responses set `Cache-Control: no-store`, `nosniff`, `no-referrer`, CSP, and `frame-ancestors VIEW_ORIGIN`.
- API responses never expose encrypted blobs, token hashes, PIN hashes, or object keys.

---

### Task 1: Extend Configuration

**Files:**
- Modify: `.env.example`
- Modify: `services/api/internal/config/config.go`
- Modify: `services/api/internal/config/config_test.go`

**Interfaces:**
- Produces: `ViewOrigin`, `ContentOrigin`, `ViewerCookieName`, `ViewerSessionTTL`, `ViewerTicketTTL`, `ContentURLTTL`, `TokenEncryptionKey`, `ContentSigningKey`, `IPHashKey`.

- [ ] Add failing tests for defaults, base64 key decoding, and 32-byte key validation.
- [ ] Run `go test ./internal/config -v` and confirm failure.
- [ ] Add the new environment variables and implement strict validation.
- [ ] Run tests and commit `feat(api): configure viewer access`.

---

### Task 2: Add Encryption, Hashing, and Signed Claims

**Files:**
- Create: `services/api/internal/security/cipher.go`
- Create: `services/api/internal/security/cipher_test.go`
- Create: `services/api/internal/security/hmac.go`
- Create: `services/api/internal/security/hmac_test.go`
- Create: `services/api/internal/share/claims.go`
- Create: `services/api/internal/share/claims_test.go`

**Interfaces:**

```go
func NewCipher(key []byte) (*Cipher, error)
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error)
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error)

func HMAC(key []byte, value string) []byte

type ContentClaims struct {
    ShareID   uuid.UUID `json:"share_id"`
    VersionID uuid.UUID `json:"version_id"`
    SessionID uuid.UUID `json:"session_id"`
    ExpiresAt int64     `json:"exp"`
}

func SignClaims(claims ContentClaims, key []byte) (string, error)
func VerifyClaims(token string, key []byte) (ContentClaims, error)
```

- [ ] Write tests for cipher round-trip, tamper rejection, stable HMAC, claim round-trip, expired claims, and tampered signatures.
- [ ] Run tests and confirm failure.
- [ ] Implement AES-256-GCM, HMAC-SHA256, and signed base64url claims.
- [ ] Run tests and commit `feat(security): add viewer crypto primitives`.

---

### Task 3: Persist Share and Session Records

**Files:**
- Create: `services/api/db/queries/share.sql`
- Regenerate: `services/api/internal/store/*.go`
- Create: `services/api/internal/share/repository.go`
- Create: `services/api/internal/share/postgres.go`
- Create: `services/api/internal/share/postgres_integration_test.go`

**Interfaces:**

```go
type Repository interface {
    CreateShare(context.Context, PersistShareInput) (ShareRecord, error)
    ListShares(context.Context, int32, int32) ([]ShareRecord, error)
    GetShareByTokenHash(context.Context, []byte) (ShareRecord, error)
    GetShareByID(context.Context, uuid.UUID) (ShareRecord, error)
    RevokeShare(context.Context, uuid.UUID, time.Time) error
    ResetPIN(context.Context, uuid.UUID, string, time.Time) error
    CreateSession(context.Context, SessionInput) (SessionRecord, error)
    GetSessionByHash(context.Context, []byte) (SessionRecord, error)
    RevokeSessionsByShare(context.Context, uuid.UUID, time.Time) error
    CreateTicket(context.Context, TicketInput) error
}
```

- [ ] Add SQL queries with joins to content title and latest version.
- [ ] Run sqlc generation.
- [ ] Write repository integration tests against PostgreSQL.
- [ ] Implement the adapter and add a transaction that consumes tickets exactly once.
- [ ] Run tests and commit `feat(share): persist shares and sessions`.

---

### Task 4: Implement Admin Share APIs

**Files:**
- Create: `services/api/internal/share/service.go`
- Create: `services/api/internal/share/service_test.go`
- Create: `services/api/internal/httpapi/admin_shares.go`
- Create: `services/api/internal/httpapi/admin_shares_test.go`
- Modify: `services/api/internal/httpapi/router.go`
- Modify: `services/api/cmd/server/main.go`

**Interfaces:**

```text
GET  /api/v1/admin/shares
POST /api/v1/admin/shares
POST /api/v1/admin/shares/{id}/revoke
POST /api/v1/admin/shares/{id}/reset-pin
```

- [ ] Write service tests for public/private creation, invalid PIN, past expiration, content lookup, revoke, and PIN reset.
- [ ] Write HTTP tests for authentication, 201 creation, link generation, and revoke/reset behavior.
- [ ] Implement token generation, AES-GCM storage, Argon2id PIN hashing, and link construction.
- [ ] Wire routes and dependencies.
- [ ] Run tests and commit `feat(api): add admin share management`.

---

### Task 5: Add PIN Rate Limiting and Viewer Sessions

**Files:**
- Create: `services/api/internal/share/limiter.go`
- Create: `services/api/internal/share/limiter_test.go`
- Create: `services/api/internal/share/access.go`
- Create: `services/api/internal/share/access_test.go`
- Create: `services/api/internal/httpapi/public_shares.go`
- Create: `services/api/internal/httpapi/public_shares_test.go`

**Interfaces:**

```text
GET  /api/v1/shares/{token}/status
POST /api/v1/shares/{token}/unlock
POST /api/v1/miniapp/shares/{token}/unlock
GET  /api/v1/viewer/session
POST /api/v1/viewer/heartbeat
POST /api/v1/viewer/content-url
GET  /enter?ticket=...
```

- [ ] Write limiter tests for per-IP lockout, global cooldown, success reset, and `Retry-After`.
- [ ] Write access tests for status, active/expired/revoked shares, PIN failure, session expiry, one-time tickets, and revocation.
- [ ] Implement in-memory limiter, session issuance, ticket consumption, H5 cookie, and mini-program ticket response.
- [ ] Run tests and commit `feat(api): add viewer unlock flow`.

---

### Task 6: Serve Isolated HTML Content

**Files:**
- Create: `services/api/internal/httpapi/content.go`
- Create: `services/api/internal/httpapi/content_test.go`
- Modify: `services/api/internal/httpapi/router.go`
- Modify: `services/api/cmd/server/main.go`
- Modify: `README.md`

**Interfaces:**

```text
GET /c/{signedToken}
```

- [ ] Write tests for valid claims, expired claims, revoked sessions, missing objects, and all security headers.
- [ ] Implement content validation, object read, CSP, frame policy, and no-store response.
- [ ] Wire the content store into HTTP dependencies.
- [ ] Run `make test` and `make check`.
- [ ] Commit `feat(api): serve isolated HTML content`.

## Self-Review

- Spec coverage: share creation, PIN expiration, public/private links, revoke/reset, limiter, viewer sessions, mini-program ticket, signed content URL, isolated content response.
- Deferred: audit event recording and frontend implementations.
- No unresolved markers or unspecified implementation steps.
