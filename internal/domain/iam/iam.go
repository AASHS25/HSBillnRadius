// Package iam (identity & access management) holds the user, role, permission
// and refresh-token entities plus their business rules. No external deps.
package iam

import (
	"errors"
	"strings"
	"time"
)

// Domain errors mapped to HTTP status codes by the transport layer.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrRoleNotFound      = errors.New("role not found")
	ErrEmailTaken        = errors.New("email already in use")
	ErrInvalidCredential = errors.New("invalid email or password")
	ErrUserInactive      = errors.New("user is inactive")
	ErrWeakPassword      = errors.New("password must be at least 8 characters")
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrTokenNotFound     = errors.New("refresh token not found")
)

// DefaultOwnerRole is the name of the all-permissions role created for a new
// tenant's first user.
const DefaultOwnerRole = "owner"

// User is an operator/admin/reseller account within a tenant.
type User struct {
	ID           int64
	TenantID     int64
	Name         string
	Email        string
	PasswordHash string // never serialized to clients
	RoleID       int64
	ParentID     *int64 // reseller hierarchy
	BalanceIDR   int64
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Role is a named set of permissions within a tenant.
type Role struct {
	ID        int64
	TenantID  int64
	Name      string
	IsSystem  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Permission is a globally-defined capability code (e.g. "user.create").
type Permission struct {
	ID          int64
	Code        string
	Description string
}

// RefreshToken is a rotating, revocable session credential stored hashed.
type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IP        string
	CreatedAt time.Time
}

// Active reports whether the refresh token may still be used.
func (t RefreshToken) Active(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// NormalizeEmail trims and lowercases an email for consistent comparison.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail performs a minimal structural email check.
func ValidateEmail(email string) error {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 || strings.Contains(email, " ") {
		return ErrInvalidEmail
	}
	if !strings.Contains(email[at+1:], ".") {
		return ErrInvalidEmail
	}
	return nil
}

// ValidatePassword enforces the minimum password policy.
func ValidatePassword(pw string) error {
	if len(pw) < 8 {
		return ErrWeakPassword
	}
	return nil
}
