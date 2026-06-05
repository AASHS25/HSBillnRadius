package authsvc_test

import (
	"context"
	"sync"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// fakeData is the shared in-memory backing store for the fake repositories.
// Each repository port is implemented by a distinct small type (their method
// sets collide on names like Create/GetByID, so they cannot share one type).
type fakeData struct {
	mu        sync.Mutex
	tenants   map[int64]tenant.Tenant
	users     map[int64]iam.User
	roles     map[int64]iam.Role
	rolePerms map[int64][]string
	tokens    map[string]iam.RefreshToken
	audits    []audit.Entry
	seqT      int64
	seqU      int64
	seqR      int64
	allPerms  []string
}

func newFakeData() *fakeData {
	return &fakeData{
		tenants:   map[int64]tenant.Tenant{},
		users:     map[int64]iam.User{},
		roles:     map[int64]iam.Role{},
		rolePerms: map[int64][]string{},
		tokens:    map[string]iam.RefreshToken{},
		allPerms:  []string{"tenant.read", "user.read", "user.create", "role.manage"},
	}
}

// fakeStore implements repo.TxManager and supplies the repository bundle.
type fakeStore struct{ d *fakeData }

func newFakeStore() *fakeStore { return &fakeStore{d: newFakeData()} }

func (s *fakeStore) repos() repo.Repositories {
	d := s.d
	return repo.Repositories{
		Tenant:       &fakeTenantRepo{d},
		User:         &fakeUserRepo{d},
		Role:         &fakeRoleRepo{d},
		Permission:   &fakePermRepo{d},
		RefreshToken: &fakeTokenRepo{d},
		Audit:        &fakeAuditRepo{d},
	}
}

func (s *fakeStore) WithTx(_ context.Context, fn func(r repo.Repositories) error) error {
	return fn(s.repos())
}

// --- tenants ----------------------------------------------------------------

type fakeTenantRepo struct{ d *fakeData }

func (r *fakeTenantRepo) Create(_ context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	for _, ex := range r.d.tenants {
		if ex.Slug == t.Slug {
			return tenant.Tenant{}, tenant.ErrSlugTaken
		}
	}
	r.d.seqT++
	t.ID = r.d.seqT
	t.CreatedAt, t.UpdatedAt = time.Now(), time.Now()
	r.d.tenants[t.ID] = t
	return t, nil
}

func (r *fakeTenantRepo) GetByID(_ context.Context, id int64) (tenant.Tenant, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	t, ok := r.d.tenants[id]
	if !ok {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	return t, nil
}

func (r *fakeTenantRepo) GetBySlug(_ context.Context, slug string) (tenant.Tenant, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	for _, t := range r.d.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (r *fakeTenantRepo) GetByDomain(_ context.Context, domain string) (tenant.Tenant, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	for _, t := range r.d.tenants {
		if domain != "" && t.Domain == domain {
			return t, nil
		}
	}
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (r *fakeTenantRepo) Update(_ context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	if _, ok := r.d.tenants[t.ID]; !ok {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	r.d.tenants[t.ID] = t
	return t, nil
}

// --- users ------------------------------------------------------------------

type fakeUserRepo struct{ d *fakeData }

func (r *fakeUserRepo) Create(_ context.Context, u iam.User) (iam.User, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	for _, ex := range r.d.users {
		if ex.TenantID == u.TenantID && ex.Email == u.Email {
			return iam.User{}, iam.ErrEmailTaken
		}
	}
	r.d.seqU++
	u.ID = r.d.seqU
	u.IsActive = true
	u.CreatedAt, u.UpdatedAt = time.Now(), time.Now()
	r.d.users[u.ID] = u
	return u, nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id int64) (iam.User, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	u, ok := r.d.users[id]
	if !ok {
		return iam.User{}, iam.ErrUserNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, tenantID int64, email string) (iam.User, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	for _, u := range r.d.users {
		if u.TenantID == tenantID && u.Email == email {
			return u, nil
		}
	}
	return iam.User{}, iam.ErrUserNotFound
}

func (r *fakeUserRepo) List(_ context.Context, tenantID int64, limit, offset int32) ([]iam.User, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	var out []iam.User
	for _, u := range r.d.users {
		if u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	if int(offset) >= len(out) {
		return []iam.User{}, nil
	}
	end := int(offset) + int(limit)
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (r *fakeUserRepo) Count(_ context.Context, tenantID int64) (int64, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	var n int64
	for _, u := range r.d.users {
		if u.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (r *fakeUserRepo) Update(_ context.Context, u iam.User) (iam.User, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	if _, ok := r.d.users[u.ID]; !ok {
		return iam.User{}, iam.ErrUserNotFound
	}
	r.d.users[u.ID] = u
	return u, nil
}

func (r *fakeUserRepo) UpdatePassword(_ context.Context, id int64, hash string) error {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	u, ok := r.d.users[id]
	if !ok {
		return iam.ErrUserNotFound
	}
	u.PasswordHash = hash
	r.d.users[id] = u
	return nil
}

func (r *fakeUserRepo) UpdateLastLogin(_ context.Context, id int64) error {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	u, ok := r.d.users[id]
	if !ok {
		return iam.ErrUserNotFound
	}
	now := time.Now()
	u.LastLoginAt = &now
	r.d.users[id] = u
	return nil
}

func (r *fakeUserRepo) SoftDelete(_ context.Context, tenantID, id int64) error {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	delete(r.d.users, id)
	return nil
}

// --- roles ------------------------------------------------------------------

type fakeRoleRepo struct{ d *fakeData }

func (r *fakeRoleRepo) Create(_ context.Context, role iam.Role) (iam.Role, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	r.d.seqR++
	role.ID = r.d.seqR
	r.d.roles[role.ID] = role
	return role, nil
}

func (r *fakeRoleRepo) GetByID(_ context.Context, id int64) (iam.Role, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	role, ok := r.d.roles[id]
	if !ok {
		return iam.Role{}, iam.ErrRoleNotFound
	}
	return role, nil
}

func (r *fakeRoleRepo) GetByName(_ context.Context, tenantID int64, name string) (iam.Role, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	for _, role := range r.d.roles {
		if role.TenantID == tenantID && role.Name == name {
			return role, nil
		}
	}
	return iam.Role{}, iam.ErrRoleNotFound
}

func (r *fakeRoleRepo) List(_ context.Context, tenantID int64) ([]iam.Role, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	var out []iam.Role
	for _, role := range r.d.roles {
		if role.TenantID == tenantID {
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *fakeRoleRepo) GrantAllPermissions(_ context.Context, roleID int64) error {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	r.d.rolePerms[roleID] = append([]string(nil), r.d.allPerms...)
	return nil
}

func (r *fakeRoleRepo) AddPermission(_ context.Context, roleID, _ int64) error {
	return nil
}

func (r *fakeRoleRepo) PermissionCodes(_ context.Context, roleID int64) ([]string, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	return append([]string(nil), r.d.rolePerms[roleID]...), nil
}

// --- permissions ------------------------------------------------------------

type fakePermRepo struct{ d *fakeData }

func (r *fakePermRepo) List(_ context.Context) ([]iam.Permission, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	out := make([]iam.Permission, len(r.d.allPerms))
	for i, c := range r.d.allPerms {
		out[i] = iam.Permission{ID: int64(i + 1), Code: c}
	}
	return out, nil
}

func (r *fakePermRepo) IDsByCodes(_ context.Context, codes []string) ([]int64, error) {
	ids := make([]int64, len(codes))
	for i := range codes {
		ids[i] = int64(i + 1)
	}
	return ids, nil
}

// --- refresh tokens ---------------------------------------------------------

type fakeTokenRepo struct{ d *fakeData }

func (r *fakeTokenRepo) Create(_ context.Context, t iam.RefreshToken) (iam.RefreshToken, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	t.ID = int64(len(r.d.tokens) + 1)
	t.CreatedAt = time.Now()
	r.d.tokens[t.TokenHash] = t
	return t, nil
}

func (r *fakeTokenRepo) GetByHash(_ context.Context, hash string) (iam.RefreshToken, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	t, ok := r.d.tokens[hash]
	if !ok {
		return iam.RefreshToken{}, iam.ErrTokenNotFound
	}
	return t, nil
}

func (r *fakeTokenRepo) Revoke(_ context.Context, hash string) error {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	if t, ok := r.d.tokens[hash]; ok && t.RevokedAt == nil {
		now := time.Now()
		t.RevokedAt = &now
		r.d.tokens[hash] = t
	}
	return nil
}

func (r *fakeTokenRepo) RevokeAllForUser(_ context.Context, userID int64) error {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	now := time.Now()
	for h, t := range r.d.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
			r.d.tokens[h] = t
		}
	}
	return nil
}

func (r *fakeTokenRepo) DeleteExpired(_ context.Context) (int64, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	var n int64
	for h, t := range r.d.tokens {
		if t.ExpiresAt.Before(time.Now()) {
			delete(r.d.tokens, h)
			n++
		}
	}
	return n, nil
}

// --- audit ------------------------------------------------------------------

type fakeAuditRepo struct{ d *fakeData }

func (r *fakeAuditRepo) Insert(_ context.Context, e audit.Entry) (audit.Entry, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	e.ID = int64(len(r.d.audits) + 1)
	e.CreatedAt = time.Now()
	r.d.audits = append(r.d.audits, e)
	return e, nil
}

func (r *fakeAuditRepo) List(_ context.Context, tenantID int64, limit, offset int32) ([]audit.Entry, error) {
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	var out []audit.Entry
	for _, e := range r.d.audits {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}
