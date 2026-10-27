-- +goose Up
CREATE INDEX admin_sessions_expiry_idx ON admin_sessions (expires_at);
CREATE INDEX access_sessions_expiry_idx ON access_sessions (expires_at);
CREATE INDEX viewer_tickets_expiry_idx ON viewer_tickets (expires_at);
CREATE INDEX audit_events_retention_idx ON audit_events (created_at);

-- +goose Down
DROP INDEX IF EXISTS audit_events_retention_idx;
DROP INDEX IF EXISTS viewer_tickets_expiry_idx;
DROP INDEX IF EXISTS access_sessions_expiry_idx;
DROP INDEX IF EXISTS admin_sessions_expiry_idx;
