-- +goose Up
CREATE TABLE acs_devices (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    customer_id   BIGINT      NULL REFERENCES customers (id) ON DELETE SET NULL,
    device_id     TEXT        NOT NULL, -- GenieACS _id
    serial_number TEXT        NOT NULL DEFAULT '',
    manufacturer  TEXT        NOT NULL DEFAULT '',
    model         TEXT        NOT NULL DEFAULT '',
    wan_ip        TEXT        NOT NULL DEFAULT '',
    ssid          TEXT        NOT NULL DEFAULT '',
    last_inform_at TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX acs_devices_tenant_device_key ON acs_devices (tenant_id, device_id);
CREATE INDEX acs_devices_customer_idx ON acs_devices (customer_id);

-- +goose Down
DROP TABLE IF EXISTS acs_devices;
