-- name: CreateShare :one
INSERT INTO shares (
    id, content_version_id, type, label, token_hash, token_ciphertext,
    pin_hash, watermark_text, expires_at, session_ttl_seconds, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetShareByTokenHash :one
SELECT
    s.id,
    s.content_version_id,
    v.content_id,
    c.title AS content_title,
    s.type,
    s.label,
    s.token_hash,
    s.token_ciphertext,
    s.pin_hash,
    s.watermark_text,
    s.expires_at,
    s.revoked_at,
    s.session_ttl_seconds,
    s.created_by,
    s.created_at,
    s.updated_at
FROM shares s
JOIN content_versions v ON v.id = s.content_version_id
JOIN contents c ON c.id = v.content_id
WHERE s.token_hash = $1
LIMIT 1;

-- name: GetShareByID :one
SELECT
    s.id,
    s.content_version_id,
    v.content_id,
    c.title AS content_title,
    s.type,
    s.label,
    s.token_hash,
    s.token_ciphertext,
    s.pin_hash,
    s.watermark_text,
    s.expires_at,
    s.revoked_at,
    s.session_ttl_seconds,
    s.created_by,
    s.created_at,
    s.updated_at
FROM shares s
JOIN content_versions v ON v.id = s.content_version_id
JOIN contents c ON c.id = v.content_id
WHERE s.id = $1
LIMIT 1;

-- name: ListShares :many
SELECT
    s.id,
    s.content_version_id,
    v.content_id,
    c.title AS content_title,
    s.type,
    s.label,
    s.token_hash,
    s.token_ciphertext,
    s.pin_hash,
    s.watermark_text,
    s.expires_at,
    s.revoked_at,
    s.session_ttl_seconds,
    s.created_by,
    s.created_at,
    s.updated_at
FROM shares s
JOIN content_versions v ON v.id = s.content_version_id
JOIN contents c ON c.id = v.content_id
ORDER BY s.created_at DESC
LIMIT $1 OFFSET $2;

-- name: RevokeShare :exec
UPDATE shares
SET revoked_at = $2, updated_at = $2
WHERE id = $1 AND revoked_at IS NULL;

-- name: ResetSharePIN :exec
UPDATE shares
SET pin_hash = $2, updated_at = $3
WHERE id = $1;

-- name: CreateAccessSession :one
INSERT INTO access_sessions (
    id, share_id, session_hash, expires_at, last_seen_at, ip_hash, user_agent_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAccessSessionByHash :one
SELECT
    a.id AS session_id,
    a.share_id,
    a.expires_at AS session_expires_at,
    a.revoked_at AS session_revoked_at,
    a.last_seen_at,
    s.expires_at AS share_expires_at,
    s.revoked_at AS share_revoked_at,
    s.type,
    s.label,
    s.watermark_text,
    s.session_ttl_seconds,
    s.content_version_id,
    v.object_key,
    c.title AS content_title
FROM access_sessions a
JOIN shares s ON s.id = a.share_id
JOIN content_versions v ON v.id = s.content_version_id
JOIN contents c ON c.id = v.content_id
WHERE a.session_hash = $1
LIMIT 1;

-- name: GetAccessSessionByID :one
SELECT
    a.id AS session_id,
    a.share_id,
    a.expires_at AS session_expires_at,
    a.revoked_at AS session_revoked_at,
    a.last_seen_at,
    s.expires_at AS share_expires_at,
    s.revoked_at AS share_revoked_at,
    s.type,
    s.label,
    s.watermark_text,
    s.session_ttl_seconds,
    s.content_version_id,
    v.object_key,
    c.title AS content_title
FROM access_sessions a
JOIN shares s ON s.id = a.share_id
JOIN content_versions v ON v.id = s.content_version_id
JOIN contents c ON c.id = v.content_id
WHERE a.id = $1
LIMIT 1;

-- name: RevokeAccessSessionsByShare :exec
UPDATE access_sessions
SET revoked_at = $2
WHERE share_id = $1 AND revoked_at IS NULL;

-- name: CreateViewerTicket :one
INSERT INTO viewer_tickets (
    id, access_session_id, ticket_hash, session_ciphertext, expires_at
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ConsumeViewerTicket :one
WITH consumed AS (
    UPDATE viewer_tickets
    SET consumed_at = $2
    WHERE ticket_hash = $1
      AND consumed_at IS NULL
      AND expires_at > $2
    RETURNING access_session_id, session_ciphertext
)
SELECT
    c.access_session_id,
    c.session_ciphertext,
    a.share_id,
    s.token_ciphertext
FROM consumed c
JOIN access_sessions a ON a.id = c.access_session_id
JOIN shares s ON s.id = a.share_id;
