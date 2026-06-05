// Package plan holds the service plan and bandwidth profile entities plus the
// rules that translate a plan into RADIUS reply attributes (Mikrotik VSA).
package plan

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ServiceType is the kind of service a plan provides.
type ServiceType string

const (
	ServicePPPoE   ServiceType = "pppoe"
	ServiceHotspot ServiceType = "hotspot"
	ServiceDHCP    ServiceType = "dhcp"
)

// Valid reports whether s is a known service type.
func (s ServiceType) Valid() bool {
	switch s {
	case ServicePPPoE, ServiceHotspot, ServiceDHCP:
		return true
	default:
		return false
	}
}

// BillingCycle is how a plan is billed.
type BillingCycle string

const (
	CycleMonthly      BillingCycle = "monthly"
	CycleFixed        BillingCycle = "fixed"
	CycleProfile      BillingCycle = "profile"
	CyclePrepaidTopup BillingCycle = "prepaid_topup"
)

// Valid reports whether c is a known billing cycle.
func (c BillingCycle) Valid() bool {
	switch c {
	case CycleMonthly, CycleFixed, CycleProfile, CyclePrepaidTopup:
		return true
	default:
		return false
	}
}

// Domain errors.
var (
	ErrNotFound     = errors.New("plan not found")
	ErrNameTaken    = errors.New("plan name already in use")
	ErrInvalidPlan  = errors.New("invalid plan")
	ErrInvalidPrice = errors.New("plan price must not be negative")
)

// Plan is a sellable service package.
type Plan struct {
	ID           int64
	TenantID     int64
	Name         string
	ServiceType  ServiceType
	PriceIDR     int64
	TaxBps       int32 // tax in basis points (1% = 100 bps)
	BillingCycle BillingCycle
	ActiveDays   int32
	DataQuotaMB  *int64
	TimeQuotaSec *int64
	IsUnlimited  bool
	PoolName     string
	IsolirPlanID *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GroupName is the RADIUS groupname that represents this plan.
func (p Plan) GroupName() string { return fmt.Sprintf("plan_%d", p.ID) }

// TaxIDR returns the integer tax amount for this plan's price.
func (p Plan) TaxIDR() int64 { return p.PriceIDR * int64(p.TaxBps) / 10000 }

// TotalIDR returns price plus tax.
func (p Plan) TotalIDR() int64 { return p.PriceIDR + p.TaxIDR() }

// Validate enforces plan invariants.
func (p Plan) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPlan)
	}
	if !p.ServiceType.Valid() {
		return fmt.Errorf("%w: unknown service_type %q", ErrInvalidPlan, p.ServiceType)
	}
	if !p.BillingCycle.Valid() {
		return fmt.Errorf("%w: unknown billing_cycle %q", ErrInvalidPlan, p.BillingCycle)
	}
	if p.PriceIDR < 0 {
		return ErrInvalidPrice
	}
	if p.ActiveDays <= 0 {
		return fmt.Errorf("%w: active_days must be positive", ErrInvalidPlan)
	}
	return nil
}

// BandwidthProfile holds the speed/burst settings for a plan.
type BandwidthProfile struct {
	ID                 int64
	TenantID           int64
	PlanID             int64
	RateLimitRx        string
	RateLimitTx        string
	BurstRx            string
	BurstTx            string
	BurstThresholdRx   string
	BurstThresholdTx   string
	BurstTime          string
	Priority           int32
	MikrotikRateString string
}

// MikrotikRate returns the Mikrotik-Rate-Limit string for this profile,
// preferring an explicit override and otherwise composing it from the parts.
// Format: rx/tx [burstRx/burstTx [threshRx/threshTx [time/time [priority]]]].
func (b BandwidthProfile) MikrotikRate() string {
	if b.MikrotikRateString != "" {
		return b.MikrotikRateString
	}
	if b.RateLimitRx == "" || b.RateLimitTx == "" {
		return ""
	}

	parts := []string{b.RateLimitRx + "/" + b.RateLimitTx}
	if b.BurstRx != "" && b.BurstTx != "" {
		parts = append(parts, b.BurstRx+"/"+b.BurstTx)
		if b.BurstThresholdRx != "" && b.BurstThresholdTx != "" {
			parts = append(parts, b.BurstThresholdRx+"/"+b.BurstThresholdTx)
			if b.BurstTime != "" {
				parts = append(parts, b.BurstTime+"/"+b.BurstTime)
				if b.Priority > 0 {
					parts = append(parts, fmt.Sprintf("%d", b.Priority))
				}
			}
		}
	}
	return strings.Join(parts, " ")
}
