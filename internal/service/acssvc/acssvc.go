// Package acssvc orchestrates TR-069 device management: it reads live CPE
// parameters from the ACS, pushes Wi-Fi/parameter changes, and keeps a local
// tenant<->device mapping for listing.
package acssvc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aashs25/hsbillnradius/internal/ports/acs"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// ErrNotConfigured is returned when no ACS is configured for the platform.
var ErrNotConfigured = errors.New("ACS is not configured")

// Service provides ACS operations.
type Service struct {
	repos  repo.Repositories
	client acs.Client // may be nil when ACS is disabled
	log    *slog.Logger
}

// New builds an ACS Service. client may be nil (ACS disabled).
func New(repos repo.Repositories, client acs.Client, log *slog.Logger) *Service {
	return &Service{repos: repos, client: client, log: log}
}

// Enabled reports whether an ACS client is configured.
func (s *Service) Enabled() bool { return s.client != nil }

// GetDevice reads live parameters and refreshes the local mapping.
func (s *Service) GetDevice(ctx context.Context, tenantID int64, deviceID string, customerID *int64) (acs.Device, error) {
	if s.client == nil {
		return acs.Device{}, ErrNotConfigured
	}
	dev, err := s.client.GetDevice(ctx, deviceID)
	if err != nil {
		return acs.Device{}, err
	}
	if _, err := s.repos.AcsDevice.Upsert(ctx, acs.StoredDevice{
		TenantID: tenantID, CustomerID: customerID, DeviceID: deviceID,
		SerialNumber: dev.SerialNumber, Manufacturer: dev.Manufacturer, Model: dev.Model,
		WANIP: dev.WANIP, SSID: dev.SSID,
	}); err != nil {
		s.log.WarnContext(ctx, "acs device upsert failed", slog.Any("error", err))
	}
	return dev, nil
}

// SetWiFi changes the SSID and passphrase.
func (s *Service) SetWiFi(ctx context.Context, deviceID, ssid, password string) error {
	if s.client == nil {
		return ErrNotConfigured
	}
	return s.client.SetWiFi(ctx, deviceID, ssid, password)
}

// Reboot reboots the CPE.
func (s *Service) Reboot(ctx context.Context, deviceID string) error {
	if s.client == nil {
		return ErrNotConfigured
	}
	return s.client.Reboot(ctx, deviceID)
}

// Refresh re-reads all CPE parameters.
func (s *Service) Refresh(ctx context.Context, deviceID string) error {
	if s.client == nil {
		return ErrNotConfigured
	}
	return s.client.Refresh(ctx, deviceID)
}

// ListDevices returns the stored device mapping for a tenant.
func (s *Service) ListDevices(ctx context.Context, tenantID int64, limit, offset int32) ([]acs.StoredDevice, error) {
	return s.repos.AcsDevice.List(ctx, tenantID, limit, offset)
}
