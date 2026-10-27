-- +goose Up
CREATE INDEX admin_sessions_revoked_idx
    ON admin_sessions (revoked_at)
    WHERE revoked_at IS NOT NULL;
CREATE INDEX access_sessions_revoked_idx
    ON access_sessions (revoked_at)
    WHERE revoked_at IS NOT NULL;
CREATE INDEX viewer_tickets_consumed_idx
    ON viewer_tickets (consumed_at)
    WHERE consumed_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS viewer_tickets_consumed_idx;
DROP INDEX IF EXISTS access_sessions_revoked_idx;
DROP INDEX IF EXISTS admin_sessions_revoked_idx;
