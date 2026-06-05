-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY code;

-- name: ListPermissionIDsByCodes :many
SELECT id FROM permissions WHERE code = ANY($1::text[]);
