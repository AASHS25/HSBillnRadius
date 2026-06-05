package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type radiusMapRepo struct{ q *sqlc.Queries }

// SetUserPassword replaces the user's radcheck entries with a single
// Cleartext-Password attribute.
func (r *radiusMapRepo) SetUserPassword(ctx context.Context, tenantID int64, username, pw string) error {
	if err := r.q.DeleteRadCheckByUser(ctx, sqlc.DeleteRadCheckByUserParams{TenantID: tenantID, Username: username}); err != nil {
		return fmt.Errorf("clear radcheck: %w", err)
	}
	if err := r.q.InsertRadCheck(ctx, sqlc.InsertRadCheckParams{
		TenantID:  tenantID,
		Username:  username,
		Attribute: radius.AttrCleartextPassword,
		Op:        ":=",
		Value:     pw,
	}); err != nil {
		return fmt.Errorf("insert radcheck: %w", err)
	}
	return nil
}

// SetUserGroup replaces the user's group membership with a single group.
func (r *radiusMapRepo) SetUserGroup(ctx context.Context, tenantID int64, username, groupname string, priority int32) error {
	if err := r.q.DeleteRadUserGroupByUser(ctx, sqlc.DeleteRadUserGroupByUserParams{TenantID: tenantID, Username: username}); err != nil {
		return fmt.Errorf("clear radusergroup: %w", err)
	}
	if err := r.q.InsertRadUserGroup(ctx, sqlc.InsertRadUserGroupParams{
		TenantID:  tenantID,
		Username:  username,
		Groupname: groupname,
		Priority:  priority,
	}); err != nil {
		return fmt.Errorf("insert radusergroup: %w", err)
	}
	return nil
}

// ClearUser removes all radcheck, radreply and radusergroup rows for a user.
func (r *radiusMapRepo) ClearUser(ctx context.Context, tenantID int64, username string) error {
	if err := r.q.DeleteRadCheckByUser(ctx, sqlc.DeleteRadCheckByUserParams{TenantID: tenantID, Username: username}); err != nil {
		return fmt.Errorf("clear radcheck: %w", err)
	}
	if err := r.q.DeleteRadReplyByUser(ctx, sqlc.DeleteRadReplyByUserParams{TenantID: tenantID, Username: username}); err != nil {
		return fmt.Errorf("clear radreply: %w", err)
	}
	if err := r.q.DeleteRadUserGroupByUser(ctx, sqlc.DeleteRadUserGroupByUserParams{TenantID: tenantID, Username: username}); err != nil {
		return fmt.Errorf("clear radusergroup: %w", err)
	}
	return nil
}

// SetGroupReply replaces a group's reply attributes.
func (r *radiusMapRepo) SetGroupReply(ctx context.Context, tenantID int64, groupname string, attrs []radius.Attr) error {
	if err := r.q.DeleteRadGroupReplyByGroup(ctx, sqlc.DeleteRadGroupReplyByGroupParams{TenantID: tenantID, Groupname: groupname}); err != nil {
		return fmt.Errorf("clear radgroupreply: %w", err)
	}
	for _, a := range attrs {
		if err := r.q.InsertRadGroupReply(ctx, sqlc.InsertRadGroupReplyParams{
			TenantID:  tenantID,
			Groupname: groupname,
			Attribute: a.Attribute,
			Op:        a.Op,
			Value:     a.Value,
		}); err != nil {
			return fmt.Errorf("insert radgroupreply: %w", err)
		}
	}
	return nil
}

// UserGroups returns the user's group memberships ordered by priority.
func (r *radiusMapRepo) UserGroups(ctx context.Context, tenantID int64, username string) ([]radius.UserGroup, error) {
	rows, err := r.q.ListRadUserGroups(ctx, sqlc.ListRadUserGroupsParams{TenantID: tenantID, Username: username})
	if err != nil {
		return nil, fmt.Errorf("list radusergroups: %w", err)
	}
	out := make([]radius.UserGroup, len(rows))
	for i, m := range rows {
		out[i] = radius.UserGroup{Username: m.Username, Groupname: m.Groupname, Priority: m.Priority}
	}
	return out, nil
}
