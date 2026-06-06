package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type radiusAuthRepo struct{ q *sqlc.Queries }

func (r *radiusAuthRepo) NasByIP(ctx context.Context, ip string) (radius.Nas, error) {
	m, err := r.q.GetNasByIP(ctx, ip)
	if err != nil {
		if isNotFound(err) {
			return radius.Nas{}, radius.ErrNasNotFound
		}
		return radius.Nas{}, fmt.Errorf("get nas by ip: %w", err)
	}
	return radius.Nas{ID: m.ID, TenantID: m.TenantID, Name: m.Nasname, Shortname: m.Shortname, Secret: m.Secret}, nil
}

func (r *radiusAuthRepo) CreateNas(ctx context.Context, n radius.Nas) (radius.Nas, error) {
	m, err := r.q.CreateNas(ctx, sqlc.CreateNasParams{
		TenantID:  n.TenantID,
		Nasname:   n.Name,
		Shortname: n.Shortname,
		Type:      "other",
		Secret:    n.Secret,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return radius.Nas{}, radius.ErrNasExists
		}
		return radius.Nas{}, fmt.Errorf("create nas: %w", err)
	}
	return radius.Nas{ID: m.ID, TenantID: m.TenantID, Name: m.Nasname, Shortname: m.Shortname, Secret: m.Secret}, nil
}

func (r *radiusAuthRepo) UserCheck(ctx context.Context, tenantID int64, username string) ([]radius.Attr, error) {
	rows, err := r.q.ListRadCheckUser(ctx, sqlc.ListRadCheckUserParams{TenantID: tenantID, Username: username})
	if err != nil {
		return nil, fmt.Errorf("list radcheck: %w", err)
	}
	attrs := make([]radius.Attr, len(rows))
	for i, m := range rows {
		attrs[i] = radius.Attr{Attribute: m.Attribute, Op: m.Op, Value: m.Value}
	}
	return attrs, nil
}

func (r *radiusAuthRepo) UserReply(ctx context.Context, tenantID int64, username string) ([]radius.Attr, error) {
	rows, err := r.q.ListRadReplyUser(ctx, sqlc.ListRadReplyUserParams{TenantID: tenantID, Username: username})
	if err != nil {
		return nil, fmt.Errorf("list radreply: %w", err)
	}
	attrs := make([]radius.Attr, len(rows))
	for i, m := range rows {
		attrs[i] = radius.Attr{Attribute: m.Attribute, Op: m.Op, Value: m.Value}
	}
	return attrs, nil
}

func (r *radiusAuthRepo) GroupReply(ctx context.Context, tenantID int64, groupname string) ([]radius.Attr, error) {
	rows, err := r.q.ListRadGroupReplyForGroup(ctx, sqlc.ListRadGroupReplyForGroupParams{TenantID: tenantID, Groupname: groupname})
	if err != nil {
		return nil, fmt.Errorf("list radgroupreply: %w", err)
	}
	attrs := make([]radius.Attr, len(rows))
	for i, m := range rows {
		attrs[i] = radius.Attr{Attribute: m.Attribute, Op: m.Op, Value: m.Value}
	}
	return attrs, nil
}

func (r *radiusAuthRepo) InsertPostAuth(ctx context.Context, pa radius.PostAuth) error {
	if err := r.q.InsertRadPostAuth(ctx, sqlc.InsertRadPostAuthParams{
		TenantID:     pa.TenantID,
		Username:     pa.Username,
		Pass:         pa.Pass,
		Reply:        pa.Reply,
		Nasipaddress: pa.NASIP,
	}); err != nil {
		return fmt.Errorf("insert radpostauth: %w", err)
	}
	return nil
}

func (r *radiusAuthRepo) ListNas(ctx context.Context, tenantID int64) ([]radius.Nas, error) {
	rows, err := r.q.ListNasByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list nas: %w", err)
	}
	out := make([]radius.Nas, len(rows))
	for i, m := range rows {
		out[i] = radius.Nas{ID: m.ID, TenantID: m.TenantID, Name: m.Nasname, Shortname: m.Shortname, Secret: m.Secret}
	}
	return out, nil
}
