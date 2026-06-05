package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMikrotikRate(t *testing.T) {
	cases := []struct {
		name string
		bp   BandwidthProfile
		want string
	}{
		{"explicit override wins", BandwidthProfile{MikrotikRateString: "x", RateLimitRx: "10M", RateLimitTx: "2M"}, "x"},
		{"empty when no rates", BandwidthProfile{}, ""},
		{"rate only", BandwidthProfile{RateLimitRx: "10M", RateLimitTx: "2M"}, "10M/2M"},
		{"with burst", BandwidthProfile{
			RateLimitRx: "10M", RateLimitTx: "2M", BurstRx: "12M", BurstTx: "4M",
		}, "10M/2M 12M/4M"},
		{"full chain with priority", BandwidthProfile{
			RateLimitRx: "10M", RateLimitTx: "2M", BurstRx: "12M", BurstTx: "4M",
			BurstThresholdRx: "8M", BurstThresholdTx: "1M", BurstTime: "16", Priority: 8,
		}, "10M/2M 12M/4M 8M/1M 16/16 8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.bp.MikrotikRate())
		})
	}
}

func TestPlan_GroupNameAndTax(t *testing.T) {
	p := Plan{ID: 42, PriceIDR: 50000, TaxBps: 1000} // 10%
	assert.Equal(t, "plan_42", p.GroupName())
	assert.Equal(t, int64(5000), p.TaxIDR())
	assert.Equal(t, int64(55000), p.TotalIDR())
}

func TestPlan_Validate(t *testing.T) {
	valid := Plan{Name: "P", ServiceType: ServicePPPoE, BillingCycle: CycleMonthly, ActiveDays: 30}
	assert.NoError(t, valid.Validate())

	bad := valid
	bad.PriceIDR = -1
	assert.ErrorIs(t, bad.Validate(), ErrInvalidPrice)

	bad = valid
	bad.ActiveDays = 0
	assert.ErrorIs(t, bad.Validate(), ErrInvalidPlan)
}
