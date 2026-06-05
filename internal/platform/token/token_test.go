package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueAndParseAccess(t *testing.T) {
	m := NewManager("test-secret-of-sufficient-length-xx", "billing-radius", 15*time.Minute)

	tok, exp, err := m.IssueAccess(AccessInput{
		TenantID:    7,
		UserID:      42,
		RoleID:      3,
		Permissions: []string{"user.read", "user.create"},
	})
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), exp, 5*time.Second)

	claims, err := m.ParseAccess(tok)
	require.NoError(t, err)
	assert.Equal(t, int64(7), claims.TenantID)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, int64(3), claims.RoleID)
	assert.Equal(t, "42", claims.Subject)
	assert.ElementsMatch(t, []string{"user.read", "user.create"}, claims.Permissions)
}

func TestParseAccess_RejectsTamperedToken(t *testing.T) {
	m := NewManager("test-secret-of-sufficient-length-xx", "billing-radius", time.Minute)
	tok, _, err := m.IssueAccess(AccessInput{UserID: 1})
	require.NoError(t, err)

	_, err = m.ParseAccess(tok + "tampered")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestParseAccess_RejectsWrongSecret(t *testing.T) {
	m1 := NewManager("secret-one-of-sufficient-length-aaa", "billing-radius", time.Minute)
	m2 := NewManager("secret-two-of-sufficient-length-bbb", "billing-radius", time.Minute)

	tok, _, err := m1.IssueAccess(AccessInput{UserID: 1})
	require.NoError(t, err)

	_, err = m2.ParseAccess(tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestParseAccess_RejectsExpired(t *testing.T) {
	m := NewManager("test-secret-of-sufficient-length-xx", "billing-radius", -time.Minute)
	tok, _, err := m.IssueAccess(AccessInput{UserID: 1})
	require.NoError(t, err)

	_, err = m.ParseAccess(tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestGenerateRefresh_UniqueAndHashable(t *testing.T) {
	a, ah, err := GenerateRefresh()
	require.NoError(t, err)
	b, bh, err := GenerateRefresh()
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
	assert.NotEqual(t, ah, bh)
	assert.Equal(t, ah, HashRefresh(a), "hash must be deterministic")
	assert.NotEqual(t, a, ah, "stored hash must differ from plaintext")
}
