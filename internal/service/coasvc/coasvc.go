// Package coasvc implements auto-isolir and restore: it switches a user's RADIUS
// group, invalidates the auth cache, and kicks active sessions via CoA so they
// immediately reconnect into the new group.
package coasvc

import (
	"context"
	"log/slog"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
)

// CoADialer kicks a session on a NAS.
type CoADialer interface {
	Disconnect(ctx context.Context, nas domainradius.Nas, username, sessionID string) error
}

// Service performs isolir/restore.
type Service struct {
	repos repo.Repositories
	coa   CoADialer
	cache cache.Cache
	log   *slog.Logger
}

// New builds a coasvc Service.
func New(repos repo.Repositories, coa CoADialer, c cache.Cache, log *slog.Logger) *Service {
	return &Service{repos: repos, coa: coa, cache: c, log: log}
}

// Isolate moves a user into the isolir group and disconnects active sessions.
func (s *Service) Isolate(ctx context.Context, tenantID int64, username, isolirGroup string) error {
	return s.switchGroup(ctx, tenantID, username, isolirGroup)
}

// Restore moves a user back to their plan group and disconnects active sessions
// so they reconnect at full speed.
func (s *Service) Restore(ctx context.Context, tenantID int64, username, planGroup string) error {
	return s.switchGroup(ctx, tenantID, username, planGroup)
}

func (s *Service) switchGroup(ctx context.Context, tenantID int64, username, group string) error {
	if err := s.repos.RadiusMap.SetUserGroup(ctx, tenantID, username, group, 1); err != nil {
		return err
	}
	s.cache.Del(ctx, radiussvc.UserCacheKey(tenantID, username))
	s.disconnectSessions(ctx, tenantID, username)
	return nil
}

// disconnectSessions kicks every active session for the user (best effort).
func (s *Service) disconnectSessions(ctx context.Context, tenantID int64, username string) {
	sessions, err := s.repos.Accounting.ActiveSessions(ctx, tenantID, username)
	if err != nil {
		s.log.WarnContext(ctx, "coa: list active sessions failed", slog.Any("error", err))
		return
	}
	for _, sess := range sessions {
		nas, err := s.repos.RadiusAuth.NasByIP(ctx, sess.NASIP)
		if err != nil {
			s.log.WarnContext(ctx, "coa: nas lookup failed", slog.String("nas_ip", sess.NASIP), slog.Any("error", err))
			continue
		}
		if err := s.coa.Disconnect(ctx, nas, username, sess.SessionID); err != nil {
			s.log.WarnContext(ctx, "coa: disconnect failed",
				slog.String("nas_ip", sess.NASIP), slog.String("session", sess.SessionID), slog.Any("error", err))
		}
	}
}
