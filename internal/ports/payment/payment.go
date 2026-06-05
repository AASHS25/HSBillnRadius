// Package payment defines the payment-gateway port and provider implementations
// (Midtrans, Xendit). The webhook path verifies provider signatures before any
// money moves; CreateCharge initiates a VA/QRIS/e-wallet charge.
package payment

import (
	"context"
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
)

// Provider identifies a payment gateway vendor.
type Provider string

const (
	ProviderMidtrans Provider = "midtrans"
	ProviderXendit   Provider = "xendit"
)

// ChargeRequest initiates a charge.
type ChargeRequest struct {
	OrderID       string
	AmountIDR     int64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Config        map[string]string // server_key/secret/base_url
	IsProduction  bool
}

// ChargeResponse is the result of initiating a charge.
type ChargeResponse struct {
	Ref         string
	RedirectURL string
	Status      billing.PaymentStatus
	Raw         []byte
}

// CallbackResult is the parsed (and verified) outcome of a webhook.
type CallbackResult struct {
	OrderID   string
	Status    billing.PaymentStatus
	AmountIDR int64
	Verified  bool
	Raw       []byte
}

// Gateway is a payment provider adapter.
type Gateway interface {
	Provider() Provider
	// ExtractOrderID reads the order id from an unverified callback body so the
	// handler can locate the local payment (and thus the tenant + config).
	ExtractOrderID(raw []byte) (string, error)
	// VerifyCallback validates the signature against config and returns the
	// parsed result with Verified set accordingly.
	VerifyCallback(ctx context.Context, raw []byte, headers http.Header, config map[string]string) (CallbackResult, error)
	// CreateCharge initiates a charge with the provider.
	CreateCharge(ctx context.Context, req ChargeRequest) (ChargeResponse, error)
}
