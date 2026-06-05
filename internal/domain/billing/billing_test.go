package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestProrate(t *testing.T) {
	start := date(2026, time.June, 1)
	end := date(2026, time.July, 1) // 30-day cycle

	cases := []struct {
		name  string
		price int64
		from  time.Time
		want  int64
	}{
		{"full cycle from start", 300000, start, 300000},
		{"before start is full", 300000, date(2026, time.May, 20), 300000},
		{"on end is zero", 300000, end, 0},
		{"after end is zero", 300000, date(2026, time.July, 5), 0},
		{"half cycle", 300000, date(2026, time.June, 16), 150000}, // 15/30 days remain
		{"ten days remain", 300000, date(2026, time.June, 21), 100000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Prorate(tc.price, start, end, tc.from))
		})
	}
}

func TestComputeTotals(t *testing.T) {
	// 100000 subtotal, 10% tax, no discount.
	got := ComputeTotals(100000, 0, 1000)
	assert.Equal(t, Totals{Subtotal: 100000, Discount: 0, Tax: 10000, Total: 110000}, got)

	// Discount applied before tax: (100000-20000)*11% = 8800.
	got = ComputeTotals(100000, 20000, 1100)
	assert.Equal(t, int64(80000+8800), got.Total)
	assert.Equal(t, int64(8800), got.Tax)

	// Over-discount clamps taxable to zero.
	got = ComputeTotals(50000, 60000, 1000)
	assert.Equal(t, int64(0), got.Total)
	assert.Equal(t, int64(0), got.Tax)
}

func TestCanTransition(t *testing.T) {
	assert.True(t, CanTransition(StatusUnpaid, StatusPaid))
	assert.True(t, CanTransition(StatusUnpaid, StatusOverdue))
	assert.True(t, CanTransition(StatusOverdue, StatusPaid))
	assert.True(t, CanTransition(StatusDraft, StatusUnpaid))

	assert.False(t, CanTransition(StatusPaid, StatusUnpaid), "paid is terminal")
	assert.False(t, CanTransition(StatusVoid, StatusPaid), "void is terminal")
	assert.False(t, CanTransition(StatusPaid, StatusVoid))
}
