package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type auditRepo struct{ q *sqlc.Queries }

func (r *auditRepo) Insert(ctx context.Context, e audit.Entry) (audit.Entry, error) {
	m, err := r.q.InsertAuditLog(ctx, sqlc.InsertAuditLogParams{
		TenantID:    e.TenantID,
		ActorUserID: pgInt8Ptr(e.ActorUserID),
		Action:      e.Action,
		Entity:      e.Entity,
		EntityID:    e.EntityID,
		Diff:        jsonOrEmpty(e.Diff),
		Ip:          pgTextOrNull(e.IP),
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("insert audit log: %w", err)
	}
	return toDomainAudit(m), nil
}

func (r *auditRepo) List(ctx context.Context, tenantID int64, limit, offset int32) ([]audit.Entry, error) {
	rows, err := r.q.ListAuditLogs(ctx, sqlc.ListAuditLogsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	entries := make([]audit.Entry, len(rows))
	for i, m := range rows {
		entries[i] = toDomainAudit(m)
	}
	return entries, nil
}
