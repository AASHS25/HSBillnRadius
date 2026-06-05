package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type userRepo struct{ q *sqlc.Queries }

func (r *userRepo) Create(ctx context.Context, u iam.User) (iam.User, error) {
	m, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		TenantID:     u.TenantID,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		RoleID:       u.RoleID,
		ParentID:     pgInt8Ptr(u.ParentID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return iam.User{}, iam.ErrEmailTaken
		}
		return iam.User{}, fmt.Errorf("create user: %w", err)
	}
	return toDomainUser(m), nil
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (iam.User, error) {
	m, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return iam.User{}, iam.ErrUserNotFound
		}
		return iam.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return toDomainUser(m), nil
}

func (r *userRepo) GetByEmail(ctx context.Context, tenantID int64, email string) (iam.User, error) {
	m, err := r.q.GetUserByEmail(ctx, sqlc.GetUserByEmailParams{TenantID: tenantID, Email: email})
	if err != nil {
		if isNotFound(err) {
			return iam.User{}, iam.ErrUserNotFound
		}
		return iam.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return toDomainUser(m), nil
}

func (r *userRepo) List(ctx context.Context, tenantID int64, limit, offset int32) ([]iam.User, error) {
	rows, err := r.q.ListUsersByTenant(ctx, sqlc.ListUsersByTenantParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := make([]iam.User, len(rows))
	for i, m := range rows {
		users[i] = toDomainUser(m)
	}
	return users, nil
}

func (r *userRepo) Count(ctx context.Context, tenantID int64) (int64, error) {
	n, err := r.q.CountUsersByTenant(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

func (r *userRepo) Update(ctx context.Context, u iam.User) (iam.User, error) {
	m, err := r.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:       u.ID,
		TenantID: u.TenantID,
		Name:     u.Name,
		RoleID:   u.RoleID,
		IsActive: u.IsActive,
	})
	if err != nil {
		if isNotFound(err) {
			return iam.User{}, iam.ErrUserNotFound
		}
		return iam.User{}, fmt.Errorf("update user: %w", err)
	}
	return toDomainUser(m), nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	if err := r.q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: id, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	return nil
}

func (r *userRepo) UpdateLastLogin(ctx context.Context, id int64) error {
	if err := r.q.UpdateUserLastLogin(ctx, id); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

func (r *userRepo) SoftDelete(ctx context.Context, tenantID, id int64) error {
	if err := r.q.SoftDeleteUser(ctx, sqlc.SoftDeleteUserParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	return nil
}
