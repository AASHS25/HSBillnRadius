package payment

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
)

// Midtrans implements Gateway for Midtrans (Snap).
type Midtrans struct{ http *http.Client }

// NewMidtrans builds a Midtrans gateway.
func NewMidtrans(httpClient *http.Client) *Midtrans { return &Midtrans{http: httpClient} }

// Provider returns the provider id.
func (m *Midtrans) Provider() Provider { return ProviderMidtrans }

type midtransNotification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}

// ExtractOrderID reads order_id without verifying.
func (m *Midtrans) ExtractOrderID(raw []byte) (string, error) {
	var n midtransNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return "", fmt.Errorf("midtrans parse: %w", err)
	}
	if n.OrderID == "" {
		return "", fmt.Errorf("midtrans: missing order_id")
	}
	return n.OrderID, nil
}

// VerifyCallback validates the SHA-512 signature and maps the status.
// signature = sha512(order_id + status_code + gross_amount + server_key).
func (m *Midtrans) VerifyCallback(_ context.Context, raw []byte, _ http.Header, config map[string]string) (CallbackResult, error) {
	var n midtransNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return CallbackResult{}, fmt.Errorf("midtrans parse: %w", err)
	}

	mac := sha512.Sum512([]byte(n.OrderID + n.StatusCode + n.GrossAmount + config["server_key"]))
	expected := hex.EncodeToString(mac[:])
	verified := subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(n.SignatureKey))) == 1

	return CallbackResult{
		OrderID:   n.OrderID,
		Status:    m.mapStatus(n),
		AmountIDR: parseGrossAmount(n.GrossAmount),
		Verified:  verified,
		Raw:       raw,
	}, nil
}

func (m *Midtrans) mapStatus(n midtransNotification) billing.PaymentStatus {
	switch n.TransactionStatus {
	case "capture":
		if n.FraudStatus == "challenge" {
			return billing.PayPending
		}
		return billing.PaySettled
	case "settlement":
		return billing.PaySettled
	case "pending":
		return billing.PayPending
	case "deny", "cancel", "failure":
		return billing.PayFailed
	case "expire":
		return billing.PayExpired
	default:
		return billing.PayPending
	}
}

// CreateCharge creates a Snap transaction and returns its redirect URL.
func (m *Midtrans) CreateCharge(ctx context.Context, req ChargeRequest) (ChargeResponse, error) {
	base := req.Config["base_url"]
	if base == "" {
		base = "https://app.sandbox.midtrans.com"
		if req.IsProduction {
			base = "https://app.midtrans.com"
		}
	}
	endpoint := strings.TrimRight(base, "/") + "/snap/v1/transactions"

	payload := map[string]any{
		"transaction_details": map[string]any{"order_id": req.OrderID, "gross_amount": req.AmountIDR},
		"customer_details": map[string]any{
			"first_name": req.CustomerName, "email": req.CustomerEmail, "phone": req.CustomerPhone,
		},
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return ChargeResponse{}, fmt.Errorf("midtrans build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(req.Config["server_key"], "")

	resp, err := m.http.Do(httpReq)
	if err != nil {
		return ChargeResponse{}, fmt.Errorf("midtrans create charge: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirect_url"`
	}
	raw, _ := readLimited(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChargeResponse{}, fmt.Errorf("midtrans http %d: %s", resp.StatusCode, string(raw))
	}
	_ = json.Unmarshal(raw, &out)
	return ChargeResponse{Ref: req.OrderID, RedirectURL: out.RedirectURL, Status: billing.PayPending, Raw: raw}, nil
}

// parseGrossAmount converts "166500.00" to int64 rupiah.
func parseGrossAmount(s string) int64 {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
