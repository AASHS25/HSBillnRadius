package tenant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSlug(t *testing.T) {
	valid := []string{"acme", "acme-isp", "isp123", "a1b2c3"}
	for _, s := range valid {
		assert.NoError(t, ValidateSlug(s), "expected %q valid", s)
	}
	invalid := []string{"", "ab", "A1", "-acme", "acme-", "has space", "uppercaseX"}
	for _, s := range invalid {
		assert.ErrorIs(t, ValidateSlug(s), ErrInvalidSlug, "expected %q invalid", s)
	}
}

func TestStatus_Valid(t *testing.T) {
	assert.True(t, StatusActive.Valid())
	assert.True(t, StatusTrial.Valid())
	assert.False(t, Status("bogus").Valid())
}
