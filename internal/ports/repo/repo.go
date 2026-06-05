// Package repo declares the repository interfaces (ports) that the service
// layer depends on, plus a transactional bundle so multiple repositories can
// participate in one database transaction. Concrete implementations live in
// internal/repo and are injected in cmd/*/main.go.
package repo

import (
	"context"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/domain/radius"
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

// PlanRepository persists service plans.
type PlanRepository interface {
	Create(ctx context.Context, p plan.Plan) (plan.Plan, error)
	GetByID(ctx context.Context, tenantID, id int64) (plan.Plan, error)
	List(ctx context.Context, tenantID int64, limit, offset int32) ([]plan.Plan, error)
	Count(ctx context.Context, tenantID int64) (int64, error)
	Update(ctx context.Context, p plan.Plan) (plan.Plan, error)
	SoftDelete(ctx context.Context, tenantID, id int64) error
}

// BandwidthProfileRepository persists per-plan bandwidth profiles.
type BandwidthProfileRepository interface {
	Upsert(ctx context.Context, b plan.BandwidthProfile) (plan.BandwidthProfile, error)
	GetByPlan(ctx context.Context, planID int64) (plan.BandwidthProfile, error)
}

// CustomerRepository persists subscribers.
type CustomerRepository interface {
	Create(ctx context.Context, c customer.Customer) (customer.Customer, error)
	GetByID(ctx context.Context, tenantID, id int64) (customer.Customer, error)
	List(ctx context.Context, tenantID int64, limit, offset int32) ([]customer.Customer, error)
	Count(ctx context.Context, tenantID int64) (int64, error)
	Update(ctx context.Context, c customer.Customer) (customer.Customer, error)
	UpdateStatus(ctx context.Context, tenantID, id int64, status customer.Status) error
	SoftDelete(ctx context.Context, tenantID, id int64) error
	SetActiveUntil(ctx context.Context, tenantID, id int64, until time.Time, status customer.Status) error
	ListExpiredActive(ctx context.Context, limit int32) ([]customer.Expired, error)
}

// BillingRepository persists invoices, items, payments and the cashbook ledger.
type BillingRepository interface {
	CreateInvoice(ctx context.Context, inv billing.Invoice) (billing.Invoice, error)
	AddInvoiceItem(ctx context.Context, it billing.InvoiceItem) (billing.InvoiceItem, error)
	GetInvoice(ctx context.Context, tenantID, id int64) (billing.Invoice, error)
	InvoiceItems(ctx context.Context, invoiceID int64) ([]billing.InvoiceItem, error)
	ListInvoices(ctx context.Context, tenantID int64, limit, offset int32) ([]billing.Invoice, error)
	SetInvoiceStatus(ctx context.Context, tenantID, id int64, status billing.Status) error
	MarkInvoicePaid(ctx context.Context, tenantID, id int64) error
	MarkOverdue(ctx context.Context) (int64, error)
	CreatePayment(ctx context.Context, p billing.Payment) (billing.Payment, error)
	ListPayments(ctx context.Context, tenantID int64, limit, offset int32) ([]billing.Payment, error)
	AddLedger(ctx context.Context, e billing.LedgerEntry) (billing.LedgerEntry, error)
	SumLedger(ctx context.Context, tenantID int64, from, to time.Time) (income, expense int64, err error)
	Outstanding(ctx context.Context, tenantID int64) (int64, error)
	GetPaymentByRef(ctx context.Context, ref string) (billing.Payment, error)
	SettlePayment(ctx context.Context, id int64, raw []byte) error
	MarkPaymentStatus(ctx context.Context, id int64, status billing.PaymentStatus, raw []byte) error
	ListPendingGatewayPayments(ctx context.Context, olderThan time.Time, limit int32) ([]billing.Payment, error)
	GetGatewayConfig(ctx context.Context, tenantID int64, provider string) (billing.GatewayConfig, error)
	UpsertGatewayConfig(ctx context.Context, cfg billing.GatewayConfig) (billing.GatewayConfig, error)
}

// RadiusAuthRepository is the read side used by the radius-service during
// authentication, plus the radpostauth audit write and NAS management.
type RadiusAuthRepository interface {
	NasByIP(ctx context.Context, ip string) (radius.Nas, error)
	CreateNas(ctx context.Context, n radius.Nas) (radius.Nas, error)
	UserCheck(ctx context.Context, tenantID int64, username string) ([]radius.Attr, error)
	UserReply(ctx context.Context, tenantID int64, username string) ([]radius.Attr, error)
	GroupReply(ctx context.Context, tenantID int64, groupname string) ([]radius.Attr, error)
	InsertPostAuth(ctx context.Context, pa radius.PostAuth) error
}

// AccountingRepository writes radacct and reads active sessions.
type AccountingRepository interface {
	Start(ctx context.Context, e radius.AcctEvent) error
	Interim(ctx context.Context, e radius.AcctEvent) error
	Stop(ctx context.Context, e radius.AcctEvent) error
	ActiveSessions(ctx context.Context, tenantID int64, username string) ([]radius.ActiveSession, error)
}

// RadiusMapRepository writes the FreeRADIUS provisioning tables (radcheck,
// radusergroup, radgroupreply) when customers and plans change.
type RadiusMapRepository interface {
	SetUserPassword(ctx context.Context, tenantID int64, username, password string) error
	SetUserGroup(ctx context.Context, tenantID int64, username, groupname string, priority int32) error
	ClearUser(ctx context.Context, tenantID int64, username string) error
	SetGroupReply(ctx context.Context, tenantID int64, groupname string, attrs []radius.Attr) error
	UserGroups(ctx context.Context, tenantID int64, username string) ([]radius.UserGroup, error)
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
	Plan         PlanRepository
	Bandwidth    BandwidthProfileRepository
	Customer     CustomerRepository
	RadiusMap    RadiusMapRepository
	RadiusAuth   RadiusAuthRepository
	Accounting   AccountingRepository
	Billing      BillingRepository
	Notification NotificationRepository
}

// NotificationRepository persists notifications (which double as the job queue),
// gateways and templates.
type NotificationRepository interface {
	Enqueue(ctx context.Context, job notification.Job) (bool, error)
	ClaimDue(ctx context.Context, limit int32, leaseUntil time.Time) ([]notification.Log, error)
	MarkSent(ctx context.Context, id int64, providerRef string) error
	Retry(ctx context.Context, id int64, errMsg string, nextAttempt time.Time) error
	Fail(ctx context.Context, id int64, errMsg string) error
	ActiveGateway(ctx context.Context, tenantID int64) (notification.Gateway, error)
	Template(ctx context.Context, tenantID int64, key string, channel notification.Channel) (notification.Template, error)
	UpsertTemplate(ctx context.Context, t notification.Template) (notification.Template, error)
	CreateGateway(ctx context.Context, g notification.Gateway) (notification.Gateway, error)
}

// TxManager runs fn inside a database transaction, providing repositories bound
// to that transaction. It commits if fn returns nil, otherwise rolls back.
type TxManager interface {
	WithTx(ctx context.Context, fn func(r Repositories) error) error
}
