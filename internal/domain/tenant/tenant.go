// Package tenant holds the tenant entity and its business rules. It has no
// dependencies outside the standard library.
package tenant

import (
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

// Status is the lifecycle state of a tenant.
type Status string

const (
	StatusTrial     Status = "trial"
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusClosed    Status = "closed"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusTrial, StatusActive, StatusSuspended, StatusClosed:
		return true
	default:
		return false
	}
}

// Domain errors. Transport maps these to HTTP status codes.
var (
	ErrNotFound    = errors.New("tenant not found")
	ErrSlugTaken   = errors.New("tenant slug already in use")
	ErrInvalidSlug = errors.New("invalid tenant slug")
)

// Tenant is an isolated operator/ISP account.
type Tenant struct {
	ID        int64
	Name      string
	Slug      string
	Domain    string // empty when no custom domain
	Branding  json.RawMessage
	Settings  json.RawMessage
	Status    Status
	Plan      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])$`)

// ValidateSlug enforces a DNS-label-like slug (3-40 chars, lowercase).
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}
