package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type roleRepo struct{ q *sqlc.Queries }

func (r *roleRepo) Create(ctx context.Context, role iam.Role) (iam.Role, error) {
	m, err := r.q.CreateRole(ctx, sqlc.CreateRoleParams{
		TenantID: role.TenantID,
		Name:     role.Name,
		IsSystem: role.IsSystem,
	})
	if err != nil {
		return iam.Role{}, fmt.Errorf("create role: %w", err)
	}
	return toDomainRole(m), nil
}

func (r *roleRepo) GetByID(ctx context.Context, id int64) (iam.Role, error) {
	m, err := r.q.GetRoleByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return iam.Role{}, iam.ErrRoleNotFound
		}
		return iam.Role{}, fmt.Errorf("get role by id: %w", err)
	}
	return toDomainRole(m), nil
}

func (r *roleRepo) GetByName(ctx context.Context, tenantID int64, name string) (iam.Role, error) {
	m, err := r.q.GetRoleByName(ctx, sqlc.GetRoleByNameParams{TenantID: tenantID, Name: name})
	if err != nil {
		if isNotFound(err) {
			return iam.Role{}, iam.ErrRoleNotFound
		}
		return iam.Role{}, fmt.Errorf("get role by name: %w", err)
	}
	return toDomainRole(m), nil
}

func (r *roleRepo) List(ctx context.Context, tenantID int64) ([]iam.Role, error) {
	rows, err := r.q.ListRolesByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	roles := make([]iam.Role, len(rows))
	for i, m := range rows {
		roles[i] = toDomainRole(m)
	}
	return roles, nil
}

func (r *roleRepo) GrantAllPermissions(ctx context.Context, roleID int64) error {
	if err := r.q.GrantAllPermissionsToRole(ctx, roleID); err != nil {
		return fmt.Errorf("grant all permissions: %w", err)
	}
	return nil
}

func (r *roleRepo) AddPermission(ctx context.Context, roleID, permissionID int64) error {
	if err := r.q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: roleID, PermissionID: permissionID}); err != nil {
		return fmt.Errorf("add role permission: %w", err)
	}
	return nil
}

func (r *roleRepo) PermissionCodes(ctx context.Context, roleID int64) ([]string, error) {
	codes, err := r.q.ListPermissionCodesByRole(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("list permission codes: %w", err)
	}
	return codes, nil
}
