-- +goose Up
ALTER TABLE viewer_tickets
ADD COLUMN session_ciphertext BYTEA NOT NULL;

-- +goose Down
ALTER TABLE viewer_tickets
DROP COLUMN session_ciphertext;
