-- +goose Up
-- Accounting table, partitioned monthly by start time. A DEFAULT partition
-- guarantees inserts always land somewhere; the scheduler pre-creates monthly
-- partitions and drops/archives old ones (housekeeping, M5).
-- Partitioned unique/PK constraints must include the partition key.
CREATE TABLE radacct (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY,
    tenant_id          BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    acctsessionid      TEXT        NOT NULL,
    acctuniqueid       TEXT        NOT NULL,
    username           TEXT        NOT NULL DEFAULT '',
    nasipaddress       TEXT        NOT NULL DEFAULT '',
    nasportid          TEXT        NOT NULL DEFAULT '',
    acctstarttime      TIMESTAMPTZ NOT NULL DEFAULT now(),
    acctupdatetime     TIMESTAMPTZ NULL,
    acctstoptime       TIMESTAMPTZ NULL,
    acctsessiontime    BIGINT      NOT NULL DEFAULT 0,
    acctinputoctets    BIGINT      NOT NULL DEFAULT 0,
    acctoutputoctets   BIGINT      NOT NULL DEFAULT 0,
    calledstationid    TEXT        NOT NULL DEFAULT '',
    callingstationid   TEXT        NOT NULL DEFAULT '',
    framedipaddress    TEXT        NOT NULL DEFAULT '',
    acctterminatecause TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (id, acctstarttime)
) PARTITION BY RANGE (acctstarttime);

CREATE UNIQUE INDEX radacct_uniqueid_key ON radacct (acctuniqueid, acctstarttime);
CREATE INDEX radacct_uniqueid_idx ON radacct (acctuniqueid);
CREATE INDEX radacct_tenant_user_idx ON radacct (tenant_id, username, acctstarttime DESC);
CREATE INDEX radacct_active_idx ON radacct (tenant_id, username) WHERE acctstoptime IS NULL;

CREATE TABLE radacct_default PARTITION OF radacct DEFAULT;

-- +goose Down
DROP TABLE IF EXISTS radacct;
