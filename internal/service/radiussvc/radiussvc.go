// Package radiussvc implements the RADIUS authentication use-cases: resolving a
// NAS to its tenant/secret, looking up a user's credentials and reply
// attributes, and recording post-auth audit rows. Hot lookups are cached; a
// cache miss or outage degrades to a direct DB read.
package radiussvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// AuthInfo is the cached result of a user lookup.
type AuthInfo struct {
	Found         bool          `json:"found"`
	ClearPassword string        `json:"clear_password"`
	Reply         []radius.Attr `json:"reply"`
}

// Service provides RADIUS auth lookups.
type Service struct {
	repos   repo.Repositories
	cache   cache.Cache
	nasTTL  time.Duration
	userTTL time.Duration
	log     *slog.Logger
}

// New builds a radius Service.
func New(repos repo.Repositories, c cache.Cache, nasTTL, userTTL time.Duration, log *slog.Logger) *Service {
	return &Service{repos: repos, cache: c, nasTTL: nasTTL, userTTL: userTTL, log: log}
}

// ResolveNAS returns the NAS (tenant + secret) for a source IP, cached.
func (s *Service) ResolveNAS(ctx context.Context, ip string) (radius.Nas, error) {
	key := "rad:nas:" + ip
	if raw, ok := s.cache.Get(ctx, key); ok {
		var n radius.Nas
		if json.Unmarshal(raw, &n) == nil {
			return n, nil
		}
	}

	n, err := s.repos.RadiusAuth.NasByIP(ctx, ip)
	if err != nil {
		return radius.Nas{}, err
	}
	if raw, mErr := json.Marshal(n); mErr == nil {
		s.cache.Set(ctx, key, raw, s.nasTTL)
	}
	return n, nil
}

// Lookup returns a user's password and merged reply attributes, cached.
func (s *Service) Lookup(ctx context.Context, tenantID int64, username string) (AuthInfo, error) {
	key := userKey(tenantID, username)
	if raw, ok := s.cache.Get(ctx, key); ok {
		var info AuthInfo
		if json.Unmarshal(raw, &info) == nil {
			return info, nil
		}
	}

	info, err := s.load(ctx, tenantID, username)
	if err != nil {
		return AuthInfo{}, err
	}
	if raw, mErr := json.Marshal(info); mErr == nil {
		s.cache.Set(ctx, key, raw, s.userTTL)
	}
	return info, nil
}

// load builds AuthInfo directly from the database.
func (s *Service) load(ctx context.Context, tenantID int64, username string) (AuthInfo, error) {
	checks, err := s.repos.RadiusAuth.UserCheck(ctx, tenantID, username)
	if err != nil {
		return AuthInfo{}, err
	}

	var password string
	var found bool
	for _, a := range checks {
		if a.Attribute == radius.AttrCleartextPassword {
			password, found = a.Value, true
		}
	}
	if !found {
		return AuthInfo{Found: false}, nil
	}

	groups, err := s.repos.RadiusMap.UserGroups(ctx, tenantID, username)
	if err != nil {
		return AuthInfo{}, err
	}

	var reply []radius.Attr
	for _, g := range groups {
		gr, err := s.repos.RadiusAuth.GroupReply(ctx, tenantID, g.Groupname)
		if err != nil {
			return AuthInfo{}, err
		}
		reply = append(reply, gr...)
	}
	userReply, err := s.repos.RadiusAuth.UserReply(ctx, tenantID, username)
	if err != nil {
		return AuthInfo{}, err
	}
	reply = append(reply, userReply...) // per-user attributes win (appended last)

	return AuthInfo{Found: true, ClearPassword: password, Reply: reply}, nil
}

// RecordPostAuth writes an audit row. Failures are logged, not returned, so a
// post-auth write never blocks the reply path.
func (s *Service) RecordPostAuth(ctx context.Context, pa radius.PostAuth) {
	if err := s.repos.RadiusAuth.InsertPostAuth(ctx, pa); err != nil {
		s.log.WarnContext(ctx, "radpostauth insert failed", slog.Any("error", err))
	}
}

// Invalidate drops a cached user lookup (called when a customer's status or
// plan changes so the next auth reflects it).
func (s *Service) Invalidate(ctx context.Context, tenantID int64, username string) {
	s.cache.Del(ctx, userKey(tenantID, username))
}

func userKey(tenantID int64, username string) string {
	return fmt.Sprintf("rad:user:%d:%s", tenantID, username)
}

// IsNasUnknown reports whether err means the source NAS is not registered.
func IsNasUnknown(err error) bool { return errors.Is(err, radius.ErrNasNotFound) }
