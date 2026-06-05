// Package reseller holds the reseller deposit and commission entities.
package reseller

import (
	"errors"
	"time"
)

// ErrInvalidAmount is returned for non-positive amounts.
var ErrInvalidAmount = errors.New("amount must be positive")

// Deposit is a balance top-up for a reseller user.
type Deposit struct {
	ID         int64
	TenantID   int64
	UserID     int64
	AmountIDR  int64
	Method     string
	GatewayRef string
	Status     string
	CreatedAt  time.Time
}

// Commission is earned by a reseller when one of their customers pays.
type Commission struct {
	ID              int64
	TenantID        int64
	ResellerID      int64
	SourcePaymentID *int64
	AmountIDR       int64
	Status          string
	CreatedAt       time.Time
}
