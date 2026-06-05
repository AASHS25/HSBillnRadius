-- +goose Up
-- +goose StatementBegin
-- pgcrypto: gen_random_uuid() and digest helpers used by later domain tables.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- citext: case-insensitive text for per-tenant unique emails/usernames.
CREATE EXTENSION IF NOT EXISTS citext;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP EXTENSION IF EXISTS citext;
DROP EXTENSION IF EXISTS pgcrypto;
-- +goose StatementEnd
