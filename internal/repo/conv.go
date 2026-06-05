package repo

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

// --- pgtype <-> Go scalar helpers -------------------------------------------

// pgTextOrNull maps an empty string to SQL NULL, otherwise a valid text.
func pgTextOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textVal returns the string value or "" when NULL.
func textVal(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// pgInt8Ptr maps a *int64 to a nullable int8.
func pgInt8Ptr(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}

// int8Ptr maps a nullable int8 back to *int64.
func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

// tsPtr maps a nullable timestamptz back to *time.Time.
func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

// jsonOrEmpty ensures a non-nil JSON payload for NOT NULL jsonb columns.
func jsonOrEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

// --- model -> domain mappers ------------------------------------------------

func toDomainTenant(m sqlc.Tenant) tenant.Tenant {
	return tenant.Tenant{
		ID:        m.ID,
		Name:      m.Name,
		Slug:      m.Slug,
		Domain:    textVal(m.Domain),
		Branding:  m.Branding,
		Settings:  m.Settings,
		Status:    tenant.Status(m.Status),
		Plan:      m.Plan,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toDomainUser(m sqlc.User) iam.User {
	return iam.User{
		ID:           m.ID,
		TenantID:     m.TenantID,
		Name:         m.Name,
		Email:        m.Email,
		PasswordHash: m.PasswordHash,
		RoleID:       m.RoleID,
		ParentID:     int8Ptr(m.ParentID),
		BalanceIDR:   m.BalanceIdr,
		IsActive:     m.IsActive,
		LastLoginAt:  tsPtr(m.LastLoginAt),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func toDomainRole(m sqlc.Role) iam.Role {
	return iam.Role{
		ID:        m.ID,
		TenantID:  m.TenantID,
		Name:      m.Name,
		IsSystem:  m.IsSystem,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toDomainPermission(m sqlc.Permission) iam.Permission {
	return iam.Permission{ID: m.ID, Code: m.Code, Description: m.Description}
}

func toDomainRefreshToken(m sqlc.RefreshToken) iam.RefreshToken {
	return iam.RefreshToken{
		ID:        m.ID,
		UserID:    m.UserID,
		TokenHash: m.TokenHash,
		ExpiresAt: m.ExpiresAt,
		RevokedAt: tsPtr(m.RevokedAt),
		UserAgent: m.UserAgent,
		IP:        textVal(m.Ip),
		CreatedAt: m.CreatedAt,
	}
}

func toDomainAudit(m sqlc.AuditLog) audit.Entry {
	return audit.Entry{
		ID:          m.ID,
		TenantID:    m.TenantID,
		ActorUserID: int8Ptr(m.ActorUserID),
		Action:      m.Action,
		Entity:      m.Entity,
		EntityID:    m.EntityID,
		Diff:        m.Diff,
		IP:          textVal(m.Ip),
		CreatedAt:   m.CreatedAt,
	}
}
