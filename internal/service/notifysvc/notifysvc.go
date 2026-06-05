// Package notifysvc orchestrates notification delivery: it enqueues jobs
// (idempotent via dedup key) and processes due jobs by rendering the tenant's
// template and sending via the tenant's WhatsApp provider, with backoff retry.
package notifysvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
	"github.com/aashs25/hsbillnradius/internal/ports/wa"
)

// Service delivers notifications.
type Service struct {
	repos       repo.Repositories
	clients     map[notification.Provider]wa.Client
	maxAttempts int32
	baseBackoff time.Duration
	leaseFor    time.Duration
	log         *slog.Logger
	now         func() time.Time
}

// New builds a notify Service. clients maps a provider to its WA client.
func New(repos repo.Repositories, clients map[notification.Provider]wa.Client, maxAttempts int32, log *slog.Logger) *Service {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &Service{
		repos:       repos,
		clients:     clients,
		maxAttempts: maxAttempts,
		baseBackoff: 30 * time.Second,
		leaseFor:    2 * time.Minute,
		log:         log,
		now:         time.Now,
	}
}

// Enqueue stores a notification job. A blank dedup key is filled with a
// time-unique value so the job always enqueues.
func (s *Service) Enqueue(ctx context.Context, job notification.Job) error {
	if job.Channel == "" {
		job.Channel = notification.ChannelWA
	}
	if job.DedupKey == "" {
		job.DedupKey = fmt.Sprintf("%d:%s:%s:%d", job.TenantID, job.TemplateKey, job.To, s.now().UnixNano())
	}
	if _, err := s.repos.Notification.Enqueue(ctx, job); err != nil {
		return err
	}
	return nil
}

// ConfigureGateway stores a tenant's WhatsApp gateway.
func (s *Service) ConfigureGateway(ctx context.Context, g notification.Gateway) (notification.Gateway, error) {
	return s.repos.Notification.CreateGateway(ctx, g)
}

// UpsertTemplate stores/updates a message template.
func (s *Service) UpsertTemplate(ctx context.Context, t notification.Template) (notification.Template, error) {
	return s.repos.Notification.UpsertTemplate(ctx, t)
}

// ProcessDue claims up to batch due jobs and attempts to deliver them, returning
// the number processed.
func (s *Service) ProcessDue(ctx context.Context, batch int32) (int, error) {
	logs, err := s.repos.Notification.ClaimDue(ctx, batch, s.now().Add(s.leaseFor))
	if err != nil {
		return 0, err
	}
	for i := range logs {
		s.process(ctx, logs[i])
	}
	return len(logs), nil
}

// process renders and sends a single claimed notification, then records the
// outcome (sent / retry with backoff / permanently failed).
func (s *Service) process(ctx context.Context, l notification.Log) {
	ref, err := s.deliver(ctx, l)
	if err == nil {
		if mErr := s.repos.Notification.MarkSent(ctx, l.ID, ref); mErr != nil {
			s.log.WarnContext(ctx, "mark sent failed", slog.Any("error", mErr))
		}
		return
	}

	// Permanent configuration errors should not be retried forever.
	permanent := errors.Is(err, notification.ErrNoGateway) ||
		errors.Is(err, notification.ErrNoTemplate) ||
		errors.Is(err, notification.ErrNoProvider)

	if permanent || l.Attempts >= s.maxAttempts {
		_ = s.repos.Notification.Fail(ctx, l.ID, err.Error())
		s.log.WarnContext(ctx, "notification failed permanently",
			slog.Int64("id", l.ID), slog.Any("error", err), slog.Int("attempts", int(l.Attempts)))
		return
	}

	next := s.now().Add(s.backoff(l.Attempts))
	_ = s.repos.Notification.Retry(ctx, l.ID, err.Error(), next)
}

// deliver renders the template and sends via the tenant's provider.
func (s *Service) deliver(ctx context.Context, l notification.Log) (string, error) {
	gateway, err := s.repos.Notification.ActiveGateway(ctx, l.TenantID)
	if err != nil {
		return "", err
	}
	tmpl, err := s.repos.Notification.Template(ctx, l.TenantID, l.TemplateKey, l.Channel)
	if err != nil {
		return "", err
	}
	client, ok := s.clients[gateway.Provider]
	if !ok {
		return "", fmt.Errorf("%w: %s", notification.ErrNoProvider, gateway.Provider)
	}

	body := notification.Render(tmpl.Body, l.Vars)
	return client.Send(ctx, wa.Message{To: l.To, Body: body, Config: gateway.Config})
}

// backoff returns an exponential delay for the given (1-based) attempt count.
func (s *Service) backoff(attempts int32) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	d := s.baseBackoff << (attempts - 1)
	if max := 30 * time.Minute; d > max {
		d = max
	}
	return d
}
