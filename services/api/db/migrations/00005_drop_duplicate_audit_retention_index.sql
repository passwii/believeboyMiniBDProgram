-- +goose Up
DROP INDEX IF EXISTS audit_events_retention_idx;

-- +goose Down
CREATE INDEX audit_events_retention_idx ON audit_events (created_at);
