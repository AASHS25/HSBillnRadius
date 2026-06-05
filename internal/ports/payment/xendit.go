package payment

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
)

// Xendit implements Gateway for Xendit invoices.
type Xendit struct{ http *http.Client }

// NewXendit builds a Xendit gateway.
func NewXendit(httpClient *http.Client) *Xendit { return &Xendit{http: httpClient} }

// Provider returns the provider id.
func (x *Xendit) Provider() Provider { return ProviderXendit }

type xenditNotification struct {
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
	PaidAmount int64  `json:"paid_amount"`
	Amount     int64  `json:"amount"`
}

// ExtractOrderID reads external_id without verifying.
func (x *Xendit) ExtractOrderID(raw []byte) (string, error) {
	var n xenditNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return "", fmt.Errorf("xendit parse: %w", err)
	}
	if n.ExternalID == "" {
		return "", fmt.Errorf("xendit: missing external_id")
	}
	return n.ExternalID, nil
}

// VerifyCallback validates the x-callback-token header and maps the status.
func (x *Xendit) VerifyCallback(_ context.Context, raw []byte, headers http.Header, config map[string]string) (CallbackResult, error) {
	var n xenditNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return CallbackResult{}, fmt.Errorf("xendit parse: %w", err)
	}

	token := headers.Get("X-Callback-Token")
	verified := subtle.ConstantTimeCompare([]byte(token), []byte(config["callback_token"])) == 1

	amount := n.PaidAmount
	if amount == 0 {
		amount = n.Amount
	}
	return CallbackResult{
		OrderID:   n.ExternalID,
		Status:    x.mapStatus(n.Status),
		AmountIDR: amount,
		Verified:  verified,
		Raw:       raw,
	}, nil
}

func (x *Xendit) mapStatus(status string) billing.PaymentStatus {
	switch strings.ToUpper(status) {
	case "PAID", "SETTLED":
		return billing.PaySettled
	case "PENDING":
		return billing.PayPending
	case "EXPIRED":
		return billing.PayExpired
	default:
		return billing.PayFailed
	}
}

// CreateCharge creates a Xendit invoice and returns its URL.
func (x *Xendit) CreateCharge(ctx context.Context, req ChargeRequest) (ChargeResponse, error) {
	base := req.Config["base_url"]
	if base == "" {
		base = "https://api.xendit.co"
	}
	endpoint := strings.TrimRight(base, "/") + "/v2/invoices"

	payload := map[string]any{
		"external_id": req.OrderID,
		"amount":      req.AmountIDR,
		"payer_email": req.CustomerEmail,
		"description": "Invoice " + req.OrderID,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return ChargeResponse{}, fmt.Errorf("xendit build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(req.Config["secret_key"], "")

	resp, err := x.http.Do(httpReq)
	if err != nil {
		return ChargeResponse{}, fmt.Errorf("xendit create charge: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := readLimited(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChargeResponse{}, fmt.Errorf("xendit http %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		ID         string `json:"id"`
		InvoiceURL string `json:"invoice_url"`
	}
	_ = json.Unmarshal(raw, &out)
	return ChargeResponse{Ref: req.OrderID, RedirectURL: out.InvoiceURL, Status: billing.PayPending, Raw: raw}, nil
}

// readLimited reads up to 64 KiB of a response body.
func readLimited(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, 1<<16))
}
