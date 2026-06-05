package payment_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/ports/payment"
)

// midtransSig computes the documented SHA-512 notification signature.
func midtransSig(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}

func midtransBody(orderID, statusCode, grossAmount, txStatus, serverKey string) []byte {
	return []byte(fmt.Sprintf(
		`{"order_id":%q,"status_code":%q,"gross_amount":%q,"transaction_status":%q,"fraud_status":"accept","signature_key":%q}`,
		orderID, statusCode, grossAmount, txStatus, midtransSig(orderID, statusCode, grossAmount, serverKey)))
}

func TestMidtrans_VerifyCallback_Valid(t *testing.T) {
	m := payment.NewMidtrans(nil)
	raw := midtransBody("order-1", "200", "166500.00", "settlement", "SERVER-KEY")

	res, err := m.VerifyCallback(context.Background(), raw, http.Header{}, map[string]string{"server_key": "SERVER-KEY"})
	require.NoError(t, err)
	assert.True(t, res.Verified)
	assert.Equal(t, billing.PaySettled, res.Status)
	assert.Equal(t, int64(166500), res.AmountIDR)
	assert.Equal(t, "order-1", res.OrderID)
}

func TestMidtrans_VerifyCallback_BadSignature(t *testing.T) {
	m := payment.NewMidtrans(nil)
	// Signature computed with a different key than configured.
	raw := midtransBody("order-1", "200", "166500.00", "settlement", "WRONG-KEY")

	res, err := m.VerifyCallback(context.Background(), raw, http.Header{}, map[string]string{"server_key": "SERVER-KEY"})
	require.NoError(t, err)
	assert.False(t, res.Verified, "signature from a different key must not verify")
}

func TestMidtrans_StatusMapping(t *testing.T) {
	m := payment.NewMidtrans(nil)
	cases := map[string]billing.PaymentStatus{
		"settlement": billing.PaySettled,
		"pending":    billing.PayPending,
		"expire":     billing.PayExpired,
		"deny":       billing.PayFailed,
	}
	for tx, want := range cases {
		raw := midtransBody("o", "200", "1000.00", tx, "K")
		res, err := m.VerifyCallback(context.Background(), raw, http.Header{}, map[string]string{"server_key": "K"})
		require.NoError(t, err)
		assert.Equal(t, want, res.Status, "tx=%s", tx)
	}
}
