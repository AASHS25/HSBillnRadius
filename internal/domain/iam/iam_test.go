package iam

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeEmail(t *testing.T) {
	assert.Equal(t, "user@example.com", NormalizeEmail("  USER@Example.COM "))
}

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.co", "user.name@sub.example.com"}
	for _, e := range valid {
		assert.NoError(t, ValidateEmail(e), "expected %q valid", e)
	}
	invalid := []string{"", "@b.com", "a@", "a@b", "no-at.com", "a b@c.com"}
	for _, e := range invalid {
		assert.ErrorIs(t, ValidateEmail(e), ErrInvalidEmail, "expected %q invalid", e)
	}
}

func TestValidatePassword(t *testing.T) {
	assert.ErrorIs(t, ValidatePassword("short"), ErrWeakPassword)
	assert.NoError(t, ValidatePassword("longenough"))
}

func TestRefreshToken_Active(t *testing.T) {
	now := time.Now()
	revoked := now.Add(-time.Minute)

	assert.True(t, RefreshToken{ExpiresAt: now.Add(time.Hour)}.Active(now))
	assert.False(t, RefreshToken{ExpiresAt: now.Add(-time.Hour)}.Active(now), "expired")
	assert.False(t, RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}.Active(now), "revoked")
}
