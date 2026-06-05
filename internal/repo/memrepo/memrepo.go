// Package memrepo is an in-memory implementation of every repository port plus
// TxManager, for unit-testing the service layer without a database. Each port
// is a distinct small type because their method sets collide on names.
package memrepo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	"github.com/aashs25/hsbillnradius/internal/domain/voucher"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Store holds all in-memory state and hands out repositories that share it.
type Store struct {
	mu             sync.Mutex
	tenants        map[int64]tenant.Tenant
	users          map[int64]iam.User
	roles          map[int64]iam.Role
	rolePerms      map[int64][]string
	tokens         map[string]iam.RefreshToken
	audits         []audit.Entry
	plans          map[int64]plan.Plan
	bandwidth      map[int64]plan.BandwidthProfile // keyed by planID
	customers      map[int64]customer.Customer
	radcheck       map[string][]radius.Attr
	radgroup       map[string]radius.UserGroup
	radreply       map[string][]radius.Attr
	nas            map[string]radius.Nas // keyed by IP
	postauth       []radius.PostAuth
	sessions       map[string]radius.AcctEvent // keyed by uniqueid
	invoices       map[int64]billing.Invoice
	items          []billing.InvoiceItem
	payments       map[string]billing.Payment // keyed by idempotency key
	ledger         []billing.LedgerEntry
	notifs         map[int64]notification.Log
	notifStat      map[int64]notification.Status
	dedup          map[string]bool
	gateways       map[int64]notification.Gateway   // by tenant
	templates      map[string]notification.Template // tenant|key|channel
	pgGateways     map[string]billing.GatewayConfig // tenant|provider
	voucherBatches map[int64]voucher.Batch
	vouchers       map[int64]voucher.Voucher
	tickets        map[int64]ticket.Ticket
	ticketEvents   []ticket.Event
	allPerms       []string
	seq            map[string]int64
}

// New returns an empty Store seeded with a default permission catalog.
func New() *Store {
	return &Store{
		tenants:        map[int64]tenant.Tenant{},
		users:          map[int64]iam.User{},
		roles:          map[int64]iam.Role{},
		rolePerms:      map[int64][]string{},
		tokens:         map[string]iam.RefreshToken{},
		plans:          map[int64]plan.Plan{},
		bandwidth:      map[int64]plan.BandwidthProfile{},
		customers:      map[int64]customer.Customer{},
		radcheck:       map[string][]radius.Attr{},
		radgroup:       map[string]radius.UserGroup{},
		radreply:       map[string][]radius.Attr{},
		nas:            map[string]radius.Nas{},
		sessions:       map[string]radius.AcctEvent{},
		invoices:       map[int64]billing.Invoice{},
		payments:       map[string]billing.Payment{},
		notifs:         map[int64]notification.Log{},
		notifStat:      map[int64]notification.Status{},
		dedup:          map[string]bool{},
		gateways:       map[int64]notification.Gateway{},
		templates:      map[string]notification.Template{},
		pgGateways:     map[string]billing.GatewayConfig{},
		voucherBatches: map[int64]voucher.Batch{},
		vouchers:       map[int64]voucher.Voucher{},
		tickets:        map[int64]ticket.Ticket{},
		allPerms: []string{
			"tenant.read", "tenant.update", "user.read", "user.create",
			"role.manage", "plan.manage", "customer.create", "customer.read",
			"customer.update", "customer.delete",
		},
		seq: map[string]int64{},
	}
}

func (s *Store) next(name string) int64 {
	s.seq[name]++
	return s.seq[name]
}

func key(tenantID int64, s string) string { return fmt.Sprintf("%d|%s", tenantID, s) }

// Repositories returns the repository bundle backed by this store.
func (s *Store) Repositories() repo.Repositories {
	return repo.Repositories{
		Tenant:       &tenantRepo{s},
		User:         &userRepo{s},
		Role:         &roleRepo{s},
		Permission:   &permRepo{s},
		RefreshToken: &tokenRepo{s},
		Audit:        &auditRepo{s},
		Plan:         &planRepo{s},
		Bandwidth:    &bandwidthRepo{s},
		Customer:     &customerRepo{s},
		RadiusMap:    &radiusRepo{s},
		RadiusAuth:   &radiusAuthRepo{s},
		Accounting:   &accountingRepo{s},
		Billing:      &billingRepo{s},
		Notification: &notificationRepo{s},
		Voucher:      &voucherRepo{s},
		Ticket:       &ticketRepo{s},
	}
}

// NotifStatus returns the delivery status of a notification (for tests).
func (s *Store) NotifStatus(id int64) notification.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notifStat[id]
}

// SeedNas registers a NAS for auth tests and returns it with an id.
func (s *Store) SeedNas(n radius.Nas) radius.Nas {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.ID = s.next("nas")
	s.nas[n.Name] = n
	return n
}

// PostAuthCount returns how many radpostauth records were written.
func (s *Store) PostAuthCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.postauth)
}

// WithTx runs fn against the same store (no real isolation/rollback).
func (s *Store) WithTx(_ context.Context, fn func(r repo.Repositories) error) error {
	return fn(s.Repositories())
}

var _ repo.TxManager = (*Store)(nil)

// --- test inspection / manipulation helpers ---------------------------------

// SetUserActive flips a user's active flag.
func (s *Store) SetUserActive(id int64, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	u.IsActive = active
	s.users[id] = u
}

// ExpireToken backdates a refresh token's expiry.
func (s *Store) ExpireToken(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.tokens[hash]
	t.ExpiresAt = time.Now().Add(-time.Hour)
	s.tokens[hash] = t
}

// UserGroupName returns the radusergroup membership for a user, if any.
func (s *Store) UserGroupName(tenantID int64, username string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.radgroup[key(tenantID, username)]
	return g.Groupname, ok
}

// RadCheckValue returns a radcheck attribute value for a user.
func (s *Store) RadCheckValue(tenantID int64, username, attr string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.radcheck[key(tenantID, username)] {
		if a.Attribute == attr {
			return a.Value, true
		}
	}
	return "", false
}

// GroupReplyValue returns a radgroupreply attribute value for a group.
func (s *Store) GroupReplyValue(tenantID int64, groupname, attr string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.radreply[key(tenantID, groupname)] {
		if a.Attribute == attr {
			return a.Value, true
		}
	}
	return "", false
}
