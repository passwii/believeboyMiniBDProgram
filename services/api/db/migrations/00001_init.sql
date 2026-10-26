-- +goose Up
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

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS viewer_tickets;
DROP TABLE IF EXISTS access_sessions;
DROP TABLE IF EXISTS shares;
DROP TABLE IF EXISTS content_versions;
DROP TABLE IF EXISTS contents;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_users;
