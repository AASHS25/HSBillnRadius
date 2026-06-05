package repo

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
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

// pgFloat8Ptr maps a *float64 to a nullable float8.
func pgFloat8Ptr(p *float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *p, Valid: true}
}

// float8Ptr maps a nullable float8 back to *float64.
func float8Ptr(v pgtype.Float8) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

// pgDatePtr maps a *time.Time to a nullable date.
func pgDatePtr(p *time.Time) pgtype.Date {
	if p == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *p, Valid: true}
}

// datePtr maps a nullable date back to *time.Time.
func datePtr(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// pgDate maps a (non-null) time to a valid date.
func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// dateVal returns the date's time, or the zero time when NULL.
func dateVal(v pgtype.Date) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

// pgTimestamptzPtr maps a *time.Time to a nullable timestamptz.
func pgTimestamptzPtr(p *time.Time) pgtype.Timestamptz {
	if p == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *p, Valid: true}
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

func toDomainPlan(m sqlc.Plan) plan.Plan {
	return plan.Plan{
		ID:           m.ID,
		TenantID:     m.TenantID,
		Name:         m.Name,
		ServiceType:  plan.ServiceType(m.ServiceType),
		PriceIDR:     m.PriceIdr,
		TaxBps:       m.TaxBps,
		BillingCycle: plan.BillingCycle(m.BillingCycle),
		ActiveDays:   m.ActiveDays,
		DataQuotaMB:  int8Ptr(m.DataQuotaMb),
		TimeQuotaSec: int8Ptr(m.TimeQuotaSec),
		IsUnlimited:  m.IsUnlimited,
		PoolName:     m.PoolName,
		IsolirPlanID: int8Ptr(m.IsolirPlanID),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func toDomainBandwidth(m sqlc.BandwidthProfile) plan.BandwidthProfile {
	return plan.BandwidthProfile{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		PlanID:             m.PlanID,
		RateLimitRx:        m.RateLimitRx,
		RateLimitTx:        m.RateLimitTx,
		BurstRx:            m.BurstRx,
		BurstTx:            m.BurstTx,
		BurstThresholdRx:   m.BurstThresholdRx,
		BurstThresholdTx:   m.BurstThresholdTx,
		BurstTime:          m.BurstTime,
		Priority:           m.Priority,
		MikrotikRateString: m.MikrotikRateString,
	}
}

func toDomainCustomer(m sqlc.Customer) customer.Customer {
	return customer.Customer{
		ID:            m.ID,
		TenantID:      m.TenantID,
		CustomerNo:    m.CustomerNo,
		Name:          m.Name,
		IDCardNo:      m.IDCardNo,
		Email:         m.Email,
		PhoneWA:       m.PhoneWa,
		Address:       m.Address,
		Lat:           float8Ptr(m.Lat),
		Lng:           float8Ptr(m.Lng),
		InstallDate:   datePtr(m.InstallDate),
		Status:        customer.Status(m.Status),
		PlanID:        int8Ptr(m.PlanID),
		ResellerID:    int8Ptr(m.ResellerID),
		BalanceIDR:    m.BalanceIdr,
		PppoeUsername: textVal(m.PppoeUsername),
		PppoePassword: textVal(m.PppoePassword),
		Notes:         m.Notes,
		ActiveUntil:   tsPtr(m.ActiveUntil),
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
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
