package memrepo

import (
	"context"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
)

// page applies limit/offset to a slice length, returning the sub-bounds.
func page[T any](items []T, limit, offset int32) []T {
	if int(offset) >= len(items) {
		return []T{}
	}
	end := int(offset) + int(limit)
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// --- tenants ----------------------------------------------------------------

type tenantRepo struct{ s *Store }

func (r *tenantRepo) Create(_ context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, ex := range r.s.tenants {
		if ex.Slug == t.Slug {
			return tenant.Tenant{}, tenant.ErrSlugTaken
		}
	}
	t.ID = r.s.next("tenant")
	t.CreatedAt, t.UpdatedAt = time.Now(), time.Now()
	r.s.tenants[t.ID] = t
	return t, nil
}

func (r *tenantRepo) GetByID(_ context.Context, id int64) (tenant.Tenant, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.tenants[id]
	if !ok {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	return t, nil
}

func (r *tenantRepo) GetBySlug(_ context.Context, slug string) (tenant.Tenant, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (r *tenantRepo) GetByDomain(_ context.Context, domain string) (tenant.Tenant, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.tenants {
		if domain != "" && t.Domain == domain {
			return t, nil
		}
	}
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (r *tenantRepo) Update(_ context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.tenants[t.ID]; !ok {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	r.s.tenants[t.ID] = t
	return t, nil
}

// --- users ------------------------------------------------------------------

type userRepo struct{ s *Store }

func (r *userRepo) Create(_ context.Context, u iam.User) (iam.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, ex := range r.s.users {
		if ex.TenantID == u.TenantID && ex.Email == u.Email {
			return iam.User{}, iam.ErrEmailTaken
		}
	}
	u.ID = r.s.next("user")
	u.IsActive = true
	u.CreatedAt, u.UpdatedAt = time.Now(), time.Now()
	r.s.users[u.ID] = u
	return u, nil
}

func (r *userRepo) GetByID(_ context.Context, id int64) (iam.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return iam.User{}, iam.ErrUserNotFound
	}
	return u, nil
}

func (r *userRepo) GetByEmail(_ context.Context, tenantID int64, email string) (iam.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.users {
		if u.TenantID == tenantID && u.Email == email {
			return u, nil
		}
	}
	return iam.User{}, iam.ErrUserNotFound
}

func (r *userRepo) List(_ context.Context, tenantID int64, limit, offset int32) ([]iam.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []iam.User
	for _, u := range r.s.users {
		if u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	return page(out, limit, offset), nil
}

func (r *userRepo) Count(_ context.Context, tenantID int64) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var n int64
	for _, u := range r.s.users {
		if u.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (r *userRepo) Update(_ context.Context, u iam.User) (iam.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.users[u.ID]; !ok {
		return iam.User{}, iam.ErrUserNotFound
	}
	r.s.users[u.ID] = u
	return u, nil
}

func (r *userRepo) UpdatePassword(_ context.Context, id int64, hash string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return iam.ErrUserNotFound
	}
	u.PasswordHash = hash
	r.s.users[id] = u
	return nil
}

func (r *userRepo) UpdateLastLogin(_ context.Context, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return iam.ErrUserNotFound
	}
	now := time.Now()
	u.LastLoginAt = &now
	r.s.users[id] = u
	return nil
}

func (r *userRepo) SoftDelete(_ context.Context, _, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	delete(r.s.users, id)
	return nil
}

// --- roles ------------------------------------------------------------------

type roleRepo struct{ s *Store }

func (r *roleRepo) Create(_ context.Context, role iam.Role) (iam.Role, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	role.ID = r.s.next("role")
	r.s.roles[role.ID] = role
	return role, nil
}

func (r *roleRepo) GetByID(_ context.Context, id int64) (iam.Role, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	role, ok := r.s.roles[id]
	if !ok {
		return iam.Role{}, iam.ErrRoleNotFound
	}
	return role, nil
}

func (r *roleRepo) GetByName(_ context.Context, tenantID int64, name string) (iam.Role, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, role := range r.s.roles {
		if role.TenantID == tenantID && role.Name == name {
			return role, nil
		}
	}
	return iam.Role{}, iam.ErrRoleNotFound
}

func (r *roleRepo) List(_ context.Context, tenantID int64) ([]iam.Role, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []iam.Role
	for _, role := range r.s.roles {
		if role.TenantID == tenantID {
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *roleRepo) GrantAllPermissions(_ context.Context, roleID int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.rolePerms[roleID] = append([]string(nil), r.s.allPerms...)
	return nil
}

func (r *roleRepo) AddPermission(_ context.Context, _, _ int64) error { return nil }

func (r *roleRepo) PermissionCodes(_ context.Context, roleID int64) ([]string, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return append([]string(nil), r.s.rolePerms[roleID]...), nil
}

// --- permissions ------------------------------------------------------------

type permRepo struct{ s *Store }

func (r *permRepo) List(_ context.Context) ([]iam.Permission, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := make([]iam.Permission, len(r.s.allPerms))
	for i, c := range r.s.allPerms {
		out[i] = iam.Permission{ID: int64(i + 1), Code: c}
	}
	return out, nil
}

func (r *permRepo) IDsByCodes(_ context.Context, codes []string) ([]int64, error) {
	ids := make([]int64, len(codes))
	for i := range codes {
		ids[i] = int64(i + 1)
	}
	return ids, nil
}

// --- refresh tokens ---------------------------------------------------------

type tokenRepo struct{ s *Store }

func (r *tokenRepo) Create(_ context.Context, t iam.RefreshToken) (iam.RefreshToken, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t.ID = r.s.next("token")
	t.CreatedAt = time.Now()
	r.s.tokens[t.TokenHash] = t
	return t, nil
}

func (r *tokenRepo) GetByHash(_ context.Context, hash string) (iam.RefreshToken, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.tokens[hash]
	if !ok {
		return iam.RefreshToken{}, iam.ErrTokenNotFound
	}
	return t, nil
}

func (r *tokenRepo) Revoke(_ context.Context, hash string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if t, ok := r.s.tokens[hash]; ok && t.RevokedAt == nil {
		now := time.Now()
		t.RevokedAt = &now
		r.s.tokens[hash] = t
	}
	return nil
}

func (r *tokenRepo) RevokeAllForUser(_ context.Context, userID int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	now := time.Now()
	for h, t := range r.s.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
			r.s.tokens[h] = t
		}
	}
	return nil
}

func (r *tokenRepo) DeleteExpired(_ context.Context) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var n int64
	for h, t := range r.s.tokens {
		if t.ExpiresAt.Before(time.Now()) {
			delete(r.s.tokens, h)
			n++
		}
	}
	return n, nil
}

// --- audit ------------------------------------------------------------------

type auditRepo struct{ s *Store }

func (r *auditRepo) Insert(_ context.Context, e audit.Entry) (audit.Entry, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e.ID = r.s.next("audit")
	e.CreatedAt = time.Now()
	r.s.audits = append(r.s.audits, e)
	return e, nil
}

func (r *auditRepo) List(_ context.Context, tenantID int64, limit, offset int32) ([]audit.Entry, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []audit.Entry
	for _, e := range r.s.audits {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return page(out, limit, offset), nil
}

// --- plans ------------------------------------------------------------------

type planRepo struct{ s *Store }

func (r *planRepo) Create(_ context.Context, p plan.Plan) (plan.Plan, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, ex := range r.s.plans {
		if ex.TenantID == p.TenantID && ex.Name == p.Name {
			return plan.Plan{}, plan.ErrNameTaken
		}
	}
	p.ID = r.s.next("plan")
	p.CreatedAt, p.UpdatedAt = time.Now(), time.Now()
	r.s.plans[p.ID] = p
	return p, nil
}

func (r *planRepo) GetByID(_ context.Context, tenantID, id int64) (plan.Plan, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.plans[id]
	if !ok || p.TenantID != tenantID {
		return plan.Plan{}, plan.ErrNotFound
	}
	return p, nil
}

func (r *planRepo) List(_ context.Context, tenantID int64, limit, offset int32) ([]plan.Plan, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []plan.Plan
	for _, p := range r.s.plans {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return page(out, limit, offset), nil
}

func (r *planRepo) Count(_ context.Context, tenantID int64) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var n int64
	for _, p := range r.s.plans {
		if p.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (r *planRepo) Update(_ context.Context, p plan.Plan) (plan.Plan, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	ex, ok := r.s.plans[p.ID]
	if !ok || ex.TenantID != p.TenantID {
		return plan.Plan{}, plan.ErrNotFound
	}
	p.CreatedAt = ex.CreatedAt
	p.UpdatedAt = time.Now()
	r.s.plans[p.ID] = p
	return p, nil
}

func (r *planRepo) SoftDelete(_ context.Context, tenantID, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if p, ok := r.s.plans[id]; ok && p.TenantID == tenantID {
		delete(r.s.plans, id)
	}
	return nil
}

// --- bandwidth profiles -----------------------------------------------------

type bandwidthRepo struct{ s *Store }

func (r *bandwidthRepo) Upsert(_ context.Context, b plan.BandwidthProfile) (plan.BandwidthProfile, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if b.ID == 0 {
		if ex, ok := r.s.bandwidth[b.PlanID]; ok {
			b.ID = ex.ID
		} else {
			b.ID = r.s.next("bandwidth")
		}
	}
	r.s.bandwidth[b.PlanID] = b
	return b, nil
}

func (r *bandwidthRepo) GetByPlan(_ context.Context, planID int64) (plan.BandwidthProfile, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	b, ok := r.s.bandwidth[planID]
	if !ok {
		return plan.BandwidthProfile{}, plan.ErrNotFound
	}
	return b, nil
}

// --- customers --------------------------------------------------------------

type customerRepo struct{ s *Store }

func (r *customerRepo) Create(_ context.Context, c customer.Customer) (customer.Customer, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, ex := range r.s.customers {
		if ex.TenantID == c.TenantID && ex.CustomerNo == c.CustomerNo {
			return customer.Customer{}, customer.ErrNoTaken
		}
		if c.PppoeUsername != "" && ex.TenantID == c.TenantID && ex.PppoeUsername == c.PppoeUsername {
			return customer.Customer{}, customer.ErrUsernameTaken
		}
	}
	c.ID = r.s.next("customer")
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	r.s.customers[c.ID] = c
	return c, nil
}

func (r *customerRepo) GetByID(_ context.Context, tenantID, id int64) (customer.Customer, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.customers[id]
	if !ok || c.TenantID != tenantID {
		return customer.Customer{}, customer.ErrNotFound
	}
	return c, nil
}

func (r *customerRepo) List(_ context.Context, tenantID int64, limit, offset int32) ([]customer.Customer, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []customer.Customer
	for _, c := range r.s.customers {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return page(out, limit, offset), nil
}

func (r *customerRepo) Count(_ context.Context, tenantID int64) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var n int64
	for _, c := range r.s.customers {
		if c.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (r *customerRepo) Update(_ context.Context, c customer.Customer) (customer.Customer, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	ex, ok := r.s.customers[c.ID]
	if !ok || ex.TenantID != c.TenantID {
		return customer.Customer{}, customer.ErrNotFound
	}
	c.CreatedAt = ex.CreatedAt
	c.UpdatedAt = time.Now()
	r.s.customers[c.ID] = c
	return c, nil
}

func (r *customerRepo) UpdateStatus(_ context.Context, tenantID, id int64, status customer.Status) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.customers[id]
	if !ok || c.TenantID != tenantID {
		return customer.ErrNotFound
	}
	c.Status = status
	r.s.customers[id] = c
	return nil
}

func (r *customerRepo) SoftDelete(_ context.Context, tenantID, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if c, ok := r.s.customers[id]; ok && c.TenantID == tenantID {
		delete(r.s.customers, id)
	}
	return nil
}

// --- radius map -------------------------------------------------------------

type radiusRepo struct{ s *Store }

func (r *radiusRepo) SetUserPassword(_ context.Context, tenantID int64, username, pw string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.radcheck[key(tenantID, username)] = []radius.Attr{{Attribute: radius.AttrCleartextPassword, Op: ":=", Value: pw}}
	return nil
}

func (r *radiusRepo) SetUserGroup(_ context.Context, tenantID int64, username, groupname string, priority int32) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.radgroup[key(tenantID, username)] = radius.UserGroup{Username: username, Groupname: groupname, Priority: priority}
	return nil
}

func (r *radiusRepo) ClearUser(_ context.Context, tenantID int64, username string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := key(tenantID, username)
	delete(r.s.radcheck, k)
	delete(r.s.radgroup, k)
	delete(r.s.radreply, k)
	return nil
}

func (r *radiusRepo) SetGroupReply(_ context.Context, tenantID int64, groupname string, attrs []radius.Attr) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.radreply[key(tenantID, groupname)] = append([]radius.Attr(nil), attrs...)
	return nil
}

func (r *radiusRepo) UserGroups(_ context.Context, tenantID int64, username string) ([]radius.UserGroup, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if g, ok := r.s.radgroup[key(tenantID, username)]; ok {
		return []radius.UserGroup{g}, nil
	}
	return []radius.UserGroup{}, nil
}

// --- radius auth (read side) ------------------------------------------------

type radiusAuthRepo struct{ s *Store }

func (r *radiusAuthRepo) NasByIP(_ context.Context, ip string) (radius.Nas, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	n, ok := r.s.nas[ip]
	if !ok {
		return radius.Nas{}, radius.ErrNasNotFound
	}
	return n, nil
}

func (r *radiusAuthRepo) CreateNas(_ context.Context, n radius.Nas) (radius.Nas, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.nas[n.Name]; ok {
		return radius.Nas{}, radius.ErrNasExists
	}
	n.ID = r.s.next("nas")
	r.s.nas[n.Name] = n
	return n, nil
}

func (r *radiusAuthRepo) UserCheck(_ context.Context, tenantID int64, username string) ([]radius.Attr, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return append([]radius.Attr(nil), r.s.radcheck[key(tenantID, username)]...), nil
}

func (r *radiusAuthRepo) UserReply(_ context.Context, _ int64, _ string) ([]radius.Attr, error) {
	return nil, nil
}

func (r *radiusAuthRepo) GroupReply(_ context.Context, tenantID int64, groupname string) ([]radius.Attr, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return append([]radius.Attr(nil), r.s.radreply[key(tenantID, groupname)]...), nil
}

func (r *radiusAuthRepo) InsertPostAuth(_ context.Context, pa radius.PostAuth) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.postauth = append(r.s.postauth, pa)
	return nil
}
