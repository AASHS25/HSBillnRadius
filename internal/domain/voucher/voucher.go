// Package voucher holds the voucher/batch entities and secure code generation.
// A voucher is a hotspot credential: its code is the RADIUS username/password.
package voucher

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Status is a voucher's lifecycle state.
type Status string

const (
	StatusUnused   Status = "unused"
	StatusUsed     Status = "used"
	StatusExpired  Status = "expired"
	StatusDisabled Status = "disabled"
)

// Domain errors.
var (
	ErrNotFound      = errors.New("voucher not found")
	ErrBatchNotFound = errors.New("voucher batch not found")
	ErrInvalidQty    = errors.New("voucher quantity must be between 1 and 5000")
)

// maxBatch caps a single generation request.
const maxBatch = 5000

// Batch is a generated set of vouchers sharing a plan and price.
type Batch struct {
	ID         int64
	TenantID   int64
	PlanID     int64
	Prefix     string
	Qty        int32
	PriceIDR   int64
	TemplateID *int64
	CreatedBy  *int64
	CreatedAt  time.Time
}

// Voucher is a single hotspot credential.
type Voucher struct {
	ID        int64
	TenantID  int64
	BatchID   int64
	Code      string
	Username  string
	Password  string
	Status    Status
	UsedAt    *time.Time
	SoldAt    *time.Time
	CreatedAt time.Time
}

// ValidateQty checks a batch size is within bounds.
func ValidateQty(qty int32) error {
	if qty < 1 || qty > maxBatch {
		return ErrInvalidQty
	}
	return nil
}

// codeAlphabet excludes ambiguous characters (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// GenerateCode returns prefix followed by length random alphabet characters.
func GenerateCode(prefix string, length int) (string, error) {
	b := make([]byte, length)
	maxIdx := big.NewInt(int64(len(codeAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", fmt.Errorf("generate voucher code: %w", err)
		}
		b[i] = codeAlphabet[n.Int64()]
	}
	return prefix + string(b), nil
}
