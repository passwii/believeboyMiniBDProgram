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
    v.sha256,
    v.byte_size,
    v.analysis_notes
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
