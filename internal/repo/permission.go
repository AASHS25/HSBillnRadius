package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type permissionRepo struct{ q *sqlc.Queries }

func (r *permissionRepo) List(ctx context.Context) ([]iam.Permission, error) {
	rows, err := r.q.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	perms := make([]iam.Permission, len(rows))
	for i, m := range rows {
		perms[i] = toDomainPermission(m)
	}
	return perms, nil
}

func (r *permissionRepo) IDsByCodes(ctx context.Context, codes []string) ([]int64, error) {
	ids, err := r.q.ListPermissionIDsByCodes(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("list permission ids by codes: %w", err)
	}
	return ids, nil
}
