-- +goose Up
-- Portal password lets a customer log into the client area (separate from
-- their PPPoE credential).
ALTER TABLE customers ADD COLUMN portal_password_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE customers DROP COLUMN IF EXISTS portal_password_hash;
