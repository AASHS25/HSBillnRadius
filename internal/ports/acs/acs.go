// Package acs defines the TR-069 ACS (Auto Configuration Server) client port and
// a GenieACS Northbound-Interface implementation. It lets operators read CPE
// parameters and push changes (Wi-Fi SSID/password, reboot, refresh) remotely.
package acs

import (
	"context"
	"time"
)

// Device is the live view of a CPE from the ACS.
type Device struct {
	ID           string
	SerialNumber string
	Manufacturer string
	Model        string
	WANIP        string
	SSID         string
	LastInform   time.Time
}

// StoredDevice is the persisted tenant<->device mapping.
type StoredDevice struct {
	ID           int64
	TenantID     int64
	CustomerID   *int64
	DeviceID     string
	SerialNumber string
	Manufacturer string
	Model        string
	WANIP        string
	SSID         string
}

// Client talks to a TR-069 ACS.
type Client interface {
	// GetDevice fetches a device's current parameters by ACS device id.
	GetDevice(ctx context.Context, deviceID string) (Device, error)
	// SetWiFi sets the 2.4GHz SSID and pre-shared key.
	SetWiFi(ctx context.Context, deviceID, ssid, password string) error
	// SetParameter queues a setParameterValues task for one TR-069 parameter.
	SetParameter(ctx context.Context, deviceID, name, value, xsdType string) error
	// Reboot queues a reboot task.
	Reboot(ctx context.Context, deviceID string) error
	// Refresh queues a refreshObject task to re-read all parameters.
	Refresh(ctx context.Context, deviceID string) error
}
