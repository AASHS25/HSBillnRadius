package notifysvc_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/ports/wa"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/notifysvc"
)

type fakeClient struct {
	mu        sync.Mutex
	sent      []wa.Message
	failTimes int
}

func (f *fakeClient) Send(_ context.Context, msg wa.Message) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failTimes > 0 {
		f.failTimes--
		return "", errors.New("transient")
	}
	f.sent = append(f.sent, msg)
	return "ref-1", nil
}

func newSvc(t *testing.T, fc *fakeClient) (*notifysvc.Service, *memrepo.Store) {
	t.Helper()
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()
	_, err := r.Notification.CreateGateway(ctx, notification.Gateway{
		TenantID: 1, Provider: notification.ProviderFonnte,
		Config: map[string]string{"token": "t"}, IsActive: true,
	})
	require.NoError(t, err)
	_, err = r.Notification.UpsertTemplate(ctx, notification.Template{
		TenantID: 1, Key: "paid", Channel: notification.ChannelWA,
		Body: "Hi {nama}, paid {tagihan}", IsActive: true,
	})
	require.NoError(t, err)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := notifysvc.New(r, map[notification.Provider]wa.Client{notification.ProviderFonnte: fc}, 3, log)
	return svc, store
}

func enqueuePaid(t *testing.T, svc *notifysvc.Service, dedup string) {
	t.Helper()
	require.NoError(t, svc.Enqueue(context.Background(), notification.Job{
		TenantID: 1, To: "628", TemplateKey: "paid",
		Vars: map[string]string{"nama": "Budi", "tagihan": "100"}, DedupKey: dedup,
	}))
}

func TestProcessDue_RendersAndSends(t *testing.T) {
	fc := &fakeClient{}
	svc, store := newSvc(t, fc)
	enqueuePaid(t, svc, "k1")

	n, err := svc.ProcessDue(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, fc.sent, 1)
	assert.Equal(t, "Hi Budi, paid 100", fc.sent[0].Body)
	assert.Equal(t, notification.StatusSent, store.NotifStatus(1))
}

func TestEnqueue_Idempotent(t *testing.T) {
	fc := &fakeClient{}
	svc, _ := newSvc(t, fc)
	enqueuePaid(t, svc, "dup")
	enqueuePaid(t, svc, "dup")

	n, err := svc.ProcessDue(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "duplicate dedup key must enqueue once")
}

func TestProcessDue_RetriesThenSends(t *testing.T) {
	fc := &fakeClient{failTimes: 1}
	svc, store := newSvc(t, fc)
	enqueuePaid(t, svc, "k2")

	_, err := svc.ProcessDue(context.Background(), 10) // first attempt fails -> requeued
	require.NoError(t, err)
	assert.Equal(t, notification.StatusQueued, store.NotifStatus(1))

	_, err = svc.ProcessDue(context.Background(), 10) // second attempt succeeds
	require.NoError(t, err)
	assert.Equal(t, notification.StatusSent, store.NotifStatus(1))
}

func TestProcessDue_NoGatewayFailsPermanently(t *testing.T) {
	store := memrepo.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := notifysvc.New(store.Repositories(), map[notification.Provider]wa.Client{}, 3, log)
	require.NoError(t, svc.Enqueue(context.Background(), notification.Job{
		TenantID: 1, To: "628", TemplateKey: "paid", DedupKey: "k3",
	}))

	_, err := svc.ProcessDue(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, notification.StatusFailed, store.NotifStatus(1))
}
