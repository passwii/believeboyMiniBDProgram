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
