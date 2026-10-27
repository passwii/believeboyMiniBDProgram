-- name: CreateAuditEvent :exec
INSERT INTO audit_events (
    id, actor_type, actor_id, share_id, event_type, request_id,
    ip_hash, user_agent_hash, metadata_json, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAuditEvents :many
SELECT
    id, actor_type, actor_id, share_id, event_type, request_id,
    metadata_json, created_at
FROM audit_events
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;
