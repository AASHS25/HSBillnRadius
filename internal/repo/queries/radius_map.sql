-- Provisioning writes for RADIUS. Sync uses delete-then-insert for idempotency.

-- name: DeleteRadCheckByUser :exec
DELETE FROM radcheck WHERE tenant_id = $1 AND username = $2;

-- name: InsertRadCheck :exec
INSERT INTO radcheck (tenant_id, username, attribute, op, value)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteRadReplyByUser :exec
DELETE FROM radreply WHERE tenant_id = $1 AND username = $2;

-- name: InsertRadReply :exec
INSERT INTO radreply (tenant_id, username, attribute, op, value)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteRadUserGroupByUser :exec
DELETE FROM radusergroup WHERE tenant_id = $1 AND username = $2;

-- name: InsertRadUserGroup :exec
INSERT INTO radusergroup (tenant_id, username, groupname, priority)
VALUES ($1, $2, $3, $4);

-- name: SetRadUserGroup :exec
UPDATE radusergroup SET groupname = $3, priority = $4
WHERE tenant_id = $1 AND username = $2;

-- name: DeleteRadGroupReplyByGroup :exec
DELETE FROM radgroupreply WHERE tenant_id = $1 AND groupname = $2;

-- name: InsertRadGroupReply :exec
INSERT INTO radgroupreply (tenant_id, groupname, attribute, op, value)
VALUES ($1, $2, $3, $4, $5);

-- name: ListRadUserGroups :many
SELECT username, groupname, priority FROM radusergroup
WHERE tenant_id = $1 AND username = $2 ORDER BY priority;
