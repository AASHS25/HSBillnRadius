package radiussvc

import (
	"context"
	"log/slog"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// acctOpTimeout bounds each accounting write.
const acctOpTimeout = 5 * time.Second

// AcctWriter consumes accounting events off a channel and applies them to the
// database in a single background goroutine, decoupling the radius request path
// from DB latency. The buffered channel provides backpressure; on overflow the
// caller drops the event (accounting is best-effort under extreme load).
type AcctWriter struct {
	repos repo.Repositories
	ch    chan radius.AcctEvent
	log   *slog.Logger
}

// NewAcctWriter builds an AcctWriter with the given queue capacity.
func NewAcctWriter(repos repo.Repositories, bufSize int, log *slog.Logger) *AcctWriter {
	if bufSize <= 0 {
		bufSize = 1024
	}
	return &AcctWriter{repos: repos, ch: make(chan radius.AcctEvent, bufSize), log: log}
}

// Submit enqueues an event without blocking; it reports false if the queue is
// full so the caller can record the drop.
func (w *AcctWriter) Submit(e radius.AcctEvent) bool {
	select {
	case w.ch <- e:
		return true
	default:
		return false
	}
}

// Run consumes events until ctx is cancelled, then drains the queue.
func (w *AcctWriter) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			w.drain()
			return nil
		case e := <-w.ch:
			w.apply(ctx, e)
		}
	}
}

// drain flushes any buffered events on shutdown using a fresh context.
func (w *AcctWriter) drain() {
	for {
		select {
		case e := <-w.ch:
			w.apply(context.Background(), e)
		default:
			return
		}
	}
}

func (w *AcctWriter) apply(ctx context.Context, e radius.AcctEvent) {
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), acctOpTimeout)
	defer cancel()

	var err error
	switch e.Status {
	case radius.AcctStart:
		err = w.repos.Accounting.Start(opCtx, e)
	case radius.AcctInterim:
		err = w.repos.Accounting.Interim(opCtx, e)
	case radius.AcctStop:
		err = w.repos.Accounting.Stop(opCtx, e)
	default:
		w.log.WarnContext(ctx, "accounting: unknown status", slog.String("status", string(e.Status)))
		return
	}
	if err != nil {
		w.log.WarnContext(ctx, "accounting write failed",
			slog.Any("error", err), slog.String("status", string(e.Status)), slog.String("session", e.SessionID))
	}
}
