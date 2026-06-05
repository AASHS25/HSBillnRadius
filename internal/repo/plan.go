package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type planRepo struct{ q *sqlc.Queries }

func (r *planRepo) Create(ctx context.Context, p plan.Plan) (plan.Plan, error) {
	m, err := r.q.CreatePlan(ctx, sqlc.CreatePlanParams{
		TenantID:     p.TenantID,
		Name:         p.Name,
		ServiceType:  sqlc.ServiceType(p.ServiceType),
		PriceIdr:     p.PriceIDR,
		TaxBps:       p.TaxBps,
		BillingCycle: sqlc.BillingCycle(p.BillingCycle),
		ActiveDays:   p.ActiveDays,
		DataQuotaMb:  pgInt8Ptr(p.DataQuotaMB),
		TimeQuotaSec: pgInt8Ptr(p.TimeQuotaSec),
		IsUnlimited:  p.IsUnlimited,
		PoolName:     p.PoolName,
		IsolirPlanID: pgInt8Ptr(p.IsolirPlanID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return plan.Plan{}, plan.ErrNameTaken
		}
		return plan.Plan{}, fmt.Errorf("create plan: %w", err)
	}
	return toDomainPlan(m), nil
}

func (r *planRepo) GetByID(ctx context.Context, tenantID, id int64) (plan.Plan, error) {
	m, err := r.q.GetPlanByID(ctx, sqlc.GetPlanByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		if isNotFound(err) {
			return plan.Plan{}, plan.ErrNotFound
		}
		return plan.Plan{}, fmt.Errorf("get plan: %w", err)
	}
	return toDomainPlan(m), nil
}

func (r *planRepo) List(ctx context.Context, tenantID int64, limit, offset int32) ([]plan.Plan, error) {
	rows, err := r.q.ListPlansByTenant(ctx, sqlc.ListPlansByTenantParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	plans := make([]plan.Plan, len(rows))
	for i, m := range rows {
		plans[i] = toDomainPlan(m)
	}
	return plans, nil
}

func (r *planRepo) Count(ctx context.Context, tenantID int64) (int64, error) {
	n, err := r.q.CountPlansByTenant(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("count plans: %w", err)
	}
	return n, nil
}

func (r *planRepo) Update(ctx context.Context, p plan.Plan) (plan.Plan, error) {
	m, err := r.q.UpdatePlan(ctx, sqlc.UpdatePlanParams{
		ID:           p.ID,
		TenantID:     p.TenantID,
		Name:         p.Name,
		ServiceType:  sqlc.ServiceType(p.ServiceType),
		PriceIdr:     p.PriceIDR,
		TaxBps:       p.TaxBps,
		BillingCycle: sqlc.BillingCycle(p.BillingCycle),
		ActiveDays:   p.ActiveDays,
		DataQuotaMb:  pgInt8Ptr(p.DataQuotaMB),
		TimeQuotaSec: pgInt8Ptr(p.TimeQuotaSec),
		IsUnlimited:  p.IsUnlimited,
		PoolName:     p.PoolName,
		IsolirPlanID: pgInt8Ptr(p.IsolirPlanID),
	})
	if err != nil {
		if isNotFound(err) {
			return plan.Plan{}, plan.ErrNotFound
		}
		if isUniqueViolation(err) {
			return plan.Plan{}, plan.ErrNameTaken
		}
		return plan.Plan{}, fmt.Errorf("update plan: %w", err)
	}
	return toDomainPlan(m), nil
}

func (r *planRepo) SoftDelete(ctx context.Context, tenantID, id int64) error {
	if err := r.q.SoftDeletePlan(ctx, sqlc.SoftDeletePlanParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("soft delete plan: %w", err)
	}
	return nil
}

// --- bandwidth profile ------------------------------------------------------

type bandwidthRepo struct{ q *sqlc.Queries }

func (r *bandwidthRepo) Upsert(ctx context.Context, b plan.BandwidthProfile) (plan.BandwidthProfile, error) {
	m, err := r.q.UpsertBandwidthProfile(ctx, sqlc.UpsertBandwidthProfileParams{
		TenantID:           b.TenantID,
		PlanID:             b.PlanID,
		RateLimitRx:        b.RateLimitRx,
		RateLimitTx:        b.RateLimitTx,
		BurstRx:            b.BurstRx,
		BurstTx:            b.BurstTx,
		BurstThresholdRx:   b.BurstThresholdRx,
		BurstThresholdTx:   b.BurstThresholdTx,
		BurstTime:          b.BurstTime,
		Priority:           b.Priority,
		MikrotikRateString: b.MikrotikRateString,
	})
	if err != nil {
		return plan.BandwidthProfile{}, fmt.Errorf("upsert bandwidth profile: %w", err)
	}
	return toDomainBandwidth(m), nil
}

func (r *bandwidthRepo) GetByPlan(ctx context.Context, planID int64) (plan.BandwidthProfile, error) {
	m, err := r.q.GetBandwidthProfileByPlan(ctx, planID)
	if err != nil {
		if isNotFound(err) {
			return plan.BandwidthProfile{}, plan.ErrNotFound
		}
		return plan.BandwidthProfile{}, fmt.Errorf("get bandwidth profile: %w", err)
	}
	return toDomainBandwidth(m), nil
}
