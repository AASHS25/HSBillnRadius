package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/ports/acs"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type acsRepo struct{ q *sqlc.Queries }

func toStoredDevice(m sqlc.AcsDevice) acs.StoredDevice {
	return acs.StoredDevice{
		ID: m.ID, TenantID: m.TenantID, CustomerID: int8Ptr(m.CustomerID), DeviceID: m.DeviceID,
		SerialNumber: m.SerialNumber, Manufacturer: m.Manufacturer, Model: m.Model, WANIP: m.WanIp, SSID: m.Ssid,
	}
}

func (r *acsRepo) Upsert(ctx context.Context, d acs.StoredDevice) (acs.StoredDevice, error) {
	m, err := r.q.UpsertAcsDevice(ctx, sqlc.UpsertAcsDeviceParams{
		TenantID: d.TenantID, CustomerID: pgInt8Ptr(d.CustomerID), DeviceID: d.DeviceID,
		SerialNumber: d.SerialNumber, Manufacturer: d.Manufacturer, Model: d.Model, WanIp: d.WANIP, Ssid: d.SSID,
	})
	if err != nil {
		return acs.StoredDevice{}, fmt.Errorf("upsert acs device: %w", err)
	}
	return toStoredDevice(m), nil
}

func (r *acsRepo) List(ctx context.Context, tenantID int64, limit, offset int32) ([]acs.StoredDevice, error) {
	rows, err := r.q.ListAcsDevices(ctx, sqlc.ListAcsDevicesParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list acs devices: %w", err)
	}
	out := make([]acs.StoredDevice, len(rows))
	for i, m := range rows {
		out[i] = toStoredDevice(m)
	}
	return out, nil
}

func (r *acsRepo) Get(ctx context.Context, tenantID int64, deviceID string) (acs.StoredDevice, error) {
	m, err := r.q.GetAcsDevice(ctx, sqlc.GetAcsDeviceParams{TenantID: tenantID, DeviceID: deviceID})
	if err != nil {
		if isNotFound(err) {
			return acs.StoredDevice{}, fmt.Errorf("acs device not found")
		}
		return acs.StoredDevice{}, fmt.Errorf("get acs device: %w", err)
	}
	return toStoredDevice(m), nil
}
