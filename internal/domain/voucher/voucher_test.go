package voucher

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode("WIFI", 6)
	require.NoError(t, err)
	assert.Len(t, code, 10)
	assert.True(t, strings.HasPrefix(code, "WIFI"))
	for _, c := range code[4:] {
		assert.Contains(t, codeAlphabet, string(c), "code must use the safe alphabet")
	}
	assert.NotContains(t, code[4:], "0")
	assert.NotContains(t, code[4:], "O")
}

func TestGenerateCode_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := GenerateCode("", 8)
		require.NoError(t, err)
		assert.False(t, seen[c], "collision at %d", i)
		seen[c] = true
	}
}

func TestValidateQty(t *testing.T) {
	assert.NoError(t, ValidateQty(1))
	assert.NoError(t, ValidateQty(5000))
	assert.ErrorIs(t, ValidateQty(0), ErrInvalidQty)
	assert.ErrorIs(t, ValidateQty(5001), ErrInvalidQty)
}
