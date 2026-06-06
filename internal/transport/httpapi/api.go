// Package httpapi is the HTTP delivery layer: it decodes/validates requests,
// invokes the service layer and maps results and domain errors to HTTP. It
// holds no business logic.
package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/aashs25/hsbillnradius/internal/platform/ratelimit"
	"github.com/aashs25/hsbillnradius/internal/platform/token"
	"github.com/aashs25/hsbillnradius/internal/service/acssvc"

	"github.com/aashs25/hsbillnradius/internal/service/authsvc"
	"github.com/aashs25/hsbillnradius/internal/service/billingsvc"
	"github.com/aashs25/hsbillnradius/internal/service/customersvc"
	"github.com/aashs25/hsbillnradius/internal/service/nassvc"
	"github.com/aashs25/hsbillnradius/internal/service/notifysvc"
	"github.com/aashs25/hsbillnradius/internal/service/paymentsvc"
	"github.com/aashs25/hsbillnradius/internal/service/plansvc"
	"github.com/aashs25/hsbillnradius/internal/service/resellersvc"
	"github.com/aashs25/hsbillnradius/internal/service/ticketsvc"
	"github.com/aashs25/hsbillnradius/internal/service/vouchersvc"
)

// API wires the service layer into HTTP handlers.
type API struct {
	auth      *authsvc.Service
	plans     *plansvc.Service
	customers *customersvc.Service
	billing   *billingsvc.Service
	notify    *notifysvc.Service
	payments  *paymentsvc.Service
	vouchers  *vouchersvc.Service
	tickets   *ticketsvc.Service
	acs       *acssvc.Service
	reseller  *resellersvc.Service
	nas       *nassvc.Service
	limiter   ratelimit.Limiter
	tokens    *token.Manager
	valid     *validator.Validate
	log       *slog.Logger
}

// New builds the API delivery layer.
func New(auth *authsvc.Service, plans *plansvc.Service, customers *customersvc.Service, billing *billingsvc.Service, notify *notifysvc.Service, payments *paymentsvc.Service, vouchers *vouchersvc.Service, tickets *ticketsvc.Service, acs *acssvc.Service, reseller *resellersvc.Service, nas *nassvc.Service, limiter ratelimit.Limiter, tokens *token.Manager, log *slog.Logger) *API {
	return &API{
		auth:      auth,
		plans:     plans,
		customers: customers,
		billing:   billing,
		notify:    notify,
		payments:  payments,
		vouchers:  vouchers,
		tickets:   tickets,
		acs:       acs,
		reseller:  reseller,
		nas:       nas,
		limiter:   limiter,
		tokens:    tokens,
		valid:     validator.New(validator.WithRequiredStructEnabled()),
		log:       log,
	}
}

// Mount registers all API routes under /api/v1 on the given router.
func (a *API) Mount(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			// Throttle credential endpoints per client IP (anti-brute-force).
			r.With(RateLimit(a.limiter, 5, time.Minute, a.log)).Post("/register", a.handleRegister)
			r.With(RateLimit(a.limiter, 10, time.Minute, a.log)).Post("/login", a.handleLogin)
			r.Post("/refresh", a.handleRefresh)
			r.Post("/logout", a.handleLogout)
		})

		// Public payment webhooks (verified by provider signature, not JWT).
		r.Post("/webhooks/payment/{provider}", a.handlePaymentWebhook)

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(Authenticator(a.tokens, a.log))
			r.Get("/me", a.handleMe)
			r.With(RequirePermission("user.read", a.log)).Get("/users", a.handleListUsers)
			r.With(RequirePermission("tenant.read", a.log)).Get("/audit-logs", a.handleListAuditLogs)

			r.Route("/plans", func(r chi.Router) {
				r.Use(RequirePermission("plan.manage", a.log))
				r.Post("/", a.handleCreatePlan)
				r.Get("/", a.handleListPlans)
				r.Get("/{id}", a.handleGetPlan)
				r.Put("/{id}", a.handleUpdatePlan)
				r.Delete("/{id}", a.handleDeletePlan)
			})

			r.Route("/customers", func(r chi.Router) {
				r.With(RequirePermission("customer.create", a.log)).Post("/", a.handleCreateCustomer)
				r.With(RequirePermission("customer.read", a.log)).Get("/", a.handleListCustomers)
				r.With(RequirePermission("customer.read", a.log)).Get("/{id}", a.handleGetCustomer)
				r.With(RequirePermission("customer.update", a.log)).Put("/{id}", a.handleUpdateCustomer)
				r.With(RequirePermission("customer.delete", a.log)).Delete("/{id}", a.handleDeleteCustomer)
				r.With(RequirePermission("customer.update", a.log)).Post("/{id}/portal-password", a.handleSetPortalPassword)
			})

			r.Route("/invoices", func(r chi.Router) {
				r.With(RequirePermission("invoice.manage", a.log)).Post("/generate", a.handleGenerateInvoice)
				r.With(RequirePermission("invoice.read", a.log)).Get("/", a.handleListInvoices)
				r.With(RequirePermission("invoice.read", a.log)).Get("/{id}", a.handleGetInvoice)
				r.With(RequirePermission("payment.manage", a.log)).Post("/{id}/pay", a.handlePayInvoice)
				r.With(RequirePermission("invoice.manage", a.log)).Post("/{id}/void", a.handleVoidInvoice)
			})
			r.With(RequirePermission("invoice.read", a.log)).Get("/reports/summary", a.handleReportSummary)

			r.Route("/notifications", func(r chi.Router) {
				r.Use(RequirePermission("tenant.update", a.log))
				r.Post("/gateways", a.handleConfigureGateway)
				r.Post("/templates", a.handleUpsertTemplate)
				r.Post("/send", a.handleSendNotification)
			})

			r.Route("/payments", func(r chi.Router) {
				r.With(RequirePermission("tenant.update", a.log)).Post("/gateways", a.handleConfigurePaymentGateway)
				r.With(RequirePermission("payment.manage", a.log)).Post("/charge", a.handleCreateCharge)
			})

			r.Route("/vouchers", func(r chi.Router) {
				r.Use(RequirePermission("plan.manage", a.log))
				r.Post("/batches", a.handleGenerateVouchers)
				r.Get("/batches", a.handleListVoucherBatches)
				r.Get("/batches/{id}/vouchers", a.handleListBatchVouchers)
			})

			r.Route("/tickets", func(r chi.Router) {
				r.Use(RequirePermission("customer.read", a.log))
				r.Post("/", a.handleCreateTicket)
				r.Get("/", a.handleListTickets)
				r.Get("/{id}", a.handleGetTicket)
				r.Post("/{id}/transition", a.handleTransitionTicket)
			})
			r.With(RequirePermission("customer.read", a.log)).Get("/maps/customers.geojson", a.handleMapsGeoJSON)

			r.Route("/nas", func(r chi.Router) {
				r.Use(RequirePermission("tenant.update", a.log))
				r.Post("/", a.handleCreateNas)
				r.Get("/", a.handleListNas)
			})

			r.Route("/reseller", func(r chi.Router) {
				r.Use(RequirePermission("payment.manage", a.log))
				r.Post("/deposits", a.handleTopup)
				r.Get("/deposits", a.handleListDeposits)
				r.Get("/commissions", a.handleListCommissions)
			})

			r.Route("/acs/devices", func(r chi.Router) {
				r.Use(RequirePermission("customer.update", a.log))
				r.Get("/", a.handleListACSDevices)
				r.Post("/", a.handleRegisterACSDevice)
				r.Post("/{deviceID}/wifi", a.handleSetACSWiFi)
				r.Post("/{deviceID}/reboot", a.handleRebootACSDevice)
			})
		})

		// Client-area (customer portal): separate customer authentication.
		r.With(RateLimit(a.limiter, 10, time.Minute, a.log)).Post("/portal/login", a.handlePortalLogin)
		r.Group(func(r chi.Router) {
			r.Use(Authenticator(a.tokens, a.log))
			r.Use(RequireCustomer(a.log))
			r.Get("/portal/me", a.handlePortalMe)
			r.Get("/portal/invoices", a.handlePortalInvoices)
			r.Get("/portal/tickets", a.handlePortalTickets)
			r.Post("/portal/tickets", a.handlePortalCreateTicket)
		})
	})
}

// parseIDParam reads a positive int64 path parameter.
func parseIDParam(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errBadRequest("invalid " + name)
	}
	return id, nil
}

// pagination parses limit/offset query params with sane bounds.
func pagination(r *http.Request) (limit, offset int32) {
	const defaultLimit, maxLimit = 50, 200
	limit, offset = defaultLimit, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		if v > maxLimit {
			v = maxLimit
		}
		limit = int32(v)
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = int32(v)
	}
	return limit, offset
}

// clientIP extracts a best-effort client IP from a trusted forwarding header,
// falling back to the connection's remote address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
