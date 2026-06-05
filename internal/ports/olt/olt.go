// Package olt defines the multi-vendor OLT (GPON/EPON) driver interface. Vendor
// implementations (ZTE, VSOL, HIOSO, C-Data, ...) talk to devices over
// Telnet/SSH (CLI parsing) or SNMP and run in the worker, never on the request
// path. Optical power (redaman) monitoring is the key differentiator.
package olt

import "context"

// Device is an OLT to connect to.
type Device struct {
	ID          int64
	TenantID    int64
	Vendor      string
	Host        string
	MgmtProto   string // telnet | ssh | snmp
	Credentials map[string]string
	PONType     string // gpon | epon
}

// ONU is a customer-premises optical unit.
type ONU struct {
	SerialNumber string
	Slot         int
	PONPort      int
	Index        int
	Name         string
	Status       string // online | offline | los | unknown
	RxPower      float64
	TxPower      float64
}

// RegisterRequest authorizes a new ONU on the OLT.
type RegisterRequest struct {
	SerialNumber string
	Slot         int
	PONPort      int
	Profile      string
}

// Driver is the contract every OLT vendor adapter implements.
type Driver interface {
	// Vendor returns the vendor key this driver handles.
	Vendor() string
	// ListONU enumerates ONUs on the OLT.
	ListONU(ctx context.Context, dev Device) ([]ONU, error)
	// RegisterONU authorizes a new ONU.
	RegisterONU(ctx context.Context, dev Device, req RegisterRequest) error
	// RebootONU reboots an ONU.
	RebootONU(ctx context.Context, dev Device, onu ONU) error
	// DeleteONU removes an ONU.
	DeleteONU(ctx context.Context, dev Device, onu ONU) error
	// OpticalPower reads the ONU's rx/tx power in dBm (redaman monitoring).
	OpticalPower(ctx context.Context, dev Device, onu ONU) (rx, tx float64, err error)
}

// Registry maps a vendor key to its driver.
type Registry map[string]Driver

// Get returns the driver for a vendor, if registered.
func (r Registry) Get(vendor string) (Driver, bool) {
	d, ok := r[vendor]
	return d, ok
}
