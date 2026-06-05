// Package customer holds the customer (subscriber) entity and its rules.
package customer

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status is the lifecycle/service state of a customer.
type Status string

const (
	StatusNew        Status = "new"
	StatusActive     Status = "active"
	StatusIsolated   Status = "isolated"
	StatusSuspended  Status = "suspended"
	StatusTerminated Status = "terminated"
	StatusFree       Status = "free"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusActive, StatusIsolated, StatusSuspended, StatusTerminated, StatusFree:
		return true
	default:
		return false
	}
}

// Domain errors.
var (
	ErrNotFound      = errors.New("customer not found")
	ErrNoTaken       = errors.New("customer_no already in use")
	ErrUsernameTaken = errors.New("pppoe username already in use")
	ErrInvalidCust   = errors.New("invalid customer")
	ErrPlanRequired  = errors.New("plan is required for service provisioning")
	ErrCredsRequired = errors.New("pppoe username and password are required")
)

// Customer is a subscriber.
type Customer struct {
	ID            int64
	TenantID      int64
	CustomerNo    string
	Name          string
	IDCardNo      string
	Email         string
	PhoneWA       string
	Address       string
	Lat           *float64
	Lng           *float64
	InstallDate   *time.Time
	Status        Status
	PlanID        *int64
	ResellerID    *int64
	BalanceIDR    int64
	PppoeUsername string
	PppoePassword string
	Notes         string
	ActiveUntil   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// HasPPPoECredentials reports whether the customer has a username+password set.
func (c Customer) HasPPPoECredentials() bool {
	return c.PppoeUsername != "" && c.PppoePassword != ""
}

// Expired is the minimal projection used by the auto-isolir scan.
type Expired struct {
	ID            int64
	TenantID      int64
	PppoeUsername string
	PlanID        *int64
}

// Validate enforces customer invariants.
func (c Customer) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCust)
	}
	if strings.TrimSpace(c.CustomerNo) == "" {
		return fmt.Errorf("%w: customer_no is required", ErrInvalidCust)
	}
	if !c.Status.Valid() {
		return fmt.Errorf("%w: unknown status %q", ErrInvalidCust, c.Status)
	}
	return nil
}
