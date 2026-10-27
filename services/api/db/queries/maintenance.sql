-- name: DeleteOldAdminSessions :execrows
DELETE FROM admin_sessions
WHERE expires_at < $1 OR revoked_at < $1;

-- name: DeleteOldAccessSessions :execrows
DELETE FROM access_sessions
WHERE expires_at < $1 OR revoked_at < $1;

-- name: DeleteOldViewerTickets :execrows
DELETE FROM viewer_tickets
WHERE expires_at < $1 OR (consumed_at IS NOT NULL AND consumed_at < $2);

-- name: DeleteOldAuditEvents :execrows
DELETE FROM audit_events
WHERE created_at < $1;
