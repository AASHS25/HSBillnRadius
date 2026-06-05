// Package plansvc implements service-plan use-cases. Creating or updating a plan
// also syncs its RADIUS group reply attributes (Mikrotik-Rate-Limit, Framed-Pool)
// so the radius-service can answer with the right bandwidth.
package plansvc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Service provides plan management.
type Service struct {
	repos repo.Repositories
	tx    repo.TxManager
	log   *slog.Logger
}

// New builds a plan Service.
func New(repos repo.Repositories, tx repo.TxManager, log *slog.Logger) *Service {
	return &Service{repos: repos, tx: tx, log: log}
}

// BandwidthInput carries the optional bandwidth profile fields.
type BandwidthInput struct {
	RateLimitRx        string
	RateLimitTx        string
	BurstRx            string
	BurstTx            string
	BurstThresholdRx   string
	BurstThresholdTx   string
	BurstTime          string
	Priority           int32
	MikrotikRateString string
}

// Input is the create/update payload for a plan plus its bandwidth profile.
type Input struct {
	Name         string
	ServiceType  plan.ServiceType
	PriceIDR     int64
	TaxBps       int32
	BillingCycle plan.BillingCycle
	ActiveDays   int32
	DataQuotaMB  *int64
	TimeQuotaSec *int64
	IsUnlimited  bool
	PoolName     string
	IsolirPlanID *int64
	Bandwidth    BandwidthInput
}

// Result bundles a plan with its bandwidth profile.
type Result struct {
	Plan      plan.Plan
	Bandwidth plan.BandwidthProfile
}

// Create stores a new plan, its bandwidth profile and RADIUS group reply.
func (s *Service) Create(ctx context.Context, tenantID, actorID int64, in Input) (Result, error) {
	p := in.toPlan(tenantID, 0)
	if err := p.Validate(); err != nil {
		return Result{}, err
	}

	var out Result
	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		created, err := r.Plan.Create(ctx, p)
		if err != nil {
			return err
		}
		bp, err := s.syncBandwidthAndGroup(ctx, r, tenantID, created, in.Bandwidth)
		if err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "plan.create", created.ID)
		out = Result{Plan: created, Bandwidth: bp}
		return nil
	})
	return out, err
}

// Update modifies a plan, its bandwidth profile and RADIUS group reply.
func (s *Service) Update(ctx context.Context, tenantID, actorID, planID int64, in Input) (Result, error) {
	p := in.toPlan(tenantID, planID)
	if err := p.Validate(); err != nil {
		return Result{}, err
	}

	var out Result
	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		updated, err := r.Plan.Update(ctx, p)
		if err != nil {
			return err
		}
		bp, err := s.syncBandwidthAndGroup(ctx, r, tenantID, updated, in.Bandwidth)
		if err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "plan.update", updated.ID)
		out = Result{Plan: updated, Bandwidth: bp}
		return nil
	})
	return out, err
}

// Get returns a plan with its bandwidth profile.
func (s *Service) Get(ctx context.Context, tenantID, planID int64) (Result, error) {
	p, err := s.repos.Plan.GetByID(ctx, tenantID, planID)
	if err != nil {
		return Result{}, err
	}
	bp, err := s.repos.Bandwidth.GetByPlan(ctx, planID)
	if err != nil && !errors.Is(err, plan.ErrNotFound) {
		return Result{}, err
	}
	return Result{Plan: p, Bandwidth: bp}, nil
}

// List returns a page of plans with the total count.
func (s *Service) List(ctx context.Context, tenantID int64, limit, offset int32) ([]plan.Plan, int64, error) {
	plans, err := s.repos.Plan.List(ctx, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repos.Plan.Count(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	return plans, total, nil
}

// Delete soft-deletes a plan and clears its RADIUS group reply.
func (s *Service) Delete(ctx context.Context, tenantID, actorID, planID int64) error {
	return s.tx.WithTx(ctx, func(r repo.Repositories) error {
		p, err := r.Plan.GetByID(ctx, tenantID, planID)
		if err != nil {
			return err
		}
		if err := r.Plan.SoftDelete(ctx, tenantID, planID); err != nil {
			return err
		}
		if err := r.RadiusMap.SetGroupReply(ctx, tenantID, p.GroupName(), nil); err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "plan.delete", planID)
		return nil
	})
}

// syncBandwidthAndGroup upserts the bandwidth profile and rewrites the plan's
// RADIUS group reply attributes.
func (s *Service) syncBandwidthAndGroup(ctx context.Context, r repo.Repositories, tenantID int64, p plan.Plan, in BandwidthInput) (plan.BandwidthProfile, error) {
	bp := plan.BandwidthProfile{
		TenantID:         tenantID,
		PlanID:           p.ID,
		RateLimitRx:      in.RateLimitRx,
		RateLimitTx:      in.RateLimitTx,
		BurstRx:          in.BurstRx,
		BurstTx:          in.BurstTx,
		BurstThresholdRx: in.BurstThresholdRx,
		BurstThresholdTx: in.BurstThresholdTx,
		BurstTime:        in.BurstTime,
		Priority:         in.Priority,
	}
	// Persist the composed Mikrotik rate string so the radius-service can read
	// it directly without recomputation.
	bp.MikrotikRateString = bp.MikrotikRate()
	if in.MikrotikRateString != "" {
		bp.MikrotikRateString = in.MikrotikRateString
	}

	saved, err := r.Bandwidth.Upsert(ctx, bp)
	if err != nil {
		return plan.BandwidthProfile{}, err
	}

	var attrs []radius.Attr
	if rate := saved.MikrotikRate(); rate != "" {
		attrs = append(attrs, radius.Attr{Attribute: radius.AttrMikrotikRateLimit, Op: ":=", Value: rate})
	}
	if p.PoolName != "" {
		attrs = append(attrs, radius.Attr{Attribute: radius.AttrFramedPool, Op: ":=", Value: p.PoolName})
	}
	if err := r.RadiusMap.SetGroupReply(ctx, tenantID, p.GroupName(), attrs); err != nil {
		return plan.BandwidthProfile{}, err
	}
	return saved, nil
}

func (in Input) toPlan(tenantID, id int64) plan.Plan {
	return plan.Plan{
		ID:           id,
		TenantID:     tenantID,
		Name:         in.Name,
		ServiceType:  in.ServiceType,
		PriceIDR:     in.PriceIDR,
		TaxBps:       in.TaxBps,
		BillingCycle: in.BillingCycle,
		ActiveDays:   in.ActiveDays,
		DataQuotaMB:  in.DataQuotaMB,
		TimeQuotaSec: in.TimeQuotaSec,
		IsUnlimited:  in.IsUnlimited,
		PoolName:     in.PoolName,
		IsolirPlanID: in.IsolirPlanID,
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
		Entity:      "plan",
		EntityID:    strconv.FormatInt(entityID, 10),
	}); err != nil {
		s.log.WarnContext(ctx, "audit insert failed", slog.Any("error", err), slog.String("action", action))
	}
}
