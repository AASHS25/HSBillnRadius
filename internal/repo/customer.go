package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type customerRepo struct{ q *sqlc.Queries }

func (r *customerRepo) Create(ctx context.Context, c customer.Customer) (customer.Customer, error) {
	m, err := r.q.CreateCustomer(ctx, sqlc.CreateCustomerParams{
		TenantID:      c.TenantID,
		CustomerNo:    c.CustomerNo,
		Name:          c.Name,
		IDCardNo:      c.IDCardNo,
		Email:         c.Email,
		PhoneWa:       c.PhoneWA,
		Address:       c.Address,
		Lat:           pgFloat8Ptr(c.Lat),
		Lng:           pgFloat8Ptr(c.Lng),
		InstallDate:   pgDatePtr(c.InstallDate),
		Status:        sqlc.CustomerStatus(c.Status),
		PlanID:        pgInt8Ptr(c.PlanID),
		ResellerID:    pgInt8Ptr(c.ResellerID),
		BalanceIdr:    c.BalanceIDR,
		PppoeUsername: pgTextOrNull(c.PppoeUsername),
		PppoePassword: pgTextOrNull(c.PppoePassword),
		Notes:         c.Notes,
	})
	if err != nil {
		return customer.Customer{}, mapCustomerErr(err)
	}
	return toDomainCustomer(m), nil
}

func (r *customerRepo) GetByID(ctx context.Context, tenantID, id int64) (customer.Customer, error) {
	m, err := r.q.GetCustomerByID(ctx, sqlc.GetCustomerByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		if isNotFound(err) {
			return customer.Customer{}, customer.ErrNotFound
		}
		return customer.Customer{}, fmt.Errorf("get customer: %w", err)
	}
	return toDomainCustomer(m), nil
}

func (r *customerRepo) List(ctx context.Context, tenantID int64, limit, offset int32) ([]customer.Customer, error) {
	rows, err := r.q.ListCustomersByTenant(ctx, sqlc.ListCustomersByTenantParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list customers: %w", err)
	}
	customers := make([]customer.Customer, len(rows))
	for i, m := range rows {
		customers[i] = toDomainCustomer(m)
	}
	return customers, nil
}

func (r *customerRepo) Count(ctx context.Context, tenantID int64) (int64, error) {
	n, err := r.q.CountCustomersByTenant(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("count customers: %w", err)
	}
	return n, nil
}

func (r *customerRepo) Update(ctx context.Context, c customer.Customer) (customer.Customer, error) {
	m, err := r.q.UpdateCustomer(ctx, sqlc.UpdateCustomerParams{
		ID:            c.ID,
		TenantID:      c.TenantID,
		Name:          c.Name,
		IDCardNo:      c.IDCardNo,
		Email:         c.Email,
		PhoneWa:       c.PhoneWA,
		Address:       c.Address,
		Lat:           pgFloat8Ptr(c.Lat),
		Lng:           pgFloat8Ptr(c.Lng),
		InstallDate:   pgDatePtr(c.InstallDate),
		Status:        sqlc.CustomerStatus(c.Status),
		PlanID:        pgInt8Ptr(c.PlanID),
		ResellerID:    pgInt8Ptr(c.ResellerID),
		PppoeUsername: pgTextOrNull(c.PppoeUsername),
		PppoePassword: pgTextOrNull(c.PppoePassword),
		Notes:         c.Notes,
	})
	if err != nil {
		if isNotFound(err) {
			return customer.Customer{}, customer.ErrNotFound
		}
		return customer.Customer{}, mapCustomerErr(err)
	}
	return toDomainCustomer(m), nil
}

func (r *customerRepo) UpdateStatus(ctx context.Context, tenantID, id int64, status customer.Status) error {
	if err := r.q.UpdateCustomerStatus(ctx, sqlc.UpdateCustomerStatusParams{
		ID: id, TenantID: tenantID, Status: sqlc.CustomerStatus(status),
	}); err != nil {
		return fmt.Errorf("update customer status: %w", err)
	}
	return nil
}

func (r *customerRepo) SoftDelete(ctx context.Context, tenantID, id int64) error {
	if err := r.q.SoftDeleteCustomer(ctx, sqlc.SoftDeleteCustomerParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("soft delete customer: %w", err)
	}
	return nil
}

func (r *customerRepo) SetActiveUntil(ctx context.Context, tenantID, id int64, until time.Time, status customer.Status) error {
	if err := r.q.SetCustomerActiveUntil(ctx, sqlc.SetCustomerActiveUntilParams{
		ID:          id,
		TenantID:    tenantID,
		ActiveUntil: pgTimestamptzPtr(&until),
		Status:      sqlc.CustomerStatus(status),
	}); err != nil {
		return fmt.Errorf("set active until: %w", err)
	}
	return nil
}

func (r *customerRepo) ListExpiredActive(ctx context.Context, limit int32) ([]customer.Expired, error) {
	rows, err := r.q.ListExpiredActiveCustomers(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired active customers: %w", err)
	}
	out := make([]customer.Expired, len(rows))
	for i, m := range rows {
		out[i] = customer.Expired{
			ID:            m.ID,
			TenantID:      m.TenantID,
			PppoeUsername: textVal(m.PppoeUsername),
			PlanID:        int8Ptr(m.PlanID),
		}
	}
	return out, nil
}

// mapCustomerErr translates unique-violation errors based on the constraint.
func mapCustomerErr(err error) error {
	if isUniqueViolation(err) {
		// Two unique indexes exist (customer_no and pppoe_username); the
		// constraint name in the message distinguishes them.
		if strings.Contains(err.Error(), "pppoe") {
			return customer.ErrUsernameTaken
		}
		return customer.ErrNoTaken
	}
	return fmt.Errorf("customer write: %w", err)
}
