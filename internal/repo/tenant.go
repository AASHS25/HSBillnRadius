package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type tenantRepo struct{ q *sqlc.Queries }

func (r *tenantRepo) Create(ctx context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	m, err := r.q.CreateTenant(ctx, sqlc.CreateTenantParams{
		Name:     t.Name,
		Slug:     t.Slug,
		Domain:   pgTextOrNull(t.Domain),
		Branding: jsonOrEmpty(t.Branding),
		Settings: jsonOrEmpty(t.Settings),
		Status:   sqlc.TenantStatus(t.Status),
		Plan:     t.Plan,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return tenant.Tenant{}, tenant.ErrSlugTaken
		}
		return tenant.Tenant{}, fmt.Errorf("create tenant: %w", err)
	}
	return toDomainTenant(m), nil
}

func (r *tenantRepo) GetByID(ctx context.Context, id int64) (tenant.Tenant, error) {
	m, err := r.q.GetTenantByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return tenant.Tenant{}, tenant.ErrNotFound
		}
		return tenant.Tenant{}, fmt.Errorf("get tenant by id: %w", err)
	}
	return toDomainTenant(m), nil
}

func (r *tenantRepo) GetBySlug(ctx context.Context, slug string) (tenant.Tenant, error) {
	m, err := r.q.GetTenantBySlug(ctx, slug)
	if err != nil {
		if isNotFound(err) {
			return tenant.Tenant{}, tenant.ErrNotFound
		}
		return tenant.Tenant{}, fmt.Errorf("get tenant by slug: %w", err)
	}
	return toDomainTenant(m), nil
}

func (r *tenantRepo) GetByDomain(ctx context.Context, domain string) (tenant.Tenant, error) {
	m, err := r.q.GetTenantByDomain(ctx, pgTextOrNull(domain))
	if err != nil {
		if isNotFound(err) {
			return tenant.Tenant{}, tenant.ErrNotFound
		}
		return tenant.Tenant{}, fmt.Errorf("get tenant by domain: %w", err)
	}
	return toDomainTenant(m), nil
}

func (r *tenantRepo) Update(ctx context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	m, err := r.q.UpdateTenant(ctx, sqlc.UpdateTenantParams{
		ID:       t.ID,
		Name:     t.Name,
		Domain:   pgTextOrNull(t.Domain),
		Branding: jsonOrEmpty(t.Branding),
		Settings: jsonOrEmpty(t.Settings),
		Status:   sqlc.TenantStatus(t.Status),
		Plan:     t.Plan,
	})
	if err != nil {
		if isNotFound(err) {
			return tenant.Tenant{}, tenant.ErrNotFound
		}
		return tenant.Tenant{}, fmt.Errorf("update tenant: %w", err)
	}
	return toDomainTenant(m), nil
}
