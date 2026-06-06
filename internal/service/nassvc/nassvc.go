// Package nassvc manages RADIUS NAS (router) registrations. A NAS must be
// registered with its shared secret before the radius-service will accept
// packets from it.
package nassvc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Service provides NAS management.
type Service struct {
	repos repo.Repositories
	log   *slog.Logger
}

// New builds a NAS Service.
func New(repos repo.Repositories, log *slog.Logger) *Service {
	return &Service{repos: repos, log: log}
}

// Create registers a NAS (router) with its shared secret.
func (s *Service) Create(ctx context.Context, tenantID int64, nasname, shortname, secret string) (radius.Nas, error) {
	if strings.TrimSpace(nasname) == "" || strings.TrimSpace(secret) == "" {
		return radius.Nas{}, fmt.Errorf("%w: nasname and secret are required", radius.ErrNasExists)
	}
	return s.repos.RadiusAuth.CreateNas(ctx, radius.Nas{
		TenantID: tenantID, Name: nasname, Shortname: shortname, Secret: secret,
	})
}

// List returns the tenant's registered NAS devices.
func (s *Service) List(ctx context.Context, tenantID int64) ([]radius.Nas, error) {
	return s.repos.RadiusAuth.ListNas(ctx, tenantID)
}
