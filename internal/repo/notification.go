package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type notificationRepo struct{ q *sqlc.Queries }

func (r *notificationRepo) Enqueue(ctx context.Context, job notification.Job) (bool, error) {
	payload, _ := json.Marshal(job.Vars)
	n, err := r.q.EnqueueNotification(ctx, sqlc.EnqueueNotificationParams{
		TenantID:    job.TenantID,
		Channel:     sqlc.NotifChannel(job.Channel),
		ToAddr:      job.To,
		TemplateKey: job.TemplateKey,
		Payload:     payload,
		DedupKey:    job.DedupKey,
	})
	if err != nil {
		return false, fmt.Errorf("enqueue notification: %w", err)
	}
	return n > 0, nil // false => duplicate (idempotent no-op)
}

func (r *notificationRepo) ClaimDue(ctx context.Context, limit int32, leaseUntil time.Time) ([]notification.Log, error) {
	rows, err := r.q.ClaimDueNotifications(ctx, sqlc.ClaimDueNotificationsParams{Limit: limit, NextAttemptAt: leaseUntil})
	if err != nil {
		return nil, fmt.Errorf("claim due notifications: %w", err)
	}
	out := make([]notification.Log, len(rows))
	for i, m := range rows {
		vars := map[string]string{}
		_ = json.Unmarshal(m.Payload, &vars)
		out[i] = notification.Log{
			ID: m.ID, TenantID: m.TenantID, Channel: notification.Channel(m.Channel),
			To: m.ToAddr, TemplateKey: m.TemplateKey, Vars: vars, Attempts: m.Attempts,
		}
	}
	return out, nil
}

func (r *notificationRepo) MarkSent(ctx context.Context, id int64, providerRef string) error {
	if err := r.q.MarkNotificationSent(ctx, sqlc.MarkNotificationSentParams{ID: id, ProviderRef: providerRef}); err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (r *notificationRepo) Retry(ctx context.Context, id int64, errMsg string, nextAttempt time.Time) error {
	if err := r.q.RetryNotification(ctx, sqlc.RetryNotificationParams{ID: id, Error: errMsg, NextAttemptAt: nextAttempt}); err != nil {
		return fmt.Errorf("retry notification: %w", err)
	}
	return nil
}

func (r *notificationRepo) Fail(ctx context.Context, id int64, errMsg string) error {
	if err := r.q.FailNotification(ctx, sqlc.FailNotificationParams{ID: id, Error: errMsg}); err != nil {
		return fmt.Errorf("fail notification: %w", err)
	}
	return nil
}

func (r *notificationRepo) ActiveGateway(ctx context.Context, tenantID int64) (notification.Gateway, error) {
	m, err := r.q.GetActiveGateway(ctx, tenantID)
	if err != nil {
		if isNotFound(err) {
			return notification.Gateway{}, notification.ErrNoGateway
		}
		return notification.Gateway{}, fmt.Errorf("get active gateway: %w", err)
	}
	config := map[string]string{}
	_ = json.Unmarshal(m.Config, &config)
	return notification.Gateway{
		ID: m.ID, TenantID: m.TenantID, Provider: notification.Provider(m.Provider),
		Config: config, IsActive: m.IsActive, DailyLimit: int(m.DailyLimit),
	}, nil
}

func (r *notificationRepo) Template(ctx context.Context, tenantID int64, key string, channel notification.Channel) (notification.Template, error) {
	m, err := r.q.GetTemplate(ctx, sqlc.GetTemplateParams{TenantID: tenantID, Key: key, Channel: sqlc.NotifChannel(channel)})
	if err != nil {
		if isNotFound(err) {
			return notification.Template{}, notification.ErrNoTemplate
		}
		return notification.Template{}, fmt.Errorf("get template: %w", err)
	}
	return notification.Template{
		ID: m.ID, TenantID: m.TenantID, Key: m.Key, Channel: notification.Channel(m.Channel),
		Body: m.Body, IsActive: m.IsActive,
	}, nil
}

func (r *notificationRepo) UpsertTemplate(ctx context.Context, t notification.Template) (notification.Template, error) {
	m, err := r.q.UpsertTemplate(ctx, sqlc.UpsertTemplateParams{
		TenantID: t.TenantID, Key: t.Key, Channel: sqlc.NotifChannel(t.Channel), Body: t.Body, IsActive: t.IsActive,
	})
	if err != nil {
		return notification.Template{}, fmt.Errorf("upsert template: %w", err)
	}
	return notification.Template{
		ID: m.ID, TenantID: m.TenantID, Key: m.Key, Channel: notification.Channel(m.Channel),
		Body: m.Body, IsActive: m.IsActive,
	}, nil
}

func (r *notificationRepo) CreateGateway(ctx context.Context, g notification.Gateway) (notification.Gateway, error) {
	config, _ := json.Marshal(g.Config)
	m, err := r.q.UpsertGateway(ctx, sqlc.UpsertGatewayParams{
		TenantID: g.TenantID, Provider: sqlc.WaProvider(g.Provider), Config: config,
		IsActive: g.IsActive, DailyLimit: int32(g.DailyLimit),
	})
	if err != nil {
		return notification.Gateway{}, fmt.Errorf("create gateway: %w", err)
	}
	g.ID = m.ID
	return g, nil
}
