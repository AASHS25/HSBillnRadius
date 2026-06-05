// Package repo declares the repository interfaces (ports) that the service
// layer depends on, plus a transactional bundle so multiple repositories can
// participate in one database transaction. Concrete implementations live in
// internal/repo and are injected in cmd/*/main.go.
package repo

import (
	"context"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
)

// TenantRepository persists tenants.
type TenantRepository interface {
	Create(ctx context.Context, t tenant.Tenant) (tenant.Tenant, error)
	GetByID(ctx context.Context, id int64) (tenant.Tenant, error)
	GetBySlug(ctx context.Context, slug string) (tenant.Tenant, error)
	GetByDomain(ctx context.Context, domain string) (tenant.Tenant, error)
	Update(ctx context.Context, t tenant.Tenant) (tenant.Tenant, error)
}

// UserRepository persists users.
type UserRepository interface {
	Create(ctx context.Context, u iam.User) (iam.User, error)
	GetByID(ctx context.Context, id int64) (iam.User, error)
	GetByEmail(ctx context.Context, tenantID int64, email string) (iam.User, error)
	List(ctx context.Context, tenantID int64, limit, offset int32) ([]iam.User, error)
	Count(ctx context.Context, tenantID int64) (int64, error)
	Update(ctx context.Context, u iam.User) (iam.User, error)
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	UpdateLastLogin(ctx context.Context, id int64) error
	SoftDelete(ctx context.Context, tenantID, id int64) error
}

// RoleRepository persists roles and their permission grants.
type RoleRepository interface {
	Create(ctx context.Context, r iam.Role) (iam.Role, error)
	GetByID(ctx context.Context, id int64) (iam.Role, error)
	GetByName(ctx context.Context, tenantID int64, name string) (iam.Role, error)
	List(ctx context.Context, tenantID int64) ([]iam.Role, error)
	GrantAllPermissions(ctx context.Context, roleID int64) error
	AddPermission(ctx context.Context, roleID, permissionID int64) error
	PermissionCodes(ctx context.Context, roleID int64) ([]string, error)
}

// PermissionRepository reads the global permission catalog.
type PermissionRepository interface {
	List(ctx context.Context) ([]iam.Permission, error)
	IDsByCodes(ctx context.Context, codes []string) ([]int64, error)
}

// RefreshTokenRepository persists rotating refresh tokens.
type RefreshTokenRepository interface {
	Create(ctx context.Context, t iam.RefreshToken) (iam.RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (iam.RefreshToken, error)
	Revoke(ctx context.Context, hash string) error
	RevokeAllForUser(ctx context.Context, userID int64) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// AuditRepository persists audit-log entries.
type AuditRepository interface {
	Insert(ctx context.Context, e audit.Entry) (audit.Entry, error)
	List(ctx context.Context, tenantID int64, limit, offset int32) ([]audit.Entry, error)
}

// Repositories bundles every repository so they can be passed together and,
// inside a transaction, share the same connection.
type Repositories struct {
	Tenant       TenantRepository
	User         UserRepository
	Role         RoleRepository
	Permission   PermissionRepository
	RefreshToken RefreshTokenRepository
	Audit        AuditRepository
}

// TxManager runs fn inside a database transaction, providing repositories bound
// to that transaction. It commits if fn returns nil, otherwise rolls back.
type TxManager interface {
	WithTx(ctx context.Context, fn func(r Repositories) error) error
}
