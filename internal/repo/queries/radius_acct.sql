-- name: InsertAcctStart :exec
INSERT INTO radacct (
    tenant_id, acctsessionid, acctuniqueid, username, nasipaddress, nasportid,
    acctstarttime, acctupdatetime, framedipaddress, callingstationid, calledstationid
) VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, $9, $10)
ON CONFLICT (acctuniqueid, acctstarttime) DO NOTHING;

-- name: UpdateAcctInterim :exec
UPDATE radacct SET
    acctsessiontime = $2, acctinputoctets = $3, acctoutputoctets = $4,
    acctupdatetime = now(), framedipaddress = COALESCE(NULLIF($5::text, ''), framedipaddress)
WHERE acctuniqueid = $1 AND acctstoptime IS NULL;

-- name: UpdateAcctStop :exec
UPDATE radacct SET
    acctstoptime = now(), acctsessiontime = $2, acctinputoctets = $3,
    acctoutputoctets = $4, acctterminatecause = $5, acctupdatetime = now()
WHERE acctuniqueid = $1 AND acctstoptime IS NULL;

-- name: ListActiveSessions :many
SELECT acctsessionid, nasipaddress, framedipaddress, callingstationid
FROM radacct
WHERE tenant_id = $1 AND username = $2 AND acctstoptime IS NULL
ORDER BY acctstarttime DESC;
