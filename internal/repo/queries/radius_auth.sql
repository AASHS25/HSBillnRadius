-- Read-side queries used by the radius-service auth handler.

-- name: GetNasByIP :one
SELECT id, tenant_id, nasname, shortname, secret FROM nas WHERE nasname = $1 LIMIT 1;

-- name: ListRadCheckUser :many
SELECT attribute, op, value FROM radcheck WHERE tenant_id = $1 AND username = $2;

-- name: ListRadReplyUser :many
SELECT attribute, op, value FROM radreply WHERE tenant_id = $1 AND username = $2;

-- name: ListRadGroupReplyForGroup :many
SELECT attribute, op, value FROM radgroupreply WHERE tenant_id = $1 AND groupname = $2;

-- name: InsertRadPostAuth :exec
INSERT INTO radpostauth (tenant_id, username, pass, reply, nasipaddress)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateNas :one
INSERT INTO nas (tenant_id, nasname, shortname, type, ports, secret, server, community, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListNasByTenant :many
SELECT * FROM nas WHERE tenant_id = $1 ORDER BY nasname;
