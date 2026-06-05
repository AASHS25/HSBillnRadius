// Package customersvc implements subscriber use-cases. Creating or updating a
// PPPoE customer provisions the matching radcheck (password) and radusergroup
// (plan or isolir group) so the radius-service can authenticate them.
package customersvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Service provides customer management with RADIUS provisioning.
type Service struct {
	repos  repo.Repositories
	tx     repo.TxManager
	hasher Hasher
	log    *slog.Logger
}

// New builds a customer Service.
func New(repos repo.Repositories, tx repo.TxManager, log *slog.Logger) *Service {
	return &Service{repos: repos, tx: tx, log: log}
}

// Input is the create/update payload for a customer.
type Input struct {
	CustomerNo    string
	Name          string
	IDCardNo      string
	Email         string
	PhoneWA       string
	Address       string
	Lat           *float64
	Lng           *float64
	InstallDate   *time.Time
	Status        customer.Status
	PlanID        *int64
	ResellerID    *int64
	PppoeUsername string
	PppoePassword string
	Notes         string
}

// Create stores a customer and provisions RADIUS when applicable.
func (s *Service) Create(ctx context.Context, tenantID, actorID int64, in Input) (customer.Customer, error) {
	c := in.toCustomer(tenantID, 0)
	if c.Status == "" {
		c.Status = customer.StatusNew
	}
	if err := c.Validate(); err != nil {
		return customer.Customer{}, err
	}

	var out customer.Customer
	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		created, err := r.Customer.Create(ctx, c)
		if err != nil {
			return err
		}
		if err := s.provisionRadius(ctx, r, tenantID, created, ""); err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "customer.create", created.ID)
		out = created
		return nil
	})
	return out, err
}

// Update modifies a customer and re-provisions RADIUS, clearing the old
// username's entries when the username changes.
func (s *Service) Update(ctx context.Context, tenantID, actorID, id int64, in Input) (customer.Customer, error) {
	var out customer.Customer
	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		existing, err := r.Customer.GetByID(ctx, tenantID, id)
		if err != nil {
			return err
		}

		c := in.toCustomer(tenantID, id)
		if c.Status == "" {
			c.Status = existing.Status
		}
		if err := c.Validate(); err != nil {
			return err
		}

		updated, err := r.Customer.Update(ctx, c)
		if err != nil {
			return err
		}
		if err := s.provisionRadius(ctx, r, tenantID, updated, existing.PppoeUsername); err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "customer.update", updated.ID)
		out = updated
		return nil
	})
	return out, err
}

// Get returns a single customer.
func (s *Service) Get(ctx context.Context, tenantID, id int64) (customer.Customer, error) {
	return s.repos.Customer.GetByID(ctx, tenantID, id)
}

// Locations returns customers that have coordinates, for the map view.
func (s *Service) Locations(ctx context.Context, tenantID int64) ([]customer.Location, error) {
	return s.repos.Customer.ListWithLocation(ctx, tenantID)
}

// List returns a page of customers with the total count.
func (s *Service) List(ctx context.Context, tenantID int64, limit, offset int32) ([]customer.Customer, int64, error) {
	customers, err := s.repos.Customer.List(ctx, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repos.Customer.Count(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	return customers, total, nil
}

// Delete soft-deletes a customer and clears its RADIUS entries.
func (s *Service) Delete(ctx context.Context, tenantID, actorID, id int64) error {
	return s.tx.WithTx(ctx, func(r repo.Repositories) error {
		c, err := r.Customer.GetByID(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if err := r.Customer.SoftDelete(ctx, tenantID, id); err != nil {
			return err
		}
		if c.PppoeUsername != "" {
			if err := r.RadiusMap.ClearUser(ctx, tenantID, c.PppoeUsername); err != nil {
				return err
			}
		}
		s.audit(ctx, r, tenantID, actorID, "customer.delete", id)
		return nil
	})
}

// provisionRadius reconciles a customer's RADIUS entries. oldUsername (if set
// and changed) is cleared first to avoid orphaned rows.
func (s *Service) provisionRadius(ctx context.Context, r repo.Repositories, tenantID int64, c customer.Customer, oldUsername string) error {
	if oldUsername != "" && oldUsername != c.PppoeUsername {
		if err := r.RadiusMap.ClearUser(ctx, tenantID, oldUsername); err != nil {
			return err
		}
	}
	if c.PppoeUsername == "" {
		return nil
	}

	// No plan, missing credentials → ensure no stale auth remains.
	if c.PlanID == nil || !c.HasPPPoECredentials() {
		return r.RadiusMap.ClearUser(ctx, tenantID, c.PppoeUsername)
	}

	p, err := r.Plan.GetByID(ctx, tenantID, *c.PlanID)
	if err != nil {
		if errors.Is(err, plan.ErrNotFound) {
			return fmt.Errorf("%w: plan %d", customer.ErrPlanRequired, *c.PlanID)
		}
		return err
	}
	if p.ServiceType != plan.ServicePPPoE {
		return r.RadiusMap.ClearUser(ctx, tenantID, c.PppoeUsername)
	}

	if err := r.RadiusMap.SetUserPassword(ctx, tenantID, c.PppoeUsername, c.PppoePassword); err != nil {
		return err
	}
	return r.RadiusMap.SetUserGroup(ctx, tenantID, c.PppoeUsername, groupFor(p, c), 1)
}

// groupFor picks the RADIUS group for a customer: the isolir group when the
// customer is isolated and the plan defines one, otherwise the plan group.
func groupFor(p plan.Plan, c customer.Customer) string {
	if c.Status == customer.StatusIsolated && p.IsolirPlanID != nil {
		return fmt.Sprintf("plan_%d", *p.IsolirPlanID)
	}
	return p.GroupName()
}

func (in Input) toCustomer(tenantID, id int64) customer.Customer {
	return customer.Customer{
		ID:            id,
		TenantID:      tenantID,
		CustomerNo:    in.CustomerNo,
		Name:          in.Name,
		IDCardNo:      in.IDCardNo,
		Email:         in.Email,
		PhoneWA:       in.PhoneWA,
		Address:       in.Address,
		Lat:           in.Lat,
		Lng:           in.Lng,
		InstallDate:   in.InstallDate,
		Status:        in.Status,
		PlanID:        in.PlanID,
		ResellerID:    in.ResellerID,
		PppoeUsername: in.PppoeUsername,
		PppoePassword: in.PppoePassword,
		Notes:         in.Notes,
	}
}

func (s *Service) audit(ctx context.Context, r repo.Repositories, tenantID, actorID int64, action string, entityID int64) {
	actor := &actorID
	if actorID == 0 {
		actor = nil
	}
	if _, err := r.Audit.Insert(ctx, audit.Entry{
		TenantID:    tenantID,
		ActorUserID: actor,
		Action:      action,
		Entity:      "customer",
		EntityID:    strconv.FormatInt(entityID, 10),
	}); err != nil {
		s.log.WarnContext(ctx, "audit insert failed", slog.Any("error", err), slog.String("action", action))
	}
}

// Hasher hashes and verifies portal passwords.
type Hasher interface {
	Hash(plain string) (string, error)
	Verify(plain, encoded string) (bool, error)
}

// WithHasher enables client-area (portal) authentication.
func (s *Service) WithHasher(h Hasher) *Service {
	s.hasher = h
	return s
}

// PortalLogin verifies a customer's client-area credentials.
func (s *Service) PortalLogin(ctx context.Context, tenantSlug, customerNo, password string) (customer.Customer, error) {
	if s.hasher == nil {
		return customer.Customer{}, customer.ErrPortalInvalid
	}
	t, err := s.repos.Tenant.GetBySlug(ctx, tenantSlug)
	if err != nil {
		return customer.Customer{}, customer.ErrPortalInvalid
	}
	c, err := s.repos.Customer.GetByNo(ctx, t.ID, customerNo)
	if err != nil {
		return customer.Customer{}, customer.ErrPortalInvalid
	}
	if c.PortalPasswordHash == "" {
		return customer.Customer{}, customer.ErrPortalInvalid
	}
	ok, err := s.hasher.Verify(password, c.PortalPasswordHash)
	if err != nil || !ok {
		return customer.Customer{}, customer.ErrPortalInvalid
	}
	return c, nil
}

// SetPortalPassword sets a customer's client-area password (admin action).
func (s *Service) SetPortalPassword(ctx context.Context, tenantID, customerID int64, password string) error {
	if s.hasher == nil {
		return customer.ErrPortalInvalid
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	return s.repos.Customer.SetPortalPassword(ctx, tenantID, customerID, hash)
}
